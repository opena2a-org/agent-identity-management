"""
Polling monitors: process, network and filesystem.

Ported from ``sdk/typescript/src/arp/monitors/{process,network,filesystem}.ts``.
Each monitor polls on a daemon thread and emits into an :class:`EventEngine`.
The parsing of ``ps``/``lsof``/``ss``/``netstat``/``/proc/net/tcp`` output is
module-level and pure so the behavioral suite can pin it to the TS reference.

Differences from the TS reference, by necessity:

* :class:`FilesystemMonitor` compares directory snapshots on an interval; the
  Python standard library has no recursive ``fs.watch``. It reports a created
  or deleted entry as ``rename`` and a modified file as ``change``, the event
  types ``fs.watch`` reports.
* The monitors' own ``ps``/``lsof`` calls run marked as ARP-internal, so the
  process interceptor does not report them.
"""
from __future__ import annotations

import getpass
import math
import os
import platform
import re
import subprocess
import threading
from typing import Any, Dict, Iterable, List, Optional, Sequence, Set

from ._reentry import arp_internal
from .types import now_ms

#: Binaries commonly used for exfiltration, lateral movement, or exploitation.
SUSPICIOUS_BINARIES = (
    "curl", "wget", "nc", "ncat", "nmap", "ssh", "scp",
    "python", "python3", "perl", "ruby", "base64",
    "socat", "telnet", "ftp", "rsync",
)

#: Known exfiltration/suspicious destinations.
SUSPICIOUS_HOSTS = (
    "webhook.site", "requestbin", "ngrok.io", "pipedream.net",
    "hookbin.com", "burpcollaborator", "interact.sh", "oastify.com",
    "pastebin.com", "transfer.sh",
)

#: Paths an agent should never touch.
SENSITIVE_PATHS = (
    ".ssh", ".aws", ".gnupg", ".kube", ".config/gcloud",
    ".docker/config.json", ".npmrc", ".pypirc",
    ".git-credentials", "wallet.json",
    ".bashrc", ".zshrc", ".bash_profile", ".profile",
    ".gitconfig", ".env", ".netrc", ".pgpass",
)

#: Arguments of the one ``ps`` call that reads the process table.
LISTING_ARGS = ("-ax", "-o", "pid=,ppid=,user=,%cpu=,%mem=,command=")
LISTING_COMMAND = " ".join(("ps",) + LISTING_ARGS)
_LISTING_ROW = re.compile(r"^\s*(\d+)\s+(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.+?)\s*$")


def _js_int(text: str) -> Optional[int]:
    """JavaScript ``parseInt(text)`` (base 10): leading digits, else None (NaN)."""
    match = re.match(r"^\s*([+-]?\d+)", text or "")
    return int(match.group(1)) if match else None


def _js_float(text: str) -> float:
    match = re.match(r"^\s*([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)", text or "")
    return float(match.group(1)) if match else float("nan")


def _basename(command: str) -> str:
    """``path.basename`` (POSIX): last segment, trailing slashes ignored."""
    stripped = command.rstrip("/")
    return stripped.rsplit("/", 1)[-1] if stripped else ""


# --------------------------------------------------------------------------- #
# Process table
# --------------------------------------------------------------------------- #
def parse_ps_listing(
    output: str, listing_pid: Optional[int] = None, agent_pid: Optional[int] = None
) -> List[Dict[str, Any]]:
    """Parse ``ps -ax -o pid=,ppid=,user=,%cpu=,%mem=,command=`` output.

    The listing ``ps`` process's own row is left out: matched by its pid, the
    agent as its parent and its exact command line.
    """
    rows = []
    for line in output.split("\n"):
        match = _LISTING_ROW.match(line)
        if not match:
            continue
        row = {
            "pid": int(match.group(1)),
            "ppid": int(match.group(2)),
            "user": match.group(3),
            "cpu": _js_float(match.group(4)),
            "mem": _js_float(match.group(5)),
            "command": match.group(6),
        }
        if row["pid"] == listing_pid and row["ppid"] == agent_pid and row["command"] == LISTING_COMMAND:
            continue
        rows.append(row)
    return rows


def get_descendants(table: Sequence[Dict[str, Any]], agent_pid: int) -> List[Dict[str, Any]]:
    """Every descendant of ``agent_pid`` in ``table``, breadth first."""
    if not agent_pid:
        return []
    children: Dict[int, List[Dict[str, Any]]] = {}
    for row in table:
        children.setdefault(row["ppid"], []).append(row)
    result: List[Dict[str, Any]] = []
    queue = [agent_pid]
    while queue:
        current = queue.pop(0)
        for child in children.get(current, []):
            result.append(child)
            queue.append(child["pid"])
    return result


class _Poller:
    """A daemon thread calling ``poll()`` every ``interval_ms``."""

    type = ""

    def __init__(self, interval_ms: float):
        self.interval_ms = interval_ms
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None

    def _loop(self) -> None:
        while not self._stop.wait(self.interval_ms / 1000.0):
            try:
                self.poll()
            except Exception:  # noqa: BLE001 - skip this cycle
                pass

    def _start_thread(self) -> None:
        self._stop.clear()
        self._thread = threading.Thread(target=self._loop, name="aim-arp-%s-monitor" % self.type,
                                        daemon=True)
        self._thread.start()

    def poll(self) -> None:  # pragma: no cover - overridden
        raise NotImplementedError

    def stop(self) -> None:
        self._stop.set()
        thread, self._thread = self._thread, None
        if thread is not None and thread is not threading.current_thread():
            thread.join(timeout=max(self.interval_ms / 1000.0, 1.0) + 5.0)

    def is_running(self) -> bool:
        return self._thread is not None


class ProcessMonitor(_Poller):
    """Tracks the agent's child processes with ``ps`` (TS ``ProcessMonitor``)."""

    type = "process"

    def __init__(self, engine: Any, interval_ms: float = 5000):
        super().__init__(interval_ms)
        self._engine = engine
        self._known: Set[int] = set()
        self._agent_pid: Optional[int] = None

    def start(self) -> None:
        if self.is_running():
            return
        self._agent_pid = os.getpid()
        table = self.read_process_table()
        self._known = {r["pid"] for r in get_descendants(table, self._agent_pid)} if table else set()
        self._start_thread()

    def read_process_table(self) -> Optional[List[Dict[str, Any]]]:
        """One ``ps`` call, no shell; None when it fails."""
        try:
            with arp_internal():
                proc = subprocess.Popen(
                    ["ps"] + list(LISTING_ARGS),
                    stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True,
                )
                try:
                    out, _ = proc.communicate(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.communicate()
                    return None
        except OSError:
            return None
        if proc.returncode != 0 or not isinstance(out, str):
            return None
        return parse_ps_listing(out, proc.pid, self._agent_pid or os.getpid())

    def poll(self) -> None:
        table = self.read_process_table()
        if table is None:
            return
        self.process_snapshot(get_descendants(table, self._agent_pid or os.getpid()))

    def process_snapshot(self, current: Sequence[Dict[str, Any]]) -> None:
        """Emit for one poll's view of the agent's descendants."""
        current_set = {info["pid"] for info in current}
        for info in current:
            if info["pid"] not in self._known:
                self._engine.emit({
                    "source": "process", "category": "normal", "severity": "info",
                    "description": "New child process: PID %d — %s" % (info["pid"], info["command"][:100]),
                    "data": {"pid": info["pid"], "command": info["command"], "user": info["user"],
                             "cpu": info["cpu"], "mem": info["mem"]},
                })
        for pid in sorted(self._known):
            if pid not in current_set:
                self._engine.emit({
                    "source": "process", "category": "normal", "severity": "info",
                    "description": "Child process terminated: PID %d" % pid,
                    "data": {"pid": pid, "action": "terminated"},
                })

        try:
            me = getpass.getuser()
        except Exception:  # noqa: BLE001
            me = ""
        for info in current:
            pid = info["pid"]
            binary = _basename(re.split(r"\s+", info["command"])[0])
            if binary in SUSPICIOUS_BINARIES:
                self._engine.emit({
                    "source": "process", "category": "violation", "severity": "high",
                    "description": "Suspicious binary executed: %s (PID %d)" % (binary, pid),
                    "data": {"pid": pid, "binary": binary, "command": info["command"], "user": info["user"]},
                })
            if info["cpu"] > 90:
                self._engine.emit({
                    "source": "process", "category": "anomaly", "severity": "medium",
                    "description": "High CPU usage: PID %d at %s%% — %s"
                                   % (pid, _num(info["cpu"]), info["command"][:60]),
                    "data": {"pid": pid, "cpu": info["cpu"], "command": info["command"]},
                })
            if info["user"] == "root" and me != "root":
                self._engine.emit({
                    "source": "process", "category": "violation", "severity": "high",
                    "description": "Child process running as root: PID %d — %s" % (pid, info["command"][:60]),
                    "data": {"pid": pid, "user": info["user"], "command": info["command"]},
                })
        self._known = current_set


def _num(value: float) -> str:
    return str(int(value)) if float(value).is_integer() else repr(float(value))


# --------------------------------------------------------------------------- #
# Network connections
# --------------------------------------------------------------------------- #
def _without_none(record: Dict[str, Any]) -> Dict[str, Any]:
    """Drop absent values, as ``JSON.stringify`` drops ``undefined``."""
    return {k: v for k, v in record.items() if v is not None}


def _conn(local_addr, local_port, remote_addr, remote_port, protocol="tcp", pid=None):
    return _without_none({
        "protocol": protocol,
        "localAddr": local_addr,
        "localPort": local_port,
        "remoteAddr": remote_addr,
        "remotePort": remote_port,
        "state": "ESTABLISHED",
        "pid": pid,
    })


def parse_lsof(output: str) -> List[Dict[str, Any]]:
    """Parse ``lsof -i -n -P | grep ESTABLISHED`` lines."""
    out = []
    for line in output.strip().split("\n"):
        if not line:
            continue
        parts = re.split(r"\s+", line)
        if len(parts) < 9:
            continue
        match = re.search(r"(\S+):(\d+)->(\S+):(\d+)", parts[8] or "")
        if not match:
            continue
        out.append(_conn(
            match.group(1), int(match.group(2)), match.group(3), int(match.group(4)),
            protocol="tcp" if "TCP" in (parts[7] or "") else "udp",
            pid=_js_int(parts[1]) or None,
        ))
    return out


def parse_ss(output: str) -> List[Dict[str, Any]]:
    """Parse ``ss -tpn state established`` (header line decides the column shape)."""
    lines = output.strip().split("\n")
    local_idx = 3 if lines[0].startswith("State") else 2
    out = []
    for line in lines[1:]:
        if not line:
            continue
        parts = re.split(r"\s+", line)
        if len(parts) < local_idx + 2:
            continue
        local = parts[local_idx].split(":")
        remote = parts[local_idx + 1].split(":")
        out.append(_conn(
            ":".join(local[:-1]), _js_int(local[-1]),
            ":".join(remote[:-1]), _js_int(remote[-1]),
        ))
    return out


def parse_hex_addr(hex_pair: str) -> Optional[Dict[str, Any]]:
    """``0100007F:0050`` (little-endian IPv4) -> ``{"addr": "127.0.0.1", "port": 80}``."""
    pieces = hex_pair.split(":")
    if len(pieces) < 2 or not pieces[0] or not pieces[1]:
        return None
    try:
        port = int(pieces[1], 16)
        num = int(pieces[0], 16)
    except ValueError:
        return None
    num &= 0xFFFFFFFF
    addr = ".".join(str((num >> shift) & 0xFF) for shift in (0, 8, 16, 24))
    return {"addr": addr, "port": port}


def parse_proc_net_tcp(content: str) -> List[Dict[str, Any]]:
    out = []
    for line in content.strip().split("\n")[1:]:
        parts = line.strip().split()
        if len(parts) < 4:
            continue
        try:
            state = int(parts[3], 16)
        except ValueError:
            continue
        if state != 1:  # 01 = ESTABLISHED
            continue
        local = parse_hex_addr(parts[1])
        remote = parse_hex_addr(parts[2])
        if not local or not remote:
            continue
        out.append(_conn(local["addr"], local["port"], remote["addr"], remote["port"]))
    return out


def split_addr_port(value: str) -> Optional[Dict[str, Any]]:
    """``addr:port`` or macOS netstat ``addr.port``."""
    colon = value.rfind(":")
    if colon > 0:
        port = _js_int(value[colon + 1:])
        if port is not None:
            return {"addr": value[:colon], "port": port}
    dot = value.rfind(".")
    if dot > 0:
        port = _js_int(value[dot + 1:])
        if port is not None:
            return {"addr": value[:dot], "port": port}
    return None


def parse_netstat(output: str) -> List[Dict[str, Any]]:
    out = []
    for line in output.strip().split("\n"):
        if not line:
            continue
        parts = line.strip().split()
        if len(parts) < 5 or not parts[0].startswith("tcp"):
            continue
        local = split_addr_port(parts[3])
        remote = split_addr_port(parts[4])
        if not local or not remote:
            continue
        out.append(_conn(local["addr"], local["port"], remote["addr"], remote["port"]))
    return out


def is_suspicious_host(host: str) -> bool:
    return any(h in host for h in SUSPICIOUS_HOSTS)


def is_allowed_host(host: str, allowed_hosts: Iterable[str]) -> bool:
    allowed = set(allowed_hosts)
    if not allowed:
        return True
    return host in allowed or any(host == h or host.endswith("." + h) for h in allowed)


def _run_shell(command: str) -> str:
    with arp_internal():
        try:
            result = subprocess.run(
                command, shell=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                text=True, timeout=5,
            )
        except (OSError, subprocess.SubprocessError):
            return ""
    return result.stdout or ""


class NetworkMonitor(_Poller):
    """Tracks outbound connections with lsof/ss/netstat (TS ``NetworkMonitor``)."""

    type = "network"
    BURST_WINDOW_MS = 60000

    def __init__(
        self,
        engine: Any,
        interval_ms: float = 10000,
        allowed_hosts: Optional[Iterable[str]] = None,
        burst_threshold: float = 3,
        clock=None,
    ):
        super().__init__(interval_ms)
        self._engine = engine
        self._allowed = set(allowed_hosts or [])
        self._known: Set[str] = set()
        self._timestamps: List[float] = []
        self._rolling_avg = 0.0
        self._window_count = 0
        self._burst_threshold = burst_threshold
        self._clock = clock or now_ms

    def start(self) -> None:
        if self.is_running():
            return
        for conn in self.get_connections():
            self._known.add("%s:%s" % (conn["remoteAddr"], conn["remotePort"]))
        self._start_thread()

    def poll(self) -> None:
        self.process_connections(self.get_connections())

    def process_connections(self, connections: Sequence[Dict[str, Any]]) -> None:
        """Emit for one poll's connections, then run burst detection."""
        for conn in connections:
            remote = conn["remoteAddr"]
            dest = "%s:%s" % (remote, conn["remotePort"])
            if is_suspicious_host(remote):
                self._engine.emit({
                    "source": "network", "category": "threat", "severity": "critical",
                    "description": "Connection to suspicious host: %s" % dest,
                    "data": _without_none({"remoteAddr": remote, "remotePort": conn["remotePort"],
                                           "protocol": conn["protocol"], "pid": conn.get("pid")}),
                })
                continue
            if dest not in self._known:
                allowed = is_allowed_host(remote, self._allowed)
                self._engine.emit({
                    "source": "network",
                    "category": "normal" if allowed else "anomaly",
                    "severity": "info" if allowed else "medium",
                    "description": "New outbound connection: %s" % dest,
                    "data": _without_none({"remoteAddr": remote, "remotePort": conn["remotePort"],
                                           "protocol": conn["protocol"], "pid": conn.get("pid"),
                                           "allowed": allowed}),
                })
                self._known.add(dest)
        self.check_connection_burst(len(connections))

    def check_connection_burst(self, connection_count: int) -> None:
        now = self._clock()
        self._timestamps.extend([now] * connection_count)
        cutoff = now - self.BURST_WINDOW_MS
        self._timestamps = [t for t in self._timestamps if t >= cutoff]
        current = len(self._timestamps)

        if self._window_count == 0:
            self._rolling_avg = current
            self._window_count = 1
            return

        previous = self._rolling_avg
        self._rolling_avg = 0.2 * current + 0.8 * previous
        self._window_count += 1
        if self._window_count < 3:
            return

        threshold = previous * self._burst_threshold
        if threshold > 0 and current > threshold:
            self._engine.emit({
                "source": "network", "category": "violation", "severity": "high",
                "description": "Connection burst detected: %d connections in last 60s "
                               "(threshold: %d, avg: %d)"
                               % (current, _js_round(threshold), _js_round(previous)),
                "data": {
                    "currentCount": current,
                    "rollingAverage": _js_round(previous),
                    "threshold": _js_round(threshold),
                    "burstMultiplier": self._burst_threshold,
                },
            })

    def get_connections(self) -> List[Dict[str, Any]]:
        if platform.system() == "Darwin":
            found = parse_lsof(_run_shell("lsof -i -n -P 2>/dev/null | grep ESTABLISHED"))
            return found or parse_netstat(_run_shell("netstat -an 2>/dev/null | grep ESTABLISHED"))
        found = []
        ss_out = _run_shell("ss -tpn state established 2>/dev/null")
        if ss_out.strip():
            found = parse_ss(ss_out)
        if found:
            return found
        try:
            with arp_internal():
                with open("/proc/net/tcp", "r", encoding="utf-8") as fh:
                    found = parse_proc_net_tcp(fh.read())
        except OSError:
            found = []
        return found or parse_netstat(_run_shell("netstat -an 2>/dev/null | grep ESTABLISHED"))


def _js_round(value: float) -> int:
    """``Math.round``: halves round up."""
    return int(math.floor(value + 0.5))


# --------------------------------------------------------------------------- #
# Filesystem
# --------------------------------------------------------------------------- #
class FilesystemMonitor(_Poller):
    """Watches paths for sensitive or out-of-bounds file activity (TS ``FilesystemMonitor``)."""

    type = "filesystem"
    DEBOUNCE_MS = 1000
    MAX_ENTRIES = 20000

    def __init__(
        self,
        engine: Any,
        watch_paths: Optional[Sequence[str]] = None,
        allowed_paths: Optional[Iterable[str]] = None,
        interval_ms: float = 2000,
        clock=None,
    ):
        super().__init__(interval_ms)
        self._engine = engine
        self._watch_paths = list(watch_paths) if watch_paths is not None else [os.getcwd()]
        self._allowed = list(allowed_paths or [])
        self._clock = clock or now_ms
        self._debounce: Dict[str, float] = {}
        self._snapshots: Dict[str, Dict[str, Any]] = {}

    def start(self) -> None:
        if self.is_running():
            return
        self._snapshots = {}
        for watch_path in self._watch_paths:
            try:
                resolved = os.path.realpath(watch_path)
                if os.path.isdir(resolved):
                    self._snapshots[resolved] = self._snapshot(resolved)
            except OSError:
                continue
        if self._snapshots:
            self._start_thread()

    def stop(self) -> None:
        super().stop()
        self._debounce.clear()

    def _snapshot(self, base: str) -> Dict[str, Any]:
        entries: Dict[str, Any] = {}
        with arp_internal():
            for root, dirs, files in os.walk(base):
                for name in dirs + files:
                    full = os.path.join(root, name)
                    try:
                        st = os.lstat(full)
                    except OSError:
                        continue
                    entries[os.path.relpath(full, base)] = (st.st_mtime_ns, st.st_size)
                    if len(entries) >= self.MAX_ENTRIES:
                        return entries
        return entries

    def poll(self) -> None:
        for base, before in list(self._snapshots.items()):
            after = self._snapshot(base)
            for name in sorted(set(before) | set(after)):
                if name not in before or name not in after:
                    self.handle_event("rename", name, base)
                elif before[name] != after[name]:
                    self.handle_event("change", name, base)
            self._snapshots[base] = after

    def handle_event(self, event_type: str, filename: str, base_path: str) -> None:
        now = self._clock()
        key = "%s:%s" % (event_type, filename)
        until = self._debounce.get(key)
        if until is not None and now < until:
            return
        self._debounce[key] = now + self.DEBOUNCE_MS

        full = os.path.join(base_path, filename)
        if any(sp in filename or sp in full for sp in SENSITIVE_PATHS):
            self._engine.emit({
                "source": "filesystem", "category": "violation", "severity": "high",
                "description": "Access to sensitive path: %s (%s)" % (filename, event_type),
                "data": {"path": filename, "eventType": event_type, "sensitive": True},
            })
            return
        if self._allowed and not any(full.startswith(ap) for ap in self._allowed):
            self._engine.emit({
                "source": "filesystem", "category": "anomaly", "severity": "medium",
                "description": "File access outside allowed paths: %s" % filename,
                "data": {"path": filename, "eventType": event_type, "allowed": False},
            })
            return
        if event_type == "rename":
            self._engine.emit({
                "source": "filesystem", "category": "normal", "severity": "info",
                "description": "File created/renamed: %s" % filename,
                "data": {"path": filename, "eventType": event_type},
            })


__all__ = [
    "SUSPICIOUS_BINARIES", "SUSPICIOUS_HOSTS", "SENSITIVE_PATHS",
    "LISTING_ARGS", "LISTING_COMMAND",
    "parse_ps_listing", "get_descendants",
    "parse_lsof", "parse_ss", "parse_hex_addr", "parse_proc_net_tcp",
    "split_addr_port", "parse_netstat", "is_suspicious_host", "is_allowed_host",
    "ProcessMonitor", "NetworkMonitor", "FilesystemMonitor",
]

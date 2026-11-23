"""
Interceptors: application-level hooks on Python's own primitives.

Ported from ``sdk/typescript/src/arp/interceptors/{filesystem,process,network}.ts``,
where they hook ``fs``, ``child_process`` and ``net.Socket``. Here:

* :class:`FilesystemInterceptor` patches ``builtins.open`` (and ``io.open``,
  the same function, which ``pathlib`` calls), ``os.remove``/``os.unlink`` and
  ``os.mkdir``;
* :class:`ProcessInterceptor` patches ``subprocess.Popen.__init__`` (every
  ``subprocess.run``/``call``/``check_output`` goes through it) and
  ``os.system``;
* :class:`NetworkInterceptor` patches ``socket.socket.connect`` and
  ``connect_ex`` (every ``http.client``/``urllib``/``requests`` connection goes
  through them).

Each hook emits its event BEFORE the call proceeds and then makes the original
call unchanged: interceptors observe, they never block or alter the agent's
I/O, and an error inside ARP never reaches the caller. They are a telemetry
producer only. ``stop()`` puts the original functions back.

Nothing ARP does itself is intercepted: the engine marks its handlers as
ARP-internal (see ``_reentry``), and paths passed as ``exclude_paths`` (ARP's
data directory) are never reported.
"""
from __future__ import annotations

import builtins
import inspect
import io
import os
import re
import socket
import subprocess
import threading
from typing import Any, Callable, Dict, Iterable, List, Optional

from ._reentry import inside_arp
from .monitors import SENSITIVE_PATHS, SUSPICIOUS_BINARIES, SUSPICIOUS_HOSTS, is_allowed_host

#: More subcommands than this in one shell command is an adversarial pipeline.
MAX_SUBCOMMANDS = 10
_SHELL_OPERATORS = re.compile(r"[|;&]|\$\(|`")

_patch_lock = threading.Lock()


def analyze_command_complexity(command: str) -> Dict[str, Any]:
    """Count shell operators: ``{"subcommandCount": n, "isAdversarial": n > 10}``."""
    count = len(_SHELL_OPERATORS.findall(command)) + 1
    return {"subcommandCount": count, "isAdversarial": count > MAX_SUBCOMMANDS}


def _basename(command: str) -> str:
    stripped = command.rstrip("/")
    return stripped.rsplit("/", 1)[-1] if stripped else ""


def _path_text(value: Any) -> Optional[str]:
    """A filesystem path argument as text; None for file descriptors and the like."""
    if isinstance(value, (int, bool)) or value is None:
        return None
    try:
        raw = os.fspath(value)
    except TypeError:
        return None
    return os.fsdecode(raw) if isinstance(raw, bytes) else raw


class _Patch:
    """One patched attribute; the wrapper passes straight through once inactive."""

    def __init__(self, owner: Any, name: str, original: Any, had_own: bool):
        self.owner = owner
        self.name = name
        self.original = original
        self.had_own = had_own
        self.wrapper: Any = None
        self.active = True

    def live_original(self):
        """``(original, had_own)``, skipping wrappers of interceptors stopped since."""
        value, had_own = self.original, self.had_own
        while True:
            inner = getattr(value, "_arp_patch", None)
            if not isinstance(inner, _Patch) or inner.active:
                return value, had_own
            value, had_own = inner.original, inner.had_own


class _Interceptor:
    type = ""

    def __init__(self, engine: Any):
        self._engine = engine
        self._patches: List[_Patch] = []
        self._active = False

    def is_running(self) -> bool:
        return self._active

    def _patch(self, owner: Any, name: str, make_wrapper: Callable[[Any], Any]) -> None:
        had_own = name in getattr(owner, "__dict__", {})
        original = getattr(owner, name)
        patch = _Patch(owner, name, original, had_own)
        patch.wrapper = make_wrapper(original)
        if getattr(patch.wrapper, "_arp_patch", None) is None:
            try:
                patch.wrapper._arp_patch = patch
            except AttributeError:  # pragma: no cover - not a plain function
                pass
        setattr(owner, name, patch.wrapper)
        self._patches.append(patch)

    def _observe(self, fn: Callable[..., None], *args: Any) -> None:
        """Run a handler unless ARP itself is on this thread; never raise."""
        if not self._active or inside_arp():
            return
        try:
            fn(*args)
        except Exception:  # noqa: BLE001 - interception never breaks the caller
            pass

    def start(self) -> None:
        with _patch_lock:
            if self._active:
                return
            self._install()
            self._active = True

    def stop(self) -> None:
        with _patch_lock:
            if not self._active:
                return
            self._active = False
            for patch in self._patches:
                patch.active = False
            for patch in reversed(self._patches):
                # Only unwind what is still ours; a hook installed on top of this
                # one stays, and this wrapper passes through until that hook is
                # stopped too, which then unwinds past it.
                if getattr(patch.owner, patch.name, None) is not patch.wrapper:
                    continue
                original, had_own = patch.live_original()
                if had_own:
                    setattr(patch.owner, patch.name, original)
                else:
                    delattr(patch.owner, patch.name)
            self._patches = []

    def _install(self) -> None:  # pragma: no cover - overridden
        raise NotImplementedError

    def _emit(self, event: Dict[str, Any]) -> None:
        self._engine.emit(event)


# --------------------------------------------------------------------------- #
class FilesystemInterceptor(_Interceptor):
    """Reports file reads, writes, mkdirs and deletes (TS ``FilesystemInterceptor``)."""

    type = "filesystem"

    def __init__(
        self,
        engine: Any,
        allowed_paths: Optional[Iterable[str]] = None,
        exclude_paths: Optional[Iterable[str]] = None,
    ):
        super().__init__(engine)
        self._allowed = list(dict.fromkeys(allowed_paths or []))
        self._exclude = list(dict.fromkeys(exclude_paths or []))

    def _install(self) -> None:
        self_ = self

        def wrap_open(original):
            def open(file, *args, **kwargs):  # noqa: A001 - mirrors the builtin
                path = _path_text(file)
                if path is not None and self_._active and not inside_arp():
                    mode = kwargs.get("mode", args[0] if args else "r")
                    mode = mode if isinstance(mode, str) else "r"
                    if not self_.is_excluded(path):
                        if any(c in mode for c in "wax+"):
                            self_._observe(self_.handle_write, path, "open:%s" % mode)
                        else:
                            self_._observe(self_.handle_read, path)
                return original(file, *args, **kwargs)
            open.__wrapped__ = original
            return open

        def wrap_delete(original):
            def remove(path, *args, **kwargs):
                text = _path_text(path)
                if text is not None and self_._active and not inside_arp() and not self_.is_excluded(text):
                    self_._observe(self_.handle_delete, text)
                return original(path, *args, **kwargs)
            remove.__wrapped__ = original
            return remove

        def wrap_mkdir(original):
            def mkdir(path, *args, **kwargs):
                text = _path_text(path)
                if text is not None and self_._active and not inside_arp() and not self_.is_excluded(text):
                    self_._observe(self_.handle_write, text, "mkdir")
                return original(path, *args, **kwargs)
            mkdir.__wrapped__ = original
            return mkdir

        shared_io_open = io.open is builtins.open
        self._patch(builtins, "open", wrap_open)
        if shared_io_open:
            # io.open is the same function; pathlib calls it.
            self._patch(io, "open", lambda _original: builtins.open)
        else:  # pragma: no cover - io.open already wrapped by someone else
            self._patch(io, "open", wrap_open)
        self._patch(os, "remove", wrap_delete)
        self._patch(os, "unlink", wrap_delete)
        self._patch(os, "mkdir", wrap_mkdir)

    # --- classification (pinned to the TS reference by the behavioral suite) ---
    def is_sensitive_path(self, file_path: str) -> bool:
        normalized = os.path.abspath(file_path)
        base = _basename(file_path)
        return any(sp in normalized for sp in SENSITIVE_PATHS) or base.startswith(".env")

    def is_outside_allowed(self, file_path: str) -> bool:
        if not self._allowed:
            return False
        normalized = os.path.abspath(file_path)
        return not any(normalized.startswith(os.path.abspath(ap)) for ap in self._allowed)

    def is_excluded(self, file_path: str) -> bool:
        if not self._exclude:
            return False
        normalized = os.path.abspath(file_path)
        return any(normalized.startswith(os.path.abspath(ep)) for ep in self._exclude)

    def handle_write(self, file_path: str, operation: str) -> None:
        if self.is_sensitive_path(file_path):
            self._emit({
                "source": "filesystem", "category": "violation", "severity": "high",
                "description": "Intercepted write to sensitive path: %s (%s)" % (file_path, operation),
                "data": {"path": file_path, "operation": operation, "sensitive": True, "intercepted": True},
            })
            return
        if self.is_outside_allowed(file_path):
            self._emit({
                "source": "filesystem", "category": "anomaly", "severity": "medium",
                "description": "Intercepted write outside allowed paths: %s" % file_path,
                "data": {"path": file_path, "operation": operation, "allowed": False, "intercepted": True},
            })
            return
        self._emit({
            "source": "filesystem", "category": "normal", "severity": "info",
            "description": "Intercepted file operation: %s %s" % (operation, file_path),
            "data": {"path": file_path, "operation": operation, "intercepted": True},
        })

    def handle_read(self, file_path: str) -> None:
        if self.is_sensitive_path(file_path):
            self._emit({
                "source": "filesystem", "category": "violation", "severity": "high",
                "description": "Intercepted read of sensitive path: %s" % file_path,
                "data": {"path": file_path, "operation": "read", "sensitive": True, "intercepted": True},
            })

    def handle_delete(self, file_path: str) -> None:
        if self.is_sensitive_path(file_path):
            self._emit({
                "source": "filesystem", "category": "violation", "severity": "critical",
                "description": "Intercepted deletion of sensitive path: %s" % file_path,
                "data": {"path": file_path, "operation": "delete", "sensitive": True, "intercepted": True},
            })


# --------------------------------------------------------------------------- #
class ProcessInterceptor(_Interceptor):
    """Reports every process the agent starts (TS ``ProcessInterceptor``)."""

    type = "process"

    def _install(self) -> None:
        self_ = self

        def wrap_init(original):
            try:
                signature = inspect.signature(original)
            except (TypeError, ValueError):  # pragma: no cover
                signature = None

            def __init__(popen_self, *args, **kwargs):
                if self_._active and not inside_arp() and signature is not None:
                    try:
                        bound = signature.bind(popen_self, *args, **kwargs)
                    except TypeError:
                        bound = None
                    if bound is not None:
                        self_._observe(
                            self_._handle_popen,
                            bound.arguments.get("args"),
                            bool(bound.arguments.get("shell", False)),
                        )
                return original(popen_self, *args, **kwargs)
            __init__.__wrapped__ = original
            return __init__

        def wrap_system(original):
            def system(command):
                text = _path_text(command)
                if text is not None:
                    self_._observe(self_.handle_exec, text)
                return original(command)
            system.__wrapped__ = original
            return system

        self._patch(subprocess.Popen, "__init__", wrap_init)
        self._patch(os, "system", wrap_system)

    def _handle_popen(self, args: Any, shell: bool) -> None:
        if args is None:
            return
        single = _path_text(args)
        if single is not None:
            if shell:
                self.handle_exec(single)
            else:
                self.handle_spawn(single, [])
            return
        argv = [_path_text(a) or str(a) for a in args]
        if not argv:
            return
        if shell:
            self.handle_exec(" ".join(argv))
        else:
            self.handle_spawn(argv[0], argv[1:])

    def handle_spawn(self, command: str, args: List[str]) -> None:
        binary = _basename(command)
        full = " ".join([command] + list(args))
        suspicious = binary in SUSPICIOUS_BINARIES
        self._emit({
            "source": "process",
            "category": "violation" if suspicious else "normal",
            "severity": "high" if suspicious else "info",
            "description": (
                "Intercepted suspicious binary: %s — %s" % (binary, full[:100]) if suspicious
                else "Intercepted process spawn: %s" % full[:100]
            ),
            "data": {"binary": binary, "command": full, "args": list(args),
                     "intercepted": True, "suspicious": suspicious},
        })

    def handle_exec(self, command: str) -> None:
        stripped = command.strip()
        first = re.split(r"\s+", stripped)[0] if stripped else ""
        binary = _basename(first)
        suspicious = binary in SUSPICIOUS_BINARIES
        complexity = analyze_command_complexity(command)
        adversarial = complexity["isAdversarial"]

        if adversarial:
            category, severity = "threat", "critical"
            description = "Adversarial command pipeline detected: %d subcommands (max %d)" % (
                complexity["subcommandCount"], MAX_SUBCOMMANDS)
        elif suspicious:
            category, severity = "violation", "high"
            description = "Intercepted suspicious exec: %s -- %s" % (binary, command[:100])
        else:
            category, severity = "normal", "info"
            description = "Intercepted exec: %s" % command[:100]

        self._emit({
            "source": "process", "category": category, "severity": severity,
            "description": description,
            "data": {"binary": binary, "command": command, "intercepted": True,
                     "suspicious": suspicious, "adversarialPipeline": adversarial,
                     "subcommandCount": complexity["subcommandCount"]},
        })


# --------------------------------------------------------------------------- #
class NetworkInterceptor(_Interceptor):
    """Reports every outbound TCP/UDP connect (TS ``NetworkInterceptor``)."""

    type = "network"

    def __init__(self, engine: Any, allowed_hosts: Optional[Iterable[str]] = None):
        super().__init__(engine)
        self._allowed = set(allowed_hosts or [])

    def _install(self) -> None:
        self_ = self

        def wrap(original):
            def connect(sock, address, *args, **kwargs):
                if (
                    self_._active
                    and not inside_arp()
                    and isinstance(address, tuple)
                    and len(address) >= 2
                    and isinstance(address[1], int)
                ):
                    self_._observe(self_.handle_connection, str(address[0]), address[1])
                return original(sock, address, *args, **kwargs)
            connect.__wrapped__ = original
            return connect

        self._patch(socket.socket, "connect", wrap)
        self._patch(socket.socket, "connect_ex", wrap)

    def handle_connection(self, host: str, port: int) -> None:
        dest = "%s:%s" % (host, port)
        if any(h in host for h in SUSPICIOUS_HOSTS):
            self._emit({
                "source": "network", "category": "threat", "severity": "critical",
                "description": "Intercepted connection to suspicious host: %s" % dest,
                "data": {"remoteAddr": host, "remotePort": port, "intercepted": True},
            })
            return
        allowed = is_allowed_host(host, self._allowed)
        self._emit({
            "source": "network",
            "category": "normal" if allowed else "anomaly",
            "severity": "info" if allowed else "medium",
            "description": "Intercepted outbound connection: %s" % dest,
            "data": {"remoteAddr": host, "remotePort": port, "allowed": allowed, "intercepted": True},
        })


__all__ = [
    "MAX_SUBCOMMANDS",
    "analyze_command_complexity",
    "FilesystemInterceptor",
    "ProcessInterceptor",
    "NetworkInterceptor",
]

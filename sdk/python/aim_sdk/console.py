"""
AIM SDK Console Output - Beautiful terminal formatting inspired by Stripe CLI

This module provides clean, professional console output for the AIM SDK.
Uses Rich library when available, falls back to simple formatting otherwise.
"""

import sys
from typing import Optional, Dict, Any, List

# Try to import Rich for beautiful output
try:
    from rich.console import Console
    from rich.panel import Panel
    from rich.table import Table
    from rich.text import Text
    from rich.box import ROUNDED, SIMPLE, MINIMAL
    from rich import box
    RICH_AVAILABLE = True
except ImportError:
    RICH_AVAILABLE = False

# Global console instance
_console = Console() if RICH_AVAILABLE else None

# AIM Brand Colors
BRAND_BLUE = "#3B82F6"
BRAND_TEAL = "#14B8A6"
BRAND_GREEN = "#22C55E"
BRAND_YELLOW = "#EAB308"
BRAND_RED = "#EF4444"
BRAND_GRAY = "#6B7280"


class AIMConsole:
    """Beautiful console output for AIM SDK operations."""

    def __init__(self, quiet: bool = False):
        self.quiet = quiet
        self.console = Console() if RICH_AVAILABLE else None

    def _simple_print(self, message: str):
        """Fallback print when Rich is not available."""
        if not self.quiet:
            print(message)

    @staticmethod
    def _normalize_trust_score(score: float) -> float:
        """Normalize trust score to a 0-1 float.

        The server may return trust scores as either a 0-1 float (e.g. 0.55)
        or a 0-100 integer (e.g. 55). Python's :.0% format expects 0-1, so
        values > 1 are divided by 100 to normalize.
        """
        if score is None:
            return 0.0
        if score > 1:
            return score / 100.0
        return float(score)

    # ═══════════════════════════════════════════════════════════════════════════
    # AGENT REGISTRATION OUTPUT
    # ═══════════════════════════════════════════════════════════════════════════

    def agent_registered(
        self,
        name: str,
        agent_id: str,
        agent_type: str,
        version: str,
        trust_score: float,
        status: str,
        capabilities: Optional[List[str]] = None,
        mcp_servers: Optional[List[str]] = None
    ):
        """Display beautiful agent registration success message."""
        if self.quiet:
            return

        trust_score = self._normalize_trust_score(trust_score)

        if RICH_AVAILABLE:
            self._agent_registered_rich(
                name, agent_id, agent_type, version,
                trust_score, status, capabilities, mcp_servers
            )
        else:
            self._agent_registered_simple(
                name, agent_id, agent_type, version,
                trust_score, status, capabilities, mcp_servers
            )

    def _agent_registered_rich(
        self,
        name: str,
        agent_id: str,
        agent_type: str,
        version: str,
        trust_score: float,
        status: str,
        capabilities: Optional[List[str]] = None,
        mcp_servers: Optional[List[str]] = None
    ):
        """Rich formatted agent registration output - compact professional style."""
        # Status indicator
        status_color = "green" if status in ["active", "verified"] else "yellow"
        status_icon = "●" if status in ["active", "verified"] else "○"

        # Build compact output lines
        self.console.print()
        self.console.print(f"[bold green]✓[/] [bold]Agent registered:[/] [cyan]{name}[/]")

        # Details in a compact grid format
        id_short = f"{agent_id[:8]}...{agent_id[-4:]}"
        trust_str = self._format_trust_score_inline(trust_score)

        self.console.print(f"  [dim]ID:[/] {id_short}  [dim]Type:[/] [magenta]{agent_type}[/]  [dim]Version:[/] {version}")
        self.console.print(f"  [dim]Status:[/] [{status_color}]{status_icon} {status}[/]  [dim]Trust:[/] {trust_str}")

        if capabilities:
            cap_str = ", ".join(capabilities)
            self.console.print(f"  [dim]Capabilities:[/] {cap_str}")

        self.console.print()

    def _agent_registered_simple(
        self,
        name: str,
        agent_id: str,
        agent_type: str,
        version: str,
        trust_score: float,
        status: str,
        capabilities: Optional[List[str]] = None,
        mcp_servers: Optional[List[str]] = None
    ):
        """Simple formatted agent registration output."""
        print()
        print("╭─────────────────────────────────────────────────╮")
        print("│  ✓ Agent Registered                             │")
        print("├─────────────────────────────────────────────────┤")
        print(f"│  Agent:       {name:<32} │")
        print(f"│  ID:          {agent_id[:8]}...{agent_id[-4:]:<20} │")
        print(f"│  Type:        {agent_type:<32} │")
        print(f"│  Version:     {version:<32} │")
        print(f"│  Status:      {status:<32} │")
        print(f"│  Trust Score: {trust_score:.0%}{'':>28} │")
        if capabilities:
            cap_str = ", ".join(capabilities[:3])
            if len(capabilities) > 3:
                cap_str += f" (+{len(capabilities) - 3})"
            print(f"│  Capabilities: {cap_str:<30} │")
        if mcp_servers:
            mcp_str = ", ".join(mcp_servers[:3])
            if len(mcp_servers) > 3:
                mcp_str += f" (+{len(mcp_servers) - 3})"
            print(f"│  MCP Servers: {mcp_str:<31} │")
        print("╰─────────────────────────────────────────────────╯")
        print()

    def _format_trust_score(self, score: float) -> str:
        """Format trust score with color based on value."""
        if score is None:
            return "[dim]N/A[/]"
        score = self._normalize_trust_score(score)
        if score >= 0.8:
            return f"[bold green]{score:.0%}[/] [dim]Excellent[/]"
        elif score >= 0.6:
            return f"[bold yellow]{score:.0%}[/] [dim]Good[/]"
        elif score >= 0.4:
            return f"[bold orange1]{score:.0%}[/] [dim]Fair[/]"
        else:
            return f"[bold red]{score:.0%}[/] [dim]Low[/]"

    def _format_trust_score_inline(self, score: float) -> str:
        """Format trust score inline (no label, just colored value)."""
        if score is None:
            return "[dim]N/A[/]"
        score = self._normalize_trust_score(score)
        if score >= 0.8:
            return f"[green]{score:.0%}[/]"
        elif score >= 0.6:
            return f"[yellow]{score:.0%}[/]"
        elif score >= 0.4:
            return f"[orange1]{score:.0%}[/]"
        else:
            return f"[red]{score:.0%}[/]"

    # ═══════════════════════════════════════════════════════════════════════════
    # AGENT FOUND (EXISTING CREDENTIALS)
    # ═══════════════════════════════════════════════════════════════════════════

    def agent_found(
        self,
        name: str,
        agent_id: str,
    ):
        """Display message when existing agent credentials are found.

        Status and trust score are deliberately NOT shown here — the cached
        credentials file on disk only captures the snapshot at registration
        time and never refreshes, so anything printed would lie (#175: a
        promoted agent at status=verified, trust=0.92 server-side would
        still print pending / 0% from cache). The dashboard is the
        authoritative source for live status; the CLI banner sticks to
        identity (name + ID) which is stable.
        """
        if self.quiet:
            return

        id_short = f"{agent_id[:8]}...{agent_id[-4:]}"

        if RICH_AVAILABLE:
            self.console.print()
            self.console.print(f"[bold blue]↻[/] [bold]Using existing credentials:[/] [cyan]{name}[/]")
            self.console.print(f"  [dim]ID:[/] {id_short}")
            self.console.print(f"  [dim]Use force_new=True to register a new agent[/]")
            self.console.print()
        else:
            print()
            print("╭─────────────────────────────────────────────────╮")
            print("│  ↻ Using Existing Credentials                   │")
            print("├─────────────────────────────────────────────────┤")
            print(f"│  Agent: {name:<40} │")
            print(f"│  ID:    {id_short:<40} │")
            print("╰─────────────────────────────────────────────────╯")
            print("  Use force_new=True to register a new agent")
            print()

    # ═══════════════════════════════════════════════════════════════════════════
    # CAPABILITY OPERATIONS
    # ═══════════════════════════════════════════════════════════════════════════

    def capability_registered(self, capability: str, risk_level: str):
        """Display capability registration."""
        if self.quiet:
            return

        risk_colors = {
            "low": "green",
            "medium": "yellow",
            "high": "orange1",
            "critical": "red"
        }
        color = risk_colors.get(risk_level, "white")

        if RICH_AVAILABLE:
            self.console.print(f"  [green]✓[/] Registered [cyan]{capability}[/] [{color}]({risk_level})[/]")
        else:
            print(f"  ✓ Registered {capability} ({risk_level})")

    def capability_verified(self, capability: str, status: str, duration_ms: Optional[float] = None):
        """Display capability verification result."""
        if self.quiet:
            return

        duration_str = f" [{duration_ms:.0f}ms]" if duration_ms else ""

        if RICH_AVAILABLE:
            if status in ("approved", "auto-approved"):
                self.console.print(f"  [green]✓[/] [cyan]{capability}[/] verified{duration_str}")
            elif status == "denied":
                self.console.print(f"  [red]✗[/] [cyan]{capability}[/] denied{duration_str}")
            else:
                self.console.print(f"  [yellow]○[/] [cyan]{capability}[/] {status}{duration_str}")
        else:
            icon = "✓" if status in ("approved", "auto-approved") else "✗" if status == "denied" else "○"
            print(f"  {icon} {capability} {status}{duration_str}")

    # ═══════════════════════════════════════════════════════════════════════════
    # JIT ACCESS
    # ═══════════════════════════════════════════════════════════════════════════

    def jit_waiting(self, capability: str, risk_level: str, timeout: int,
                     dashboard_url: Optional[str] = None):
        """Display JIT access waiting message.

        ``dashboard_url`` is the caller's configured ``aim_url`` with the
        verifications page appended. It used to be a hardcoded
        ``http://localhost:3000/...`` regardless of what server the client was
        actually talking to, so a hosted user was told to approve at an address
        that did not exist for them. Falls back to that same string only when
        the caller has none, so offline/unconfigured callers still see a link
        rather than nothing.
        """
        if self.quiet:
            return

        url = dashboard_url or "http://localhost:3000/dashboard/admin/verifications"

        if RICH_AVAILABLE:
            self.console.print()
            panel = Panel(
                f"[bold]Capability:[/] [cyan]{capability}[/]\n"
                f"[bold]Risk Level:[/] [red]{risk_level.upper()}[/]\n"
                f"[bold]Timeout:[/] {timeout}s\n\n"
                f"[dim]Approve in dashboard: [link]{url}[/link][/]",
                title="[bold yellow]⏳ Awaiting Admin Approval[/]",
                title_align="left",
                border_style="yellow",
                padding=(1, 2)
            )
            self.console.print(panel)
        else:
            print()
            print("╭─────────────────────────────────────────────────╮")
            print("│  ⏳ Awaiting Admin Approval                     │")
            print("├─────────────────────────────────────────────────┤")
            print(f"│  Capability:  {capability:<32} │")
            print(f"│  Risk Level:  {risk_level.upper():<32} │")
            print(f"│  Timeout:     {timeout}s{'':>30} │")
            print("╰─────────────────────────────────────────────────╯")
            print("  Approve in dashboard:")
            print(f"  {url}")
            print()

    def jit_approved(self, capability: str):
        """Display JIT access approved message.

        Callers must call this ONLY when AIM returned an explicit ALLOW
        (``decision.outcome is Outcome.ALLOW``). It asserts that a human or
        policy approved the action; printing it for any other reason the
        wrapped function happens to run (monitoring-mode leniency after a
        deny, or the unresolved-mode warning window) is a false statement in
        a security tool's output. See ``jit_unverified`` for those cases.
        """
        if self.quiet:
            return

        if RICH_AVAILABLE:
            self.console.print(f"  [bold green]✓[/] [cyan]{capability}[/] [green]approved[/] - executing...")
        else:
            print(f"  ✓ {capability} approved - executing...")

    def jit_unverified(self, capability: str):
        """Display that the action is proceeding WITHOUT a completed approval.

        Used when the enforcement rule says run=True for a reason other than
        an explicit ALLOW: AIM denied but the organization is in monitoring
        mode, AIM could not be reached and the mode is unresolved, etc. The
        detailed reason is already printed separately via
        ``verdict.warning``/``verdict.pending_change`` before this is called;
        this line exists only so the approval box is not silently followed by
        nothing, which reads as an unexplained pause rather than a completed
        step.
        """
        if self.quiet:
            return

        if RICH_AVAILABLE:
            self.console.print(f"  [dim]○[/] [cyan]{capability}[/] [dim]proceeding without a completed approval[/] - see warning above")
        else:
            print(f"  ○ {capability} proceeding without a completed approval - see warning above")

    def jit_denied(self, capability: str, reason: Optional[str] = None):
        """Display JIT access denied message."""
        if self.quiet:
            return

        reason_str = f" - {reason}" if reason else ""
        if RICH_AVAILABLE:
            self.console.print(f"  [bold red]✗[/] [cyan]{capability}[/] [red]denied[/]{reason_str}")
        else:
            print(f"  ✗ {capability} denied{reason_str}")

    # ═══════════════════════════════════════════════════════════════════════════
    # AUTO-DETECTION
    # ═══════════════════════════════════════════════════════════════════════════

    def detection_result(self, detection_type: str, detected: str, reason: str):
        """Display auto-detection result."""
        if self.quiet:
            return

        if RICH_AVAILABLE:
            self.console.print(f"  [green]✓[/] {detection_type}: [bold]{detected}[/] [dim]({reason})[/]")
        else:
            print(f"  ✓ {detection_type}: {detected} ({reason})")

    def detection_none(self, detection_type: str, fallback: Optional[str] = None):
        """Display when nothing was detected."""
        if self.quiet:
            return

        if fallback:
            if RICH_AVAILABLE:
                self.console.print(f"  [dim]○ {detection_type}: using default '{fallback}'[/]")
            else:
                print(f"  ○ {detection_type}: using default '{fallback}'")
        else:
            if RICH_AVAILABLE:
                self.console.print(f"  [dim]○ {detection_type}: none detected[/]")
            else:
                print(f"  ○ {detection_type}: none detected")

    # ═══════════════════════════════════════════════════════════════════════════
    # ERRORS AND WARNINGS
    # ═══════════════════════════════════════════════════════════════════════════

    def error(self, message: str, details: Optional[str] = None):
        """Display error message."""
        if RICH_AVAILABLE:
            self.console.print(f"[bold red]✗ Error:[/] {message}")
            if details:
                self.console.print(f"  [dim]{details}[/]")
        else:
            print(f"✗ Error: {message}")
            if details:
                print(f"  {details}")

    def warning(self, message: str):
        """Display warning message."""
        if RICH_AVAILABLE:
            self.console.print(f"[yellow]⚠ Warning:[/] {message}")
        else:
            print(f"⚠ Warning: {message}")

    def info(self, message: str):
        """Display info message."""
        if RICH_AVAILABLE:
            self.console.print(f"[blue]ℹ[/] {message}")
        else:
            print(f"ℹ {message}")

    # ═══════════════════════════════════════════════════════════════════════════
    # PROGRESS INDICATORS
    # ═══════════════════════════════════════════════════════════════════════════

    def step(self, number: int, total: int, message: str):
        """Display a step in a process."""
        if self.quiet:
            return

        if RICH_AVAILABLE:
            self.console.print(f"[dim]({number}/{total})[/] {message}")
        else:
            print(f"({number}/{total}) {message}")

    def success(self, message: str):
        """Display success message."""
        if RICH_AVAILABLE:
            self.console.print(f"[bold green]✓[/] {message}")
        else:
            print(f"✓ {message}")

    def debug(self, message: str):
        """Display debug message (only when not in quiet mode and DEBUG env is set)."""
        import os
        if self.quiet or not os.environ.get("AIM_DEBUG"):
            return
        if RICH_AVAILABLE:
            self.console.print(f"[dim]🔍 {message}[/]")
        else:
            print(f"🔍 {message}")


# Global console instance for the SDK
console = AIMConsole()


def set_quiet(quiet: bool = True):
    """Set quiet mode to suppress all output."""
    global console
    console.quiet = quiet

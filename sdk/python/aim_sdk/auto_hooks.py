"""
AIM SDK Auto-Hook Activation

Automatically detects and patches AI frameworks after agent registration.
Activates existing AIM integration handlers without requiring manual imports.
"""

import sys
import time
import threading
from typing import Any, Dict, List, Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from .client import AIMClient


class PolicyCache:
    """
    Local policy cache with TTL for offline enforcement.
    Downloads policies on init, caches in memory with configurable TTL.
    """

    def __init__(self, client: 'AIMClient', ttl_seconds: int = 300):
        self._client = client
        self._ttl = ttl_seconds
        self._cache: Dict[str, Any] = {}
        self._last_refresh: float = 0
        self._lock = threading.Lock()

    def _refresh_if_needed(self) -> None:
        now = time.time()
        if now - self._last_refresh < self._ttl:
            return
        with self._lock:
            # Double-check after acquiring lock
            if time.time() - self._last_refresh < self._ttl:
                return
            try:
                url = f"{self._client.aim_url}/api/v1/agents/{self._client.agent_id}/policies"
                import requests
                headers: Dict[str, str] = {"Content-Type": "application/json"}
                if getattr(self._client, 'api_key', None):
                    headers["X-AIM-API-Key"] = self._client.api_key
                elif getattr(self._client, 'oauth_token_manager', None):
                    token = self._client.oauth_token_manager.get_access_token()
                    if token:
                        headers["Authorization"] = f"Bearer {token}"
                resp = requests.get(url, headers=headers, timeout=10)
                if resp.status_code == 200:
                    policies = resp.json()
                    self._cache = policies if isinstance(policies, dict) else {}
                self._last_refresh = time.time()
            except Exception:
                # On error, keep stale cache rather than failing
                if not self._cache:
                    self._cache = {}
                # Don't update last_refresh so we retry sooner

    def check(self, capability: str) -> bool:
        """Check if a capability is allowed by cached policy. Returns True if allowed."""
        self._refresh_if_needed()
        rules = self._cache.get("rules", [])
        default_action = self._cache.get("defaultAction", "allow")
        for rule in rules:
            if rule.get("capability") == capability:
                return rule.get("action") == "allow"
        return default_action == "allow"

    def invalidate(self) -> None:
        """Force cache refresh on next check."""
        self._last_refresh = 0


def _detect_frameworks() -> Dict[str, bool]:
    """Detect which AI frameworks are currently imported."""
    return {
        "langchain": "langchain" in sys.modules or "langchain_core" in sys.modules,
        "crewai": "crewai" in sys.modules,
        "openai": "openai" in sys.modules,
        "anthropic": "anthropic" in sys.modules,
    }


def _hook_langchain(client: 'AIMClient') -> bool:
    """Inject AIMCallbackHandler into LangChain's default callback manager."""
    try:
        from .integrations.langchain.callback import AIMCallbackHandler
        from langchain_core.callbacks import CallbackManager

        handler = AIMCallbackHandler(agent=client)

        # Patch CallbackManager.__init__ to always include our handler
        _original_init = CallbackManager.__init__

        def _patched_init(self: Any, *args: Any, **kwargs: Any) -> None:
            _original_init(self, *args, **kwargs)
            # Avoid duplicates
            if not any(isinstance(h, AIMCallbackHandler) for h in self.handlers):
                self.add_handler(handler)

        CallbackManager.__init__ = _patched_init  # type: ignore[assignment]
        return True
    except ImportError:
        return False
    except Exception:
        return False


def _hook_crewai(client: 'AIMClient') -> bool:
    """Inject AIMTaskCallback into CrewAI's Crew class."""
    try:
        from .integrations.crewai.callbacks import AIMTaskCallback
        import crewai

        callback = AIMTaskCallback(agent=client)
        _original_crew_init = crewai.Crew.__init__

        def _patched_crew_init(self: Any, *args: Any, **kwargs: Any) -> None:
            _original_crew_init(self, *args, **kwargs)
            # Attach callback to all tasks that don't have one
            if hasattr(self, 'tasks'):
                for task in self.tasks:
                    if not getattr(task, 'callback', None):
                        task.callback = callback.on_task_complete

        crewai.Crew.__init__ = _patched_crew_init  # type: ignore[assignment]
        return True
    except ImportError:
        return False
    except Exception:
        return False


def _hook_openai(client: 'AIMClient') -> bool:
    """Monkey-patch OpenAI client to log LLM calls via security logger."""
    try:
        import openai

        if not hasattr(openai, '_aim_hooked'):
            from .security_logging import security_logger, AgentEventType

            _original_create = openai.resources.chat.completions.Completions.create

            def _patched_create(self: Any, *args: Any, **kwargs: Any) -> Any:
                model = kwargs.get('model', 'unknown')
                try:
                    security_logger.log_agent_event(
                        AgentEventType.AGENT_ACTION,
                        agent_id=client.agent_id,
                        details={
                            "action": "llm_call",
                            "resource": f"openai:{model}",
                            "method": "chat.completions.create",
                        },
                    )
                except Exception:
                    pass  # Never block LLM calls due to logging failure
                return _original_create(self, *args, **kwargs)

            openai.resources.chat.completions.Completions.create = _patched_create  # type: ignore[assignment]
            openai._aim_hooked = True  # type: ignore[attr-defined]
        return True
    except (ImportError, AttributeError):
        return False
    except Exception:
        return False


def _hook_anthropic(client: 'AIMClient') -> bool:
    """Monkey-patch Anthropic client to log LLM calls via security logger."""
    try:
        import anthropic

        if not hasattr(anthropic, '_aim_hooked'):
            from .security_logging import security_logger, AgentEventType

            _original_create = anthropic.resources.messages.Messages.create

            def _patched_create(self: Any, *args: Any, **kwargs: Any) -> Any:
                model = kwargs.get('model', 'unknown')
                try:
                    security_logger.log_agent_event(
                        AgentEventType.AGENT_ACTION,
                        agent_id=client.agent_id,
                        details={
                            "action": "llm_call",
                            "resource": f"anthropic:{model}",
                            "method": "messages.create",
                        },
                    )
                except Exception:
                    pass  # Never block LLM calls due to logging failure
                return _original_create(self, *args, **kwargs)

            anthropic.resources.messages.Messages.create = _patched_create  # type: ignore[assignment]
            anthropic._aim_hooked = True  # type: ignore[attr-defined]
        return True
    except (ImportError, AttributeError):
        return False
    except Exception:
        return False


_HOOK_MAP = {
    "langchain": _hook_langchain,
    "crewai": _hook_crewai,
    "openai": _hook_openai,
    "anthropic": _hook_anthropic,
}


def activate_hooks(client: 'AIMClient', auto_hooks: bool = True) -> List[str]:
    """
    Auto-detect and activate framework hooks.

    Args:
        client: AIMClient instance to use for logging
        auto_hooks: If False, skip hook activation (for manual control)

    Returns:
        List of framework names that were successfully hooked
    """
    if not auto_hooks:
        return []

    detected = _detect_frameworks()
    hooked: List[str] = []

    for framework, is_present in detected.items():
        if is_present:
            hook_fn = _HOOK_MAP.get(framework)
            if hook_fn and hook_fn(client):
                hooked.append(framework)

    return hooked

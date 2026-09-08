"""
AIM SDK Auto-Hook Activation

Automatically detects and patches AI frameworks after agent registration.
Activates existing AIM integration handlers without requiring manual imports.

The hooks installed here are instrumentation. They consult no policy and
never raise: a hook that cannot be installed is logged as a warning and the
framework's calls proceed unchanged.
"""

import logging
import sys
import time
import threading
from typing import Any, Dict, List, Optional, TYPE_CHECKING

from .exceptions import VerificationUnavailableError

if TYPE_CHECKING:
    from .client import AIMClient

logger = logging.getLogger(__name__)

# The route PolicyCache fetches. No released AIM server registers it: the
# backend exposes organization policies under /api/v1/admin/security-policies
# and evaluation under /api/v1/a2a/policies/evaluate, neither per agent. The
# fetch therefore answers 404 against every server released so far.
POLICY_ROUTE = "/api/v1/agents/{agent_id}/policies"


class PolicyCache:
    """
    Local policy cache with TTL.

    Fetches ``GET {aim_url}/api/v1/agents/{agent_id}/policies`` and keeps the
    document for ``ttl_seconds``. No released AIM server registers that route,
    so against every server released so far no document is ever loaded and
    :meth:`check` raises :class:`~aim_sdk.exceptions.VerificationUnavailableError`
    on every call. No SDK code path consults this cache. The enforcement path is
    the decorators, which ask the server on each call and raise
    :class:`~aim_sdk.exceptions.ActionDeniedError` on an explicit denial.

    Once a document is loaded, :meth:`check` fails closed: a rule for the
    capability decides (its ``action`` must equal ``"allow"``), and with no rule
    the document's ``defaultAction`` must be present and equal ``"allow"``.

    An unloaded cache is never an allow. It is not a silent deny either, because
    "AIM was never asked" must stay distinguishable from "AIM said no".
    """

    def __init__(self, client: 'AIMClient', ttl_seconds: int = 300):
        self._client = client
        self._ttl = ttl_seconds
        self._cache: Dict[str, Any] = {}
        self._loaded: bool = False
        self._last_refresh: float = 0
        self._last_error: Optional[str] = None
        self._lock = threading.Lock()

    @property
    def loaded(self) -> bool:
        """True once a policy document (a JSON object) has been fetched."""
        return self._loaded

    def _policy_url(self) -> str:
        return f"{self._client.aim_url}{POLICY_ROUTE.format(agent_id=self._client.agent_id)}"

    def _refresh_if_needed(self) -> None:
        now = time.time()
        if now - self._last_refresh < self._ttl:
            return
        with self._lock:
            # Double-check after acquiring lock
            if time.time() - self._last_refresh < self._ttl:
                return
            url = self._policy_url()
            try:
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
                    if isinstance(policies, dict):
                        self._cache = policies
                        self._loaded = True
                        self._last_error = None
                    else:
                        # A body that is not a JSON object is not a policy
                        # document. Keep whatever was loaded before, if anything.
                        self._last_error = f"HTTP 200 with a non-object body from {url}"
                else:
                    self._last_error = f"HTTP {resp.status_code} from {url}"
                self._last_refresh = time.time()
            except Exception as exc:
                # Keep any previously loaded document rather than failing.
                # last_refresh is not updated, so the next call retries.
                self._last_error = f"{type(exc).__name__} fetching {url}: {exc}"

    def check(self, capability: str) -> bool:
        """
        Return whether ``capability`` is allowed by the loaded policy document.

        Raises :class:`~aim_sdk.exceptions.VerificationUnavailableError` when no
        document has been loaded: the fetch failed, answered non-200, or
        returned a body that is not a JSON object. Never returns ``True`` from
        an unloaded cache.
        """
        self._refresh_if_needed()
        if not self._loaded:
            detail = f" ({self._last_error})" if self._last_error else ""
            raise VerificationUnavailableError(
                "No policy document is loaded, so this capability check cannot be "
                f"answered. Attempted {self._policy_url()}{detail}. No released AIM "
                f"server serves {POLICY_ROUTE}, so PolicyCache cannot enforce; the "
                "decorators, which ask the server on each call, are the enforcement path."
            )
        rules = self._cache.get("rules") or []
        for rule in rules:
            if isinstance(rule, dict) and rule.get("capability") == capability:
                return rule.get("action") == "allow"
        return self._cache.get("defaultAction") == "allow"

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
    """Monkey-patch OpenAI chat completions to record LLM calls via the security logger."""
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
    """Monkey-patch Anthropic messages to record LLM calls via the security logger."""
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
        List of framework names that were successfully hooked. A detected
        framework whose hook could not be installed is logged as a warning and
        left untouched; the hooks never raise.
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
            else:
                logger.warning(
                    "AIM auto-instrumentation: the %s hook could not be installed; "
                    "its calls proceed uninstrumented.",
                    framework,
                )

    return hooked

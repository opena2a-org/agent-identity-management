"""
aim_verify()'s auto-init failure must raise something `except AIMError:` catches.

Found by the aim-sdk 2.0.0 release test. `aim_verify` is one of the SDK's seven
verification entry points, and the SDK's own documentation tells users to write
`except AIMError:` to handle every verification failure in one place
(sdk_python/README.md). Before this fix, the one path where the decorator could
not even START verifying -- no client provided and no AIM_AGENT_NAME to
auto-init one -- raised a bare `builtins.ValueError`, which is not an `AIMError`
and slips straight past that handler.

Reproduced by execution, not by reading the code: with `except AIMError:` wrapped
around the call, the pre-fix ValueError propagates uncaught.
"""

import os

import pytest

from aim_sdk.decorators import aim_verify
from aim_sdk.exceptions import AIMError, ConfigurationError


@pytest.fixture(autouse=True)
def _no_agent_name_env(monkeypatch):
    """The failure this test targets only fires when auto_init has nothing to
    work with. A developer's shell exporting AIM_AGENT_NAME would silently
    change which branch this test exercises."""
    monkeypatch.delenv("AIM_AGENT_NAME", raising=False)


def test_missing_client_and_failed_autoinit_raises_configuration_error():
    @aim_verify(action_type="db:read")
    def read_something():
        return "should not run"

    with pytest.raises(ConfigurationError):
        read_something()


def test_missing_client_and_failed_autoinit_is_caught_by_except_aimerror():
    """The property that actually matters: the SDK's own documented recovery
    pattern must work. `except ValueError:` would also pass this test if the
    fix regressed to a different non-AIMError type, so the assertion is on
    AIMError specifically, not merely "some exception was raised"."""
    @aim_verify(action_type="db:read")
    def read_something():
        return "should not run"

    caught = None
    try:
        read_something()
    except AIMError as e:
        caught = e
    except Exception as e:  # pragma: no cover - failure path, asserted below
        pytest.fail(
            f"except AIMError did not catch it; {type(e).__module__}.{type(e).__name__} "
            f"escaped instead: {e}"
        )

    assert caught is not None, "no exception was raised at all"


def test_message_still_names_both_ways_to_fix_it():
    """Changing the exception TYPE must not silently drop the actionable
    message that was already correct."""
    @aim_verify(action_type="db:read")
    def read_something():
        return "should not run"

    with pytest.raises(ConfigurationError, match="aim_client parameter|AIM_AGENT_NAME"):
        read_something()

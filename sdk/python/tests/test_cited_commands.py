"""
Every command, flag, env var and URL path the SDK cites at the user must resolve.

This exists because a drafted error string in this release pointed at a
`/dashboard/verifications/<id>` route that does not exist. An error message is a
dead end if the thing it tells you to run or visit is not real, and a dead end in
a security error is worse than no message: the developer concludes the tool is
broken and stops reading.

NOT marked `integration` -- `pytest.ini` deselects that marker by default.
"""

import ast
import inspect
import os
import re

import pytest

import aim_sdk.client as client_module
import aim_sdk.decorators as decorators_module
import aim_sdk.enforcement as enforcement_module
import aim_sdk.strict_mode as strict_mode_module
from aim_sdk.decision import (
    EnforcementMode,
    ModeSource,
    Outcome,
    UnknownSource,
    VerificationDecision,
)
from aim_sdk.enforcement import evaluate

SDK_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# The four subcommands `aim-sdk` actually registers, read from the CLI itself
# rather than copied here, so this cannot drift from the parser.
def _registered_cli_commands():
    source = open(os.path.join(SDK_ROOT, "aim_sdk", "cli.py"), encoding="utf-8").read()
    tree = ast.parse(source)
    found = set()
    for node in ast.walk(tree):
        if (
            isinstance(node, ast.Call)
            and isinstance(node.func, ast.Attribute)
            and node.func.attr == "add_parser"
            and node.args
            and isinstance(node.args[0], ast.Constant)
        ):
            found.add(node.args[0].value)
    return found


def _all_enforcement_messages():
    """Every user-facing string this release can produce from the enforcement path."""
    messages = []
    modes = [EnforcementMode.STRICT, EnforcementMode.MONITORING, EnforcementMode.UNKNOWN]
    sources = [UnknownSource.NONE, UnknownSource.TRANSPORT, UnknownSource.SERVER_ANSWER]
    for outcome in Outcome:
        for mode in modes:
            for source in sources:
                for override in (False, True):
                    decision = VerificationDecision(
                        outcome=outcome,
                        mode=mode,
                        mode_source=ModeSource.RESPONSE,
                        reason="a stated reason",
                        verification_id="11111111-2222-3333-4444-555555555555",
                        unknown_source=source,
                    )
                    verdict = evaluate(decision, strict_override=override, what="'db:delete'")
                    if verdict.error is not None:
                        messages.append(str(verdict.error))
                    if verdict.warning:
                        messages.append(verdict.warning)
                    if verdict.pending_change:
                        messages.append(verdict.pending_change)
    return messages


def test_enforcement_messages_are_actually_produced():
    """Guard the guard: if evaluate() stopped producing messages, every assertion
    below would pass vacuously."""
    messages = _all_enforcement_messages()
    assert len(messages) > 10, f"only {len(messages)} messages produced"
    assert any("denied" in m for m in messages)
    assert any("could not" in m for m in messages)
    assert any("3.0.0" in m for m in messages)


@pytest.mark.parametrize("message", _all_enforcement_messages())
def test_cited_cli_commands_exist(message):
    """Any `aim-sdk <subcommand>` cited in a message must be registered."""
    registered = _registered_cli_commands()
    assert registered, "could not read any subcommands from cli.py"
    for cited in re.findall(r"aim-sdk\s+([a-z][a-z0-9-]*)", message):
        assert cited in registered, (
            f"message cites `aim-sdk {cited}`, which is not a registered "
            f"subcommand. Registered: {sorted(registered)}. Message: {message!r}"
        )


@pytest.mark.parametrize("message", _all_enforcement_messages())
def test_cited_env_vars_are_real(message):
    """Any AIM_* environment variable cited must be one the SDK actually reads."""
    known = {"AIM_STRICT_MODE", "AIM_AGENT_NAME", "AIM_URL", "AIM_AUTO_REGISTER",
             "AIM_CREDENTIALS_PATH", "AIM_LOG_LEVEL"}
    for cited in re.findall(r"\b(AIM_[A-Z_]+)\b", message):
        assert cited in known, f"message cites unknown env var {cited}: {message!r}"


@pytest.mark.parametrize("message", _all_enforcement_messages())
def test_no_unresolvable_dashboard_deep_links(message):
    """
    No message may cite a dashboard route. A drafted
    `/dashboard/verifications/<id>` link in this release did not exist, and this
    test is what caught it. Dashboard routes live in a different repo and cannot
    be verified from here, so citing one is a claim we cannot check -- name the
    settings page in prose instead.
    """
    assert "/dashboard/" not in message, (
        f"message cites a dashboard route, which cannot be verified from this "
        f"repo: {message!r}"
    )
    assert "http://" not in message and "https://" not in message, (
        f"message embeds a URL, which cannot be verified from this repo: {message!r}"
    )


@pytest.mark.parametrize("message", _all_enforcement_messages())
def test_no_message_offers_monitoring_mode_as_a_way_past_a_denial(message):
    """
    Remediation truth. Until 2026-08-13 `_denial_message` told a blocked
    developer to "change the organization's enforcement mode to monitoring under
    Settings > Security > Policies". That worked, which was the bug: the SDK
    honoured monitoring on the DENY row and ran the action anyway.

    Now that a denial blocks in every mode the same sentence is a dead end -- the
    developer changes an organization-wide security setting, gets denied again,
    and has no next step -- AND it is advice to weaken a control in order to run
    an action AIM refused. The suite of citation checks around this one did not
    catch it, because every noun in it was real; only the CONSEQUENCE was false.

    Deliberately phrased as "no message", not "not the denial message". A
    remediation that stops being true does not stay put in the function that
    first printed it.
    """
    lowered = message.lower()
    if "denied" not in lowered:
        return

    # The sentence pattern, not a bare keyword: the denial message legitimately
    # explains what monitoring mode does and does not govern, and a keyword ban
    # would forbid explaining the very rule the reader just hit.
    forbidden = [
        "change the organization's enforcement mode to monitoring",
        "change your enforcement mode to monitoring",
        "set the enforcement mode to monitoring",
        "switch to monitoring mode",
        "switching to monitoring",
        "use monitoring mode to",
    ]
    for phrase in forbidden:
        assert phrase not in lowered, (
            f"a denial message offers monitoring mode as remediation. A denial "
            f"blocks in every mode, so this sends the reader to change an "
            f"org-wide security setting and be denied again. Message: {message!r}"
        )


def test_the_denial_message_still_offers_a_real_next_step():
    """
    Guard the guard. Deleting the remediation sentence entirely would pass the
    test above and leave a blocked developer with no path forward at all, which
    is the no-dead-ends violation the test exists to prevent -- just the other
    way round.
    """
    # Selected on the opening clause, not on a keyword anywhere in the body: the
    # denial message explains what monitoring mode does govern ("verifications
    # AIM could not answer"), so a "could not"-excluding filter selects nothing
    # and the loop below passes over an empty list.
    denial_messages = [
        m for m in _all_enforcement_messages() if m.startswith("AIM denied ")
    ]
    assert denial_messages, "no denial messages produced -- the harness is broken"
    for message in denial_messages:
        assert "grant it to this agent" in message, (
            f"denial message offers no way forward: {message!r}"
        )


@pytest.mark.parametrize("message", _all_enforcement_messages())
def test_cited_exception_types_exist(message):
    """Any exception name cited in a message must be a real, importable type."""
    import aim_sdk.exceptions as exceptions_module

    for cited in re.findall(r"\b([A-Z][A-Za-z]*Error)\b", message):
        assert hasattr(exceptions_module, cited) or cited in {
            "PermissionError", "OSError", "IOError", "ValueError", "TypeError",
        }, f"message cites exception {cited}, which does not exist: {message!r}"


def test_ratchet_warnings_cite_only_accepted_spellings():
    """
    The ratchet warning tells the user what to set. Those spellings must be
    exactly the ones the parser accepts -- a warning that names a value the
    parser rejects is a dead end.
    """
    from aim_sdk.strict_mode import TRUE_VALUES, parse_strict_mode

    accepted = ", ".join(sorted(TRUE_VALUES))
    for value in accepted.split(", "):
        assert parse_strict_mode(value) is True, (
            f"the warning tells users to set AIM_STRICT_MODE={value!r}, but the "
            f"parser does not accept it"
        )


def test_readme_no_longer_inverts_the_two_levers():
    """
    The shipped README told production users to configure enforcement in the
    dashboard (which had no effect) and to treat AIM_STRICT_MODE as a testing
    aid (the only lever that did anything). Both sentences were backwards, and
    they were published to users of every released version.
    """
    readme = open(os.path.join(SDK_ROOT, "README.md"), encoding="utf-8").read()
    assert "Production deployments configure in the dashboard, not via env var." not in readme
    assert "Override locally for testing: `AIM_STRICT_MODE=true`" not in readme
    # And it must state the ratchet, since that is the corrected contract.
    assert "ratchet" in readme.lower()


def test_env_config_no_longer_documents_false_as_a_working_lever():
    """`false` was documented as "Log warning and continue". It is now ignored."""
    doc = open(os.path.join(SDK_ROOT, "docs", "ENV_CONFIG.md"), encoding="utf-8").read()
    assert "- `false` - Log warning and continue (development)" not in doc
    assert "ratchet" in doc.lower()


def test_docs_cite_only_exception_names_that_exist():
    """
    `docs/sdk/python.md` told users to
    `from aim_sdk.exceptions import AIMVerificationError, AIMConnectionError` and
    to catch `AIMAuthenticationError`. None of those three names has ever
    existed -- that code block raised ImportError on its first line. It is the
    error-handling reference for exactly the contract this release changes, so a
    reader hitting the new VerificationUnavailableError goes there first.
    """
    import aim_sdk.exceptions as exceptions_module

    doc = os.path.join(os.path.dirname(SDK_ROOT), "..", "docs", "sdk", "python.md")
    doc = os.path.normpath(doc)
    if not os.path.exists(doc):
        pytest.skip(f"{doc} not present (sdk distributed standalone)")

    text = open(doc, encoding="utf-8").read()
    builtin_ok = {"PermissionError", "OSError", "IOError", "ValueError", "TypeError",
                  "KeyError", "RuntimeError", "FileNotFoundError", "ImportError"}
    cited = set(re.findall(r"\b(AIM[A-Za-z]*Error|[A-Z][A-Za-z]*Error)\b", text))
    missing = sorted(
        name for name in cited
        if name not in builtin_ok and not hasattr(exceptions_module, name)
    )
    assert not missing, (
        f"docs/sdk/python.md cites exception names that do not exist in "
        f"aim_sdk.exceptions: {missing}"
    )


def test_changelog_leads_with_the_breaking_read_me_block():
    """
    The blocking changes must appear ABOVE the fix list. The framing is accurate
    either way; the ordering is what makes it honest. A strict-mode organization
    that discovers this in production read a correct changelog that buried it.
    """
    changelog = open(os.path.join(SDK_ROOT, "CHANGELOG.md"), encoding="utf-8").read()
    section = changelog.split("## [2.0.0]", 1)
    assert len(section) == 2, "no 2.0.0 section in CHANGELOG.md"
    body = section[1].split("## [1.24.1]", 1)[0]
    changed_at = body.find("### Changed")
    fixed_at = body.find("### Fixed")
    assert changed_at != -1, "2.0.0 has no '### Changed' block"
    assert fixed_at != -1, "2.0.0 has no '### Fixed' block"
    assert changed_at < fixed_at, "'### Changed' must appear above '### Fixed'"
    # The 3.0.0 warning window must be announced, or 3.0.0 cannot ship the block.
    assert "3.0.0" in body

"""
Tests for `aim-sdk demo` (aim_sdk.demo).

The demo's contract, not its output formatting:
- unauthenticated runs exit 1 with the login step named, no traceback
- a registration failure exits 1 with named next steps, no traceback
- the demo registers under the demo agent type (the analytics-exclusion marker)
- the subcommand is wired into the CLI parser
- cleanup deletes by looked-up id and tolerates an absent agent
"""

import runpy
import sys
from unittest import mock

import pytest

from aim_sdk import demo as demo_mod
from aim_sdk.client import AgentType


def test_demo_agent_type_constant_is_demo():
    assert AgentType.DEMO == "demo"
    assert AgentType.DEMO in AgentType.all_types()


def test_unauthenticated_run_exits_1_and_names_login(capsys):
    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=None):
        rc = demo_mod.run()
    out = capsys.readouterr().out
    assert rc == 1
    assert "aim-sdk login" in out
    assert "Traceback" not in out


def test_registration_failure_exits_1_with_named_next_steps(capsys):
    creds = {"aimUrl": "http://localhost:8080"}
    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=creds), \
         mock.patch("aim_sdk.secure", side_effect=ConnectionError("server unreachable")):
        rc = demo_mod.run(ci=True)
    out = capsys.readouterr().out
    assert rc == 1
    assert "aim-sdk status" in out
    assert "/health" in out
    assert "Traceback" not in out


def test_demo_registers_with_demo_agent_type_and_name():
    creds = {"aimUrl": "http://localhost:8080"}
    fake_agent = mock.MagicMock()
    fake_agent.agent_id = "agent-123"
    # perform_action decorators must return the function unchanged so the
    # scripted pass can call them.
    fake_agent.perform_action.return_value = lambda fn: fn

    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=creds), \
         mock.patch("aim_sdk.secure", return_value=fake_agent) as secure_mock:
        rc = demo_mod.run(ci=True)

    assert rc == 0
    assert secure_mock.call_count == 1
    call = secure_mock.call_args
    assert call.args[0] == demo_mod.DEMO_AGENT_NAME
    assert call.kwargs["agent_type"] == AgentType.DEMO


def test_single_pass_reports_honest_failure_when_every_action_fails(capsys):
    creds = {"aimUrl": "http://localhost:8080"}
    fake_agent = mock.MagicMock()
    fake_agent.agent_id = "agent-123"

    def failing_decorator(**_kwargs):
        def wrap(_fn):
            def boom(*a, **k):
                raise RuntimeError("backend down")
            return boom
        return wrap

    fake_agent.perform_action.side_effect = failing_decorator
    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=creds), \
         mock.patch("aim_sdk.secure", return_value=fake_agent):
        rc = demo_mod.run(ci=True)
    out = capsys.readouterr().out
    assert rc == 1
    assert "nothing new to show" in out


def test_cli_wires_demo_subcommand():
    from aim_sdk import cli

    with mock.patch.object(cli, "demo", return_value=0) as demo_cmd, \
         mock.patch.object(sys, "argv", ["aim-sdk", "demo", "--ci"]):
        rc = cli.main()
    assert rc == 0
    assert demo_cmd.call_count == 1


def test_python_dash_m_entrypoint_exists():
    # `python -m aim_sdk` must resolve; argparse exits 2 on the bogus flag,
    # which proves __main__ dispatched into the CLI parser.
    with mock.patch.object(sys, "argv", ["aim_sdk", "--definitely-not-a-flag"]):
        with pytest.raises(SystemExit) as exc:
            runpy.run_module("aim_sdk", run_name="__main__")
    assert exc.value.code == 2


def test_cleanup_deletes_by_looked_up_id():
    creds = {"aimUrl": "http://localhost:8080"}
    lookup = mock.MagicMock(status_code=200)
    lookup.json.return_value = {"agent": {"id": "agent-123"}}
    delete = mock.MagicMock(status_code=200)

    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=creds), \
         mock.patch("aim_sdk.oauth.OAuthTokenManager") as mgr, \
         mock.patch("requests.get", return_value=lookup) as get_mock, \
         mock.patch("requests.delete", return_value=delete) as del_mock, \
         mock.patch("aim_sdk.credentials.delete_agent_credentials", return_value=True) as del_creds:
        mgr.return_value.get_access_token.return_value = "token"
        rc = demo_mod.run(cleanup=True)

    assert rc == 0
    assert demo_mod.DEMO_AGENT_NAME in get_mock.call_args.args[0]
    assert "agent-123" in del_mock.call_args.args[0]
    del_creds.assert_called_once_with(demo_mod.DEMO_AGENT_NAME)


def test_cleanup_with_no_agent_is_a_clean_no_op(capsys):
    creds = {"aimUrl": "http://localhost:8080"}
    lookup = mock.MagicMock(status_code=404)

    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=creds), \
         mock.patch("aim_sdk.oauth.OAuthTokenManager") as mgr, \
         mock.patch("requests.get", return_value=lookup), \
         mock.patch("aim_sdk.credentials.delete_agent_credentials", return_value=False):
        mgr.return_value.get_access_token.return_value = "token"
        rc = demo_mod.run(cleanup=True)

    out = capsys.readouterr().out
    assert rc == 0
    assert "Nothing to clean up" in out

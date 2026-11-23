"""
In sign-in mode, secure() / register_agent() send the stored sign-in's access
token to the server that sign-in is for, and to no other server.
OAuthTokenManager obtains and refreshes that token at the sign-in's aimUrl, so
an aim_url naming another server would carry a token one server issued to a
server that did not issue it. Such an aim_url is refused before any request,
with the message `aim-sdk demo --cleanup` gives and the sign-in to run first.
An aim_url naming the sign-in's server, with or without a trailing "/", and no
aim_url, register as before.

Before this change secure() sent POST /api/v1/agents with the stored sign-in's
bearer token to the server aim_url named.
"""
from unittest import mock

import pytest

from aim_sdk import secure
from aim_sdk.exceptions import ConfigurationError

SIGNED_IN = "http://127.0.0.1:18080"
OTHER = "http://127.0.0.1:18081"


def sign_in_for(url):
    return {"aimUrl": url, "refreshToken": "rt", "accessToken": "at"}


def run_secure(creds, **kwargs):
    with mock.patch("aim_sdk.client._load_credentials", return_value=None), \
         mock.patch("aim_sdk.client.load_sdk_credentials", return_value=creds), \
         mock.patch("aim_sdk.client.OAuthTokenManager") as manager, \
         mock.patch("aim_sdk.client._register_via_oauth") as register, \
         mock.patch("requests.Session.request") as session_request, \
         mock.patch("requests.request") as plain_request:
        register.return_value = mock.MagicMock()
        try:
            secure("my-agent", auto_detect=False, auto_hooks=False, **kwargs)
            error = None
        except ConfigurationError as e:
            error = e
    return error, manager, register, session_request, plain_request


def test_an_aim_url_naming_another_server_is_refused_before_any_request():
    error, manager, register, session_request, plain_request = run_secure(sign_in_for(SIGNED_IN), aim_url=OTHER)

    assert error is not None
    assert str(error).splitlines() == [
        f"The sign-in stored on this machine is for another server, so its token is not sent to {OTHER}.",
        f"To register there, sign in to it first: aim-sdk login --force --url {OTHER}",
    ]
    register.assert_not_called()
    manager.assert_not_called()
    session_request.assert_not_called()
    plain_request.assert_not_called()


def test_a_sign_in_storing_no_address_is_for_the_default_server():
    creds = {"refreshToken": "rt", "accessToken": "at"}

    error, _, register, _, _ = run_secure(creds, aim_url=OTHER)

    assert error is not None
    assert f"aim-sdk login --force --url {OTHER}" in str(error)
    register.assert_not_called()


def test_the_refusal_does_not_repeat_a_stored_address_with_a_user_name_or_password():
    stored = sign_in_for("http://FAKE-user:FAKE-password@127.0.0.1:1")

    error, _, register, _, _ = run_secure(stored, aim_url=OTHER)

    assert error is not None
    assert "FAKE-user" not in str(error) and "FAKE-password" not in str(error)
    assert f"aim-sdk login --force --url {OTHER}" in str(error)
    register.assert_not_called()


@pytest.mark.parametrize("aim_url", [None, SIGNED_IN, SIGNED_IN + "/"],
                         ids=["no-aim_url", "as-stored", "trailing-slash"])
def test_the_signed_in_server_is_registered_with(aim_url):
    kwargs = {} if aim_url is None else {"aim_url": aim_url}

    error, _, register, _, _ = run_secure(sign_in_for(SIGNED_IN), **kwargs)

    assert error is None
    register.assert_called_once()
    assert register.call_args.kwargs["aim_url"] in (SIGNED_IN, SIGNED_IN + "/")

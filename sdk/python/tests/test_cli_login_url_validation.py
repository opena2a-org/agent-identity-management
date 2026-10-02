"""
#408: `aim-sdk login` with an empty --url or a string that is not an http(s)
URL must fail at once and name the actual problem. On main before this change
all three inputs from the issue already failed within the probe's bound, but
the empty and non-URL cases reported "could not reach the AIM server ... no
HTTP response within 5s": a network failure that never happened.
"""
import io
from contextlib import redirect_stdout
from types import SimpleNamespace

import pytest

from aim_sdk import cli


@pytest.fixture
def no_network(monkeypatch):
    calls = []

    def refuse(*args, **kwargs):
        calls.append(args)
        raise AssertionError("login touched the network for an invalid --url")

    monkeypatch.setattr(cli.requests, "get", refuse)
    monkeypatch.setattr(cli.requests, "post", refuse)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    return calls


def _login(url):
    buf = io.StringIO()
    with redirect_stdout(buf):
        rc = cli.login(SimpleNamespace(url=url, force=True))
    return rc, buf.getvalue()


@pytest.mark.parametrize("url, problem", [
    ("", "--url is empty"),
    ("   ", "--url is empty"),
    ("not-a-url", "is not an http(s) URL"),
    ("ftp://aim.example.com", "is not an http(s) URL"),
    ("localhost:8080", "is not an http(s) URL"),
    ("http://", "has no host"),
])
def test_an_invalid_url_fails_before_any_request_and_names_the_problem(no_network, url, problem):
    rc, out = _login(url)
    assert rc == 1
    assert problem in out
    assert "aim-sdk login --url https://" in out
    assert "could not reach" not in out
    assert no_network == []


@pytest.mark.parametrize("url", ["http://localhost:8080", "https://aim.example.com/", " https://aim.example.com "])
def test_a_valid_url_passes_validation(url):
    assert cli.invalid_server_url_reason(url) is None


def test_an_unreachable_valid_url_still_reports_unreachable(monkeypatch):
    monkeypatch.setattr(cli, "check_server_reachable", lambda url, timeout: False)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    rc, out = _login("http://127.0.0.1:1")
    assert rc == 1
    assert "could not reach the AIM server at http://127.0.0.1:1" in out

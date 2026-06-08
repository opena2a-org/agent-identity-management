"""
Regression tests for the SDK CLI OAuth token exchange error handling.

Guards against the doubled "Token exchange failed: Token exchange failed: <status>"
prefix: exchange_code_for_tokens() returned a pre-prefixed string and login()
prefixed it again. The reason returned here must be BARE (the caller adds the
single "Token exchange failed: " prefix) and should prefer the server's
human-readable error_description.
"""

from unittest.mock import patch, MagicMock

from aim_sdk.cli import exchange_code_for_tokens

ARGS = ("https://api.aim.opena2a.org", "code", "verifier", "http://localhost:1234/callback")


def _mock_response(status_code, *, json_body=None, content=b"x", raises_json=False):
    resp = MagicMock()
    resp.status_code = status_code
    resp.content = content
    if raises_json:
        resp.json.side_effect = ValueError("No JSON")
    else:
        resp.json.return_value = json_body
    return resp


def test_reason_is_bare_not_preprefixed():
    """The returned error must NOT itself contain 'Token exchange failed:' —
    the caller adds that exactly once, so a pre-prefixed value doubles it."""
    resp = _mock_response(400, json_body={"error": "invalid_grant",
                                          "error_description": "Authorization code is invalid or expired"})
    with patch("aim_sdk.cli.requests.post", return_value=resp):
        out = exchange_code_for_tokens(*ARGS)
    assert "Token exchange failed:" not in out["error"], out["error"]
    # And the single-prefix composition the caller prints is not doubled:
    rendered = f"Token exchange failed: {out['error']}"
    assert rendered.count("Token exchange failed:") == 1, rendered


def test_prefers_error_description():
    resp = _mock_response(400, json_body={"error": "invalid_grant",
                                          "error_description": "Authorization code is invalid or expired"})
    with patch("aim_sdk.cli.requests.post", return_value=resp):
        out = exchange_code_for_tokens(*ARGS)
    assert out["error"] == "Authorization code is invalid or expired"


def test_falls_back_to_error_field():
    resp = _mock_response(400, json_body={"error": "invalid_grant"})
    with patch("aim_sdk.cli.requests.post", return_value=resp):
        out = exchange_code_for_tokens(*ARGS)
    assert out["error"] == "invalid_grant"


def test_non_json_body_gives_status_and_url_hint_without_crashing():
    """A 405 with an HTML body (classic symptom of pointing at the dashboard/
    frontend URL) must not raise and must hint at the URL, not double a prefix."""
    resp = _mock_response(405, content=b"<html>405</html>", raises_json=True)
    with patch("aim_sdk.cli.requests.post", return_value=resp):
        out = exchange_code_for_tokens(*ARGS)
    assert "405" in out["error"]
    assert "api.aim.opena2a.org" in out["error"]  # actionable URL hint
    assert "Token exchange failed:" not in out["error"]
    assert f"Token exchange failed: {out['error']}".count("Token exchange failed:") == 1


if __name__ == "__main__":
    import pytest
    pytest.main([__file__, "-v"])

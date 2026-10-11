"""
What decides an @perform_action call, as the docs and `aim-sdk login` say it.

The SDK posts each decorated call to /api/v1/sdk-api/verifications. The server
decides that request with AgentService.VerifyCapability: the capabilities
granted to the agent, the organization's enforcement mode and its security
policies. The 5-step Fine-Grained Authorization engine (FGAEngine.Authorize)
runs on POST /api/v1/agents/:id/authorize, which no SDK here calls. The
READMEs and the login closing text said each decorated call ran through the
5-step engine (#656).
"""
import base64
import io
import json
import re
from contextlib import redirect_stdout
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import MagicMock

import pytest

from aim_sdk import cli

SDK_DIR = Path(__file__).resolve().parents[1]
REPO = SDK_DIR.parents[1]
READMES = {
    "README.md": REPO / "README.md",
    "sdk/python/README.md": SDK_DIR / "README.md",
    "sdk/java/README.md": REPO / "sdk" / "java" / "README.md",
}
DECORATOR = re.compile(r"perform_action|performAction\(\)")
ENGINE = re.compile(r"5-step|\bFGA\b|Fine-Grained")

AIM = "http://localhost:8080"
DEVICE = {
    "deviceCode": "d" * 80,
    "userCode": "BCDF-GHJK",
    "verificationUri": "http://localhost:3000/device",
    "verificationUriComplete": "http://localhost:3000/device?user_code=BCDF-GHJK",
    "expiresIn": 900,
    "interval": 5,
}


def _jwt(claims):
    def seg(d):
        return base64.urlsafe_b64encode(json.dumps(d).encode()).decode().rstrip("=")
    return f"{seg({'alg': 'HS256', 'typ': 'JWT'})}.{seg(claims)}.c2ln"


PAIR = {
    "accessToken": _jwt({"user_id": "u-1", "organization_id": "o-1", "email": "dev@example.com",
                         "role": "admin", "typ": "access", "iss": "agent-identity-management",
                         "exp": 4102444800}),
    "refreshToken": _jwt({"user_id": "u-1", "organization_id": "o-1", "typ": "refresh",
                          "iss": "agent-identity-management", "exp": 4102444800, "jti": "j-1"}),
    "tokenType": "Bearer",
    "expiresIn": 7200,
}


def _resp(status, body):
    r = MagicMock()
    r.status_code = status
    r.content = json.dumps(body).encode()
    r.text = json.dumps(body)
    r.headers = {"content-type": "application/json"}
    r.json.return_value = body
    return r


def _post(url, json=None, headers=None, timeout=None):
    if url == f"{AIM}/api/v1/oauth/device/code":
        return _resp(200, DEVICE)
    if url == f"{AIM}/api/v1/oauth/device/token":
        return _resp(200, PAIR)
    return _resp(404, {"error": "no such route"})


@pytest.fixture
def login_output(monkeypatch):
    monkeypatch.setattr(cli.requests, "post", _post)
    monkeypatch.setattr(cli.requests, "get", lambda *a, **k: _resp(200, {}))
    monkeypatch.setattr(cli.time, "sleep", lambda n: None)
    monkeypatch.setattr(cli.webbrowser, "open", lambda url, *a, **k: True)
    monkeypatch.setattr("aim_sdk.credentials.save_sdk_credentials", lambda c: True)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    buf = io.StringIO()
    with redirect_stdout(buf):
        rc = cli.login(SimpleNamespace(url=AIM, force=True))
    out = buf.getvalue()
    assert rc == 0, out
    return " ".join(out.split())


def test_login_closing_text_does_not_say_the_five_step_engine_decides_a_call(login_output):
    assert "5-step" not in login_output
    assert "Fine-Grained Authorization" not in login_output


def test_login_closing_text_names_what_the_server_checks_and_the_enforcement_modes(login_output):
    assert "checked by the server against the agent's granted capabilities" in login_output
    assert "security policies" in login_output
    assert "In strict enforcement mode, calls outside the granted capabilities are denied" in login_output
    assert "in monitoring mode, the default, they run and are recorded" in login_output


@pytest.mark.parametrize("name", sorted(READMES))
def test_readme_does_not_say_the_decorator_runs_through_the_five_step_engine(name):
    lines = READMES[name].read_text(encoding="utf-8").splitlines()
    claims = [line for line in lines if DECORATOR.search(line) and ENGINE.search(line)]
    for line in claims:
        assert "does not decide" in line, f"{name}: {line}"
    stated = [line for line in lines if DECORATOR.search(line)
              and "granted" in line and "security policies" in line]
    assert stated, f"{name} must say the server checks the call against grants and security policies"


def _fga_section(readme):
    start = readme.index("### 5-step Fine-Grained Authorization")
    end = readme.index("\n### ", start + 1)
    return readme[start:end]


def test_root_readme_names_the_route_the_five_step_engine_runs_on():
    readme = READMES["README.md"].read_text(encoding="utf-8")
    section = _fga_section(readme)
    assert "Every privileged action runs through five checks" not in readme
    assert "`POST /api/v1/agents/:id/authorize`" in section
    assert "it runs step 1 only" in section
    assert "never denies" in section
    assert "The SDKs do not call this route" in section
    assert "`/api/v1/sdk-api/verifications`" in section


def test_root_readme_does_not_say_every_decision_has_five_child_spans():
    readme = READMES["README.md"].read_text(encoding="utf-8")
    assert "with 5 child spans" not in readme
    assert "one child span per FGA step it ran" in readme


def test_the_sdk_posts_verifications_to_the_route_the_docs_name_and_never_to_authorize():
    # The docs' claim is pinned to the client: if the SDK starts calling the
    # authorize route, the sentences above must change with it.
    sources = {p.name: p.read_text(encoding="utf-8") for p in (SDK_DIR / "aim_sdk").rglob("*.py")}
    assert 'endpoint = "/api/v1/sdk-api/verifications"' in sources["client.py"]
    callers = [name for name, src in sources.items() if re.search(r"/agents/[^\"'\s]*/authorize", src)]
    assert callers == []

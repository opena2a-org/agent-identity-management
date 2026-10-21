"""
What decides an @perform_action call, as the docs and `aim-sdk login` say it.

The SDK posts each decorated call to /api/v1/sdk-api/verifications. The server
decides that request with FGAEngine.Authorize, the 5-step Fine-Grained
Authorization engine, with AgentService.VerifyCapability as step 1: the
capabilities granted to the agent, the organization's enforcement mode and its
security policies. Steps 2 to 5 run on the agent's FGA policy for the
capability, or on the default policy when it has none. The READMEs first said
each decorated call ran through the 5-step engine while the route decided with
VerifyCapability alone, then said the engine did not decide these calls; the
route now runs through it (#656).
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


def test_login_closing_text_names_what_the_server_checks_and_the_enforcement_modes(login_output):
    assert "checked by the server against the agent's granted capabilities" in login_output
    assert "security policies" in login_output
    assert "In strict enforcement mode, calls outside the granted capabilities are denied" in login_output
    assert "in monitoring mode, the default, they run and are recorded" in login_output


@pytest.mark.parametrize("name", sorted(READMES))
def test_readme_says_the_decorator_runs_through_the_five_step_engine(name):
    lines = READMES[name].read_text(encoding="utf-8").splitlines()
    claims = [line for line in lines if DECORATOR.search(line) and ENGINE.search(line)]
    assert claims, f"{name} must say the decorated call runs through the 5-step engine"
    for line in claims:
        assert "5-step Fine-Grained Authorization engine" in line, f"{name}: {line}"
        assert "does not decide" not in line, f"{name}: {line}"
    stated = [line for line in lines if DECORATOR.search(line)
              and "granted" in line and "security policies" in line]
    assert stated, f"{name} must say the server checks the call against grants and security policies"


# The root README links these guides for the engine and for observability; the
# passages that describe them live there.
SERVER_FEATURES = REPO / "docs" / "server-features.md"
DEPLOYMENT = REPO / "docs" / "DEPLOYMENT.md"


def _fga_section(guide):
    start = guide.index("## 5-step Fine-Grained Authorization")
    end = guide.index("\n## ", start + 1)
    return guide[start:end]


def test_server_features_guide_names_both_routes_the_five_step_engine_runs_on():
    readme = READMES["README.md"].read_text(encoding="utf-8")
    guide = SERVER_FEATURES.read_text(encoding="utf-8")
    section = _fga_section(guide)
    assert "Every privileged action runs through five checks" not in readme
    assert "Every privileged action runs through five checks" not in guide
    assert "`POST /api/v1/agents/:id/authorize`" in section
    assert "`POST /api/v1/sdk-api/verifications`" in section
    assert "it runs step 1 only" not in section
    assert "decided under a default policy" in section
    assert "never denies" in section
    assert "The SDKs do not call `POST /api/v1/agents/:id/authorize`" in section
    assert "in monitoring mode it runs" in section
    assert "`fgaDeniedBy`" in section


def test_deployment_guide_says_which_routes_carry_the_step_spans():
    readme = READMES["README.md"].read_text(encoding="utf-8")
    guide = DEPLOYMENT.read_text(encoding="utf-8")
    assert "with 5 child spans" not in readme
    assert "with 5 child spans" not in guide
    assert "one child span per FGA step it ran" in guide
    assert "and on `POST /api/v1/sdk-api/verifications`" in guide
    assert "the decision is an SDK verification's" not in guide


def test_the_sdk_posts_verifications_to_the_route_the_docs_name_and_never_to_authorize():
    # The docs' claim is pinned to the client: if the SDK starts calling the
    # authorize route, the sentences above must change with it.
    sources = {p.name: p.read_text(encoding="utf-8") for p in (SDK_DIR / "aim_sdk").rglob("*.py")}
    assert 'endpoint = "/api/v1/sdk-api/verifications"' in sources["client.py"]
    callers = [name for name, src in sources.items() if re.search(r"/agents/[^\"'\s]*/authorize", src)]
    assert callers == []

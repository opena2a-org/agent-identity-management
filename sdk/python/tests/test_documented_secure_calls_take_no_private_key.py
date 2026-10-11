"""
The secure() and AIMClient() calls in the documentation run as written (#665).

docs/sdk/python.md, docs/sdk/authentication.md, the LangChain and CrewAI
integration guides and the guides under docs/examples/ passed ``private_key=``
to ``secure()``. ``secure()`` has no such parameter: it generates the agent's
keypair itself and saves it to ``~/.aim/agents/<name>.json``. A keypair the
caller holds goes to ``AIMClient(agent_id=..., public_key=..., private_key=...,
aim_url=...)``.

These tests read every ``secure(``, ``register_agent(`` and ``AIMClient(`` call
in the documentation and bind its arguments to the SDK's own signatures, hold
the parameter list python.md documents for ``secure()`` to the real one, and
build an AIMClient the way authentication.md shows, from the keypair file
``secure()`` saves.
"""

import ast
import base64
import inspect
import re
from pathlib import Path

import pytest

import aim_sdk.client as client_module
import aim_sdk.credentials as credentials_module
from aim_sdk import AIMClient, load_agent_credentials, secure

SDK_DIR = Path(__file__).resolve().parent.parent
REPO_ROOT = SDK_DIR.parent.parent

CALL = re.compile(r"(?<![\w.])(secure|register_agent|AIMClient)\(")

# The files #665 named; each documents a keypair passed to the SDK.
ISSUE_FILES = (
    "docs/sdk/python.md",
    "docs/sdk/authentication.md",
    "docs/integrations/langchain.md",
    "docs/integrations/crewai.md",
    "docs/examples/weather-agent.md",
    "docs/examples/flight-tracker.md",
    "docs/examples/database-agent.md",
)


def _doc_paths():
    paths = set((REPO_ROOT / "docs").rglob("*.md")) | set((SDK_DIR / "docs").rglob("*.md"))
    paths |= {REPO_ROOT / "README.md", SDK_DIR / "README.md"}
    return sorted(paths)


def _calls(path):
    """Yield (line, callee, call text) for each documented call in ``path``."""
    text = path.read_text(encoding="utf-8")
    for match in CALL.finditer(text):
        end, depth = match.end(), 1
        while end < len(text) and depth:
            depth += {"(": 1, ")": -1}.get(text[end], 0)
            end += 1
        line = text.count("\n", 0, match.start()) + 1
        yield line, match.group(1), text[match.start():end]


def _parsed(call):
    try:
        return ast.parse(call, mode="eval").body
    except SyntaxError:
        return None


def _site(path, line):
    return f"{path.relative_to(REPO_ROOT)}:{line}"


def test_no_documented_secure_call_passes_private_key():
    sites = []
    for path in _doc_paths():
        for line, callee, call in _calls(path):
            node = _parsed(call)
            if callee == "AIMClient" or node is None:
                continue
            if any(keyword.arg == "private_key" for keyword in node.keywords):
                sites.append(_site(path, line))
    assert sites == [], (
        "secure() has no private_key parameter; pass a keypair you hold to AIMClient: "
        + ", ".join(sites)
    )


def test_every_documented_aimclient_call_binds_to_its_signature():
    signature = inspect.signature(AIMClient)
    checked, failures = 0, []
    for path in _doc_paths():
        for line, callee, call in _calls(path):
            if callee != "AIMClient":
                continue
            node = _parsed(call)
            if node is None:
                failures.append(f"{_site(path, line)}: not a Python call")
                continue
            arguments = {keyword.arg: None for keyword in node.keywords if keyword.arg}
            try:
                signature.bind_partial(*[None] * len(node.args), **arguments)
            except TypeError as error:
                failures.append(f"{_site(path, line)}: {error}")
            checked += 1
    assert failures == []
    assert checked > 0


@pytest.mark.parametrize("relative", ISSUE_FILES)
def test_issue_files_call_aimclient_with_every_key_it_needs(relative):
    """A keypair given to AIMClient comes with the agent ID and public key it belongs to."""
    path = REPO_ROOT / relative
    assert path.is_file(), relative
    for line, callee, call in _calls(path):
        if callee != "AIMClient":
            continue
        node = _parsed(call)
        assert node is not None, _site(path, line)
        names = {keyword.arg for keyword in node.keywords}
        if "private_key" in names:
            assert {"agent_id", "public_key", "aim_url"} <= names, _site(path, line)


def test_python_md_secure_parameters_are_secure_parameters():
    text = (REPO_ROOT / "docs/sdk/python.md").read_text(encoding="utf-8")
    section = text.split("### `secure()` - The Magic Function", 1)[1].split("**Returns**", 1)[0]
    listed = re.findall(r"^    (\w+): ", section, flags=re.MULTILINE)
    described = re.findall(r"^- \*\*(\w+)\*\*", section, flags=re.MULTILINE)
    real = set(inspect.signature(secure).parameters)
    assert listed and described
    assert [name for name in listed + described if name not in real] == []


def test_aimclient_runs_from_the_keypair_secure_saves(tmp_path, monkeypatch):
    """authentication.md, Option 2: load the saved keypair and pass it to AIMClient."""
    from nacl.encoding import Base64Encoder
    from nacl.signing import SigningKey

    monkeypatch.setattr(credentials_module, "AGENTS_DIR", tmp_path / "agents")
    monkeypatch.setattr(client_module, "load_sdk_credentials", lambda *args, **kwargs: None)

    signing_key = SigningKey.generate()
    public_key = signing_key.verify_key.encode(encoder=Base64Encoder).decode("utf-8")
    seed_and_public = bytes(signing_key) + bytes(signing_key.verify_key)
    client_module._save_credentials(
        "my-agent",
        {
            "agent_id": "550e8400-e29b-41d4-a716-446655440000",
            "public_key": public_key,
            "private_key": base64.b64encode(seed_and_public).decode("utf-8"),
            "aim_url": "http://localhost:8080",
        },
    )

    creds = load_agent_credentials("my-agent")
    agent = AIMClient(
        agent_id=creds["agent_id"],
        public_key=creds["public_key"],
        private_key=creds["private_key"],
        aim_url=creds["aim_url"],
    )

    assert agent.agent_id == "550e8400-e29b-41d4-a716-446655440000"
    assert agent.public_key == public_key
    assert bytes(agent.signing_key) == bytes(signing_key)


def test_secure_still_refuses_private_key():
    """The documented calls were wrong, not secure(): it takes no keypair."""
    assert "private_key" not in inspect.signature(secure).parameters
    with pytest.raises(TypeError, match="private_key"):
        secure("my-agent", private_key="x")

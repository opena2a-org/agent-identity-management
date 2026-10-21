"""
Every aim_sdk name the Python SDK pages document exists.

The pages documented AIMClient methods the class never had
(``export_compliance_report``, ``rotate_key``, ``submit_feedback`` ...),
functions and modules the package never shipped (``auto_detect_mcp_servers``,
``aim_sdk.crypto.generate_keypair``, ``aim_sdk.validation`` ...) and keyword
arguments the callables do not take (``AIMCallbackHandler(aim_agent=...)``).
This test reads each page's python blocks and checks them against the package:

- every ``from aim_sdk... import name`` and ``import aim_sdk...`` resolves;
- every attribute read on a variable bound to an AIMClient is a member of it;
- every keyword passed to an imported aim_sdk callable, or to an AIMClient
  method, is one of its parameters.

The integration packages import their framework (langchain, crewai) at import
time, so names under ``aim_sdk.integrations`` are resolved from the source.
"""

import ast
import importlib
import inspect
import pathlib
import re

import pytest

import aim_sdk
from aim_sdk import AIMClient

PKG = pathlib.Path(aim_sdk.__file__).resolve().parent
REPO = PKG.parents[2]

PAGES = [
    "docs/sdk/python.md",
    "docs/sdk/authentication.md",
    "docs/sdk/auto-detection.md",
    "docs/sdk/trust-scoring.md",
    "docs/guides/A2A.md",
    "docs/guides/PQC.md",
    "docs/integrations/langchain.md",
    "docs/integrations/crewai.md",
    "docs/integrations/mcp.md",
    "docs/DOCUMENTATION_INDEX.md",
    "docs/troubleshooting/authentication.md",
    "docs/troubleshooting/README.md",
    "sdk/python/docs/MCP_INTEGRATION.md",
    "sdk/python/docs/LANGCHAIN_INTEGRATION.md",
    "sdk/python/docs/CREWAI_INTEGRATION.md",
]

# Calls whose result is an AIMClient: secure is register_agent's alias.
CLIENT_FACTORIES = {"AIMClient", "secure", "register_agent"}

# Snippets often skip the import line; these names mean the SDK's own.
UNIMPORTED = {
    "AIMClient": ("aim_sdk", "AIMClient"),
    "secure": ("aim_sdk", "secure"),
    "register_agent": ("aim_sdk", "register_agent"),
    "AIMCallbackHandler": ("aim_sdk.integrations.langchain", "AIMCallbackHandler"),
    "wrap_tools_with_aim": ("aim_sdk.integrations.langchain", "wrap_tools_with_aim"),
    "AIMCrewWrapper": ("aim_sdk.integrations.crewai", "AIMCrewWrapper"),
    "AIMTaskCallback": ("aim_sdk.integrations.crewai", "AIMTaskCallback"),
    "aim_verified_task": ("aim_sdk.integrations.crewai", "aim_verified_task"),
}

# A page that never binds `client` still means an AIMClient by it.
CARRIED_AT_START = {"client"}

FENCE_OPEN = re.compile(r"^\s*```\s*(\w*)\s*$")
IMPORT_FROM = re.compile(r"^\s*from\s+(aim_sdk[\w.]*)\s+import\s+\(?([^)#\n]*)", re.M)
IMPORT_PLAIN = re.compile(r"^\s*import\s+(aim_sdk[\w.]*)", re.M)
CLIENT_BINDING = re.compile(
    r"^\s*(\w+)\s*=\s*(?:secure|register_agent|AIMClient(?:\.from_credentials|\.auto_register_or_load)?)\(",
    re.M,
)

MISSING = object()


def _python_blocks(text):
    """Yield (first line number, source) for each ```python block."""
    lang, buf, start = None, [], 0
    for number, line in enumerate(text.splitlines(), 1):
        if lang is None:
            match = FENCE_OPEN.match(line)
            if match:
                lang, buf, start = match.group(1).lower(), [], number + 1
        elif line.strip() == "```":
            if lang in ("python", "py"):
                yield start, "\n".join(buf)
            lang = None
        else:
            buf.append(line)


def _client_members():
    members = set(dir(AIMClient))
    tree = ast.parse((PKG / "client.py").read_text())
    cls = next(n for n in tree.body if isinstance(n, ast.ClassDef) and n.name == "AIMClient")
    for node in ast.walk(cls):
        if isinstance(node, ast.Attribute) and isinstance(node.value, ast.Name) and node.value.id == "self":
            if isinstance(node.ctx, ast.Store):
                members.add(node.attr)
    return members


CLIENT_MEMBERS = _client_members()


class _Static:
    """A name found in a module's source; params is None when any keyword is accepted."""

    def __init__(self, params):
        self.params = params


def _static_params(node):
    if isinstance(node, ast.ClassDef):
        init = next((n for n in node.body if isinstance(n, ast.FunctionDef) and n.name == "__init__"), None)
        if init is None:
            return None
        node = init
    if node.args.kwarg is not None:
        return None
    return {a.arg for a in node.args.posonlyargs + node.args.args + node.args.kwonlyargs} - {"self"}


def _static_resolve(module, name):
    rel = pathlib.Path(*module.split(".")[1:])
    package = PKG / rel
    init = package / "__init__.py" if package.is_dir() else PKG / rel.with_suffix(".py")
    if not init.is_file():
        return MISSING
    if name is None:
        return _Static(None)
    tree = ast.parse(init.read_text())
    exported = None
    for node in tree.body:
        if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == "__all__" for t in node.targets):
            exported = set(ast.literal_eval(node.value))
    if exported is not None and name not in exported:
        return MISSING
    sources = sorted(package.glob("*.py")) if package.is_dir() else [init]
    for source in sources:
        for node in ast.parse(source.read_text()).body:
            if isinstance(node, (ast.FunctionDef, ast.ClassDef)) and node.name == name:
                return _Static(_static_params(node))
    if (package / f"{name}.py").is_file():
        return _Static(None)
    return MISSING if exported is None else _Static(None)


def _resolve(module, name=None):
    if module.startswith("aim_sdk.integrations"):
        return _static_resolve(module, name)
    try:
        mod = importlib.import_module(module)
    except ImportError:
        return MISSING
    if name is None:
        return mod
    if hasattr(mod, name):
        return getattr(mod, name)
    try:
        return importlib.import_module(f"{module}.{name}")
    except ImportError:
        return MISSING


def _params(obj):
    """Keyword names obj accepts, or None when it accepts any keyword."""
    if isinstance(obj, _Static):
        return obj.params
    if inspect.ismodule(obj) or not callable(obj):
        return None
    try:
        sig = inspect.signature(obj)
    except (TypeError, ValueError):
        return None
    if any(p.kind is p.VAR_KEYWORD for p in sig.parameters.values()):
        return None
    return set(sig.parameters) - {"self"}


def _is_client_call(value):
    if not isinstance(value, ast.Call):
        return False
    func = value.func
    if isinstance(func, ast.Name):
        return func.id in CLIENT_FACTORIES
    # Any AIMClient.<constructor>(); one the class lacks is reported on its own.
    return isinstance(func, ast.Attribute) and isinstance(func.value, ast.Name) and func.value.id == "AIMClient"


def _bindings(tree):
    """(line, name, is_client, is_parameter) for each name bound in the block, in line order."""
    events = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Assign):
            for target in node.targets:
                if isinstance(target, ast.Name):
                    events.append((node.lineno, target.id, _is_client_call(node.value), False))
        elif isinstance(node, (ast.With, ast.AsyncWith)):
            for item in node.items:
                if isinstance(item.optional_vars, ast.Name):
                    events.append((node.lineno, item.optional_vars.id, _is_client_call(item.context_expr), False))
        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            # A parameter shadows the name for the rest of this block only.
            for arg in node.args.posonlyargs + node.args.args + node.args.kwonlyargs:
                events.append((node.lineno, arg.arg, False, True))
    return sorted(events)


def _is_client_at(name, line, events, carried):
    state = name in carried
    for event_line, event_name, is_client, _ in events:
        if event_line > line:
            break
        if event_name == name:
            state = is_client
    return state


def _check_parsed(tree, start, carried, problems):
    names = {name: _resolve(*where) for name, where in UNIMPORTED.items()}
    for node in ast.walk(tree):
        if isinstance(node, ast.ImportFrom) and node.module and node.module.split(".")[0] == "aim_sdk":
            for alias in node.names:
                obj = _resolve(node.module, alias.name)
                if obj is MISSING:
                    problems.append((start + node.lineno - 1, f"from {node.module} import {alias.name}"))
                else:
                    names[alias.asname or alias.name] = obj
        elif isinstance(node, ast.Import):
            for alias in node.names:
                if alias.name.split(".")[0] == "aim_sdk" and _resolve(alias.name) is MISSING:
                    problems.append((start + node.lineno - 1, f"import {alias.name}"))
    events = _bindings(tree)
    for node in ast.walk(tree):
        line = start + getattr(node, "lineno", 1) - 1
        if isinstance(node, ast.Attribute) and isinstance(node.value, ast.Name):
            base = node.value.id
            if _is_client_at(base, node.lineno, events, carried):
                if node.attr not in CLIENT_MEMBERS:
                    problems.append((line, f"{base}.{node.attr} is not an AIMClient member"))
            elif base == "aim_sdk" and _resolve("aim_sdk", node.attr) is MISSING:
                problems.append((line, f"aim_sdk.{node.attr}"))
            elif base in names and not isinstance(names[base], _Static) and not hasattr(names[base], node.attr):
                problems.append((line, f"{base}.{node.attr}"))
        if isinstance(node, ast.Call):
            func, target, label = node.func, None, None
            if isinstance(func, ast.Name) and func.id in names:
                target, label = names[func.id], func.id
            elif (
                isinstance(func, ast.Attribute)
                and isinstance(func.value, ast.Name)
                and func.attr in CLIENT_MEMBERS
                and _is_client_at(func.value.id, node.lineno, events, carried)
            ):
                target, label = getattr(AIMClient, func.attr, None), f"{func.value.id}.{func.attr}"
            accepted = _params(target) if target is not None else None
            if accepted is not None:
                for keyword in node.keywords:
                    if keyword.arg is not None and keyword.arg not in accepted:
                        problems.append((line, f"{label}() takes no {keyword.arg}="))
    for line, name, is_client, is_parameter in events:
        if is_parameter:
            continue
        if is_client:
            carried.add(name)
        else:
            carried.discard(name)


def _check_unparsed(source, start, carried, problems):
    """A block that is not valid Python (a signature sketch): imports and client members."""

    def line_of(match):
        return start + source.count("\n", 0, match.start())

    for match in IMPORT_FROM.finditer(source):
        for part in match.group(2).split(","):
            name = part.split()[0] if part.split() else None
            if name and _resolve(match.group(1), name) is MISSING:
                problems.append((line_of(match), f"from {match.group(1)} import {name}"))
    for match in IMPORT_PLAIN.finditer(source):
        if _resolve(match.group(1)) is MISSING:
            problems.append((line_of(match), f"import {match.group(1)}"))
    carried.update(CLIENT_BINDING.findall(source))
    for match in re.finditer(r"\b(\w+)\.(\w+)\b", source):
        if match.group(1) in carried and match.group(2) not in CLIENT_MEMBERS:
            problems.append((line_of(match), f"{match.group(1)}.{match.group(2)} is not an AIMClient member"))


def _check_text(text):
    """Problems in a page's python blocks, as 'line: what' strings in line order."""
    problems, carried = [], set(CARRIED_AT_START)
    for start, source in _python_blocks(text):
        try:
            tree = ast.parse(source)
        except SyntaxError:
            _check_unparsed(source, start, carried, problems)
        else:
            _check_parsed(tree, start, carried, problems)
    return [f"{line}: {what}" for line, what in sorted(set(problems))]


@pytest.mark.skipif(not (REPO / "docs").is_dir(), reason="needs the repository checkout")
@pytest.mark.parametrize("page", PAGES)
def test_page_documents_only_names_the_sdk_has(page):
    assert _check_text((REPO / page).read_text()) == []


def test_the_check_catches_each_kind_of_missing_name():
    page = "\n".join([
        "```python",
        "from aim_sdk import secure, auto_detect_mcp_servers",
        "from aim_sdk.crypto import generate_keypair",
        "from aim_sdk.integrations.langchain import AIMCallbackHandler",
        "import aim_sdk.validation",
        "agent = secure('a', private_key='k')",
        "agent.rotate_key(limit=10)",
        "handler = AIMCallbackHandler(aim_agent=agent)",
        "```",
        "```python",
        "agent.export_compliance_report()",
        "agent = make_something_else()",
        "agent.invoke('no longer a client')",
        "```",
        "```python",
        "from aim_sdk.detection import health_check_server",
        "secured = secure('b')",
        "secured.submit_feedback(rating: int) -> dict",
        "```",
    ])
    assert _check_text(page) == [
        "2: from aim_sdk import auto_detect_mcp_servers",
        "3: from aim_sdk.crypto import generate_keypair",
        "5: import aim_sdk.validation",
        "6: secure() takes no private_key=",
        "7: agent.rotate_key is not an AIMClient member",
        "8: AIMCallbackHandler() takes no aim_agent=",
        "11: agent.export_compliance_report is not an AIMClient member",
        "16: from aim_sdk.detection import health_check_server",
        "18: secured.submit_feedback is not an AIMClient member",
    ]


def test_the_check_catches_the_filters_the_pages_gave_get_audit_logs():
    # The pages called get_audit_logs with date and action filters the
    # method does not take.
    page = "\n".join([
        "```python",
        "agent = secure('a')",
        "agent.get_audit_logs(limit=100, start_date='2025-10-01T00:00:00Z', action_name='delete_user')",
        "```",
    ])
    assert _check_text(page) == [
        "3: agent.get_audit_logs() takes no action_name=",
        "3: agent.get_audit_logs() takes no start_date=",
    ]


def test_the_check_accepts_the_names_the_sdk_has():
    page = "\n".join([
        "```python",
        "from aim_sdk import secure, auto_detect_mcps, AIMClient",
        "from aim_sdk.crypto import generate_mldsa_keypair",
        "from aim_sdk.integrations.langchain import AIMCallbackHandler",
        "from aim_sdk.integrations.crewai import AIMCrewWrapper",
        "client = AIMClient.from_credentials('a')",
        "client.get_trust_score()",
        "client.get_trust_score_history(limit=5)",
        "client.get_audit_logs(limit=20, offset=40)",
        "print(client.agent_id, client.public_key)",
        "handler = AIMCallbackHandler(agent=client)",
        "crew = AIMCrewWrapper(crew=None, aim_agent=client)",
        "```",
    ])
    assert _check_text(page) == []

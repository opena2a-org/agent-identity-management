"""The aim-sdk console script pays only for the modules the command needs.

Every command imported the whole SDK before argparse ran: the entry point
imports ``aim_sdk.cli``, importing a submodule runs ``aim_sdk/__init__.py``,
and that file imported every submodule, which pulled in requests, PyJWT,
cryptography and PyNaCl: ``import aim_sdk.cli`` loaded 477 modules on a clean
Python 3.14 interpreter that starts with 76. The package now resolves its public
names on first use.

These cells pin the import set of the CLI entry point and keep the lazy name
table equal to the TYPE_CHECKING imports that type checkers read.
"""

import ast
import json
import os
import subprocess
import sys
from pathlib import Path

import pytest

SDK_ROOT = Path(__file__).resolve().parents[1]
INIT = SDK_ROOT / "aim_sdk" / "__init__.py"

# Third-party packages that only network, signing or keychain commands need.
HEAVY = ("requests", "urllib3", "jwt", "cryptography", "nacl", "keyring")

# Runs the console script's entry point the way the installed `aim-sdk` does,
# then records which modules were loaded.
_PROBE = """
import json, os, sys
sys.argv = ["aim-sdk", *json.loads(os.environ["PROBE_ARGS"])]
from aim_sdk.cli import main
try:
    main()
except SystemExit:
    pass
with open(os.environ["PROBE_OUT"], "w", encoding="utf-8") as fh:
    json.dump(sorted(sys.modules), fh)
"""


def _modules_loaded_by(tmp_path: Path, *args: str) -> set:
    home = tmp_path / "home"
    home.mkdir()
    out = tmp_path / "modules.json"
    env = {k: v for k, v in os.environ.items() if not k.startswith("AIM_")}
    env.update({
        "HOME": str(home),
        "USERPROFILE": str(home),
        "PROBE_ARGS": json.dumps(list(args)),
        "PROBE_OUT": str(out),
    })
    proc = subprocess.run(
        [sys.executable, "-c", _PROBE],
        cwd=SDK_ROOT, env=env, capture_output=True, text=True, timeout=60,
    )
    assert out.exists(), proc.stderr
    return set(json.loads(out.read_text(encoding="utf-8")))


def _heavy(modules: set) -> list:
    return sorted(m for m in modules if m.split(".")[0] in HEAVY)


@pytest.mark.parametrize("args", [("--help",), ("version",), ("--version",)])
def test_help_and_version_load_only_the_cli_module(tmp_path, args):
    modules = _modules_loaded_by(tmp_path, *args)
    assert sorted(m for m in modules if m.split(".")[0] == "aim_sdk") == ["aim_sdk", "aim_sdk.cli"]
    assert _heavy(modules) == []


def test_status_signed_out_loads_no_network_or_crypto_package(tmp_path):
    modules = _modules_loaded_by(tmp_path, "status")
    assert _heavy(modules) == []
    assert "aim_sdk.client" not in modules


def _type_checking_imports() -> dict:
    """Names imported under ``if _TYPE_CHECKING:`` in aim_sdk/__init__.py."""
    tree = ast.parse(INIT.read_text(encoding="utf-8"))
    block = next(
        node for node in tree.body
        if isinstance(node, ast.If) and getattr(node.test, "id", "") == "_TYPE_CHECKING"
    )
    names = {}
    for node in block.body:
        if isinstance(node, ast.ImportFrom):
            for alias in node.names:
                names[alias.asname or alias.name] = node.module
    return names


def test_lazy_table_equals_the_type_checking_imports():
    import aim_sdk

    assert _type_checking_imports() == aim_sdk._LAZY


def test_every_public_name_resolves_to_its_submodule_object():
    import importlib

    import aim_sdk

    for name in aim_sdk.__all__:
        value = getattr(aim_sdk, name)
        if name in aim_sdk._ALIASES:
            module, attr = aim_sdk._ALIASES[name]
        else:
            module, attr = aim_sdk._LAZY[name], name
        assert value is getattr(importlib.import_module(f"aim_sdk.{module}"), attr), name
    assert aim_sdk.secure is aim_sdk.register_agent
    assert set(aim_sdk.__all__) <= set(dir(aim_sdk))


def test_star_import_and_submodule_attributes_still_work():
    namespace = {}
    exec("from aim_sdk import *", namespace)
    assert "AIMClient" in namespace and "secure" in namespace

    import aim_sdk

    assert aim_sdk.oauth.__name__ == "aim_sdk.oauth"
    assert not hasattr(aim_sdk, "no_such_name")


def test_cli_requests_stays_patchable():
    import requests

    from aim_sdk import cli

    assert cli.requests is requests

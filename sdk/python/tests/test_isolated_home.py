"""
Every test runs against an empty temporary home (#726).

aim_sdk.credentials computes its paths from Path.home() at import time, and
load_sdk_credentials() also adopts SDK credentials from a `.aim` folder above
the working directory. A checkout inside a home directory that holds
`aim-sdk login` credentials therefore handed that sign-in to the code under
test. The isolated_home fixture in conftest.py closes both routes.

A Python child process a test starts runs that discovery itself, without the
fixture's patches, from its own working directory: the import-cost probe of
`aim-sdk status` ran in the SDK directory and copied the credentials of a
`.aim` folder above the checkout into its temporary home. The last three tests
check that the working directory is the temporary home with no `.aim` folder
above it, that no test starts a Python child with a cwd computed from a file's
location, and that the import-cost tests, run from a checkout under a home that
holds SDK credentials, leave no copy of them under the run's base temp.
"""
import ast
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

from aim_sdk import credentials, grant_client

TESTS_DIR = Path(__file__).resolve().parent
SDK_ROOT = TESTS_DIR.parent


def _inside(path, root) -> bool:
    path, root = Path(path).resolve(), Path(root).resolve()
    return path == root or root in path.parents


def test_home_is_an_empty_directory_under_the_pytest_base_temp(tmp_path_factory):
    home = Path.home()
    assert _inside(home, tmp_path_factory.getbasetemp()), home
    assert list(home.iterdir()) == []
    assert os.environ["USERPROFILE"] == os.environ["HOME"] == str(home)


def test_paths_computed_at_import_time_point_into_the_temporary_home(tmp_path_factory):
    home = Path.home()
    assert _inside(home, tmp_path_factory.getbasetemp()), home
    for path in (
        credentials.AIM_DIR,
        credentials.SDK_CREDENTIALS_FILE,
        credentials.AGENTS_DIR,
        credentials.LEGACY_CREDENTIALS_FILE,
        grant_client.DEFAULT_SOCKET_PATH,
        grant_client.DEFAULT_TOKEN_PATH,
    ):
        assert _inside(path, home), path


def test_credentials_in_a_folder_above_the_working_directory_are_not_adopted(
    tmp_path, tmp_path_factory, monkeypatch
):
    # Checked first: without the fixture this test stops here, before
    # load_sdk_credentials() can read or write a real home directory.
    assert _inside(credentials.SDK_CREDENTIALS_FILE, tmp_path_factory.getbasetemp())

    outer = tmp_path / "outer"
    (outer / ".aim").mkdir(parents=True)
    (outer / ".aim" / "sdk_credentials.json").write_text(json.dumps({
        "aimUrl": "http://127.0.0.1:9",
        "refreshToken": "FAKE-REFRESH-726",
        "type": "sdk_oauth",
    }))
    project = outer / "project"
    project.mkdir()
    monkeypatch.chdir(project)

    assert credentials.load_sdk_credentials() is None
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_working_directory_is_the_temporary_home_with_no_aim_folder_above_it():
    # A Python child that inherits this directory walks it and its ancestors.
    cwd = Path.cwd().resolve()
    assert cwd == Path.home().resolve(), cwd
    assert [str(p) for p in (cwd, *cwd.parents) if (p / ".aim").exists()] == []


def _from_a_file_location(expr: ast.AST, names: set) -> bool:
    """The expression reads __file__, a module's __file__, or one of names."""
    return any(
        (isinstance(n, ast.Name) and (n.id == "__file__" or n.id in names))
        or (isinstance(n, ast.Attribute) and n.attr == "__file__")
        for n in ast.walk(expr)
    )


def _checkout_directory_names(tree: ast.Module) -> set:
    """Names assigned anywhere in the module from a file's location, directly
    or through another such name: directories of the checkout."""
    assignments = [
        (node.targets if isinstance(node, ast.Assign) else [node.target], node.value)
        for node in ast.walk(tree)
        if isinstance(node, (ast.Assign, ast.AnnAssign)) and node.value is not None
    ]
    names: set = set()
    while True:
        found = {
            target.id
            for targets, value in assignments if _from_a_file_location(value, names)
            for target in targets if isinstance(target, ast.Name)
        }
        if found <= names:
            return names
        names |= found


def _is_a_python_command(expr) -> bool:
    """A list or tuple whose first element is sys.executable."""
    if not isinstance(expr, (ast.List, ast.Tuple)) or not expr.elts:
        return False
    first = expr.elts[0]
    return (
        isinstance(first, ast.Attribute) and first.attr == "executable"
        and isinstance(first.value, ast.Name) and first.value.id == "sys"
    )


def _python_command_names(tree: ast.Module) -> set:
    """Names assigned a Python command anywhere in the module."""
    return {
        target.id
        for node in ast.walk(tree)
        if isinstance(node, ast.Assign) and _is_a_python_command(node.value)
        for target in node.targets if isinstance(target, ast.Name)
    }


def test_no_python_child_process_runs_in_a_directory_of_the_checkout():
    """A Python child started in a directory of the checkout walks that
    directory's ancestors for a `.aim/`, so under a checkout inside a home
    directory that holds SDK credentials it adopts them. This fails on a call
    whose command starts with sys.executable, written in the call or assigned
    to a name, and whose cwd is computed from a file's location."""
    children, offenders = [], []
    for path in sorted(TESTS_DIR.glob("*.py")):
        tree = ast.parse(path.read_text(encoding="utf-8"))
        checkout_dirs = _checkout_directory_names(tree)
        commands = _python_command_names(tree)
        for call in ast.walk(tree):
            if not isinstance(call, ast.Call) or not call.args:
                continue
            command = call.args[0]
            if not (
                _is_a_python_command(command)
                or (isinstance(command, ast.Name) and command.id in commands)
            ):
                continue
            children.append(f"{path.name}:{call.lineno}")
            for keyword in call.keywords:
                if keyword.arg == "cwd" and _from_a_file_location(keyword.value, checkout_dirs):
                    offenders.append(f"{path.name}:{call.lineno}")
    # The scan still recognises the suite's child processes.
    assert children
    assert offenders == []


def test_import_cost_tests_in_a_checkout_under_a_signed_in_home_copy_none_of_its_credentials(tmp_path):
    """The checkout is a copy of this SDK placed under a home whose `.aim/`
    holds SDK credentials, as `aim-sdk login` leaves them. No file the run
    writes under its pytest base temp, where its temporary homes are, holds
    the refresh token."""
    outer_home = tmp_path / "outer-home"
    (outer_home / ".aim").mkdir(parents=True)
    (outer_home / ".aim" / "sdk_credentials.json").write_text(json.dumps({
        "aimUrl": "http://127.0.0.1:9",
        "refreshToken": "FAKE-REFRESH-PLACEHOLDER",
        "type": "sdk_oauth",
    }))
    sdk_copy = outer_home / "work" / "sdk" / "python"
    no_cache = shutil.ignore_patterns("__pycache__")
    shutil.copytree(SDK_ROOT / "aim_sdk", sdk_copy / "aim_sdk", ignore=no_cache)
    shutil.copytree(TESTS_DIR, sdk_copy / "tests", ignore=no_cache)
    shutil.copy(SDK_ROOT / "pytest.ini", sdk_copy / "pytest.ini")

    basetemp = tmp_path / "basetemp"
    env = {k: v for k, v in os.environ.items() if not k.startswith("AIM_")}
    env.update({"HOME": str(outer_home), "USERPROFILE": str(outer_home)})
    proc = subprocess.run(
        [
            sys.executable, "-m", "pytest", "-p", "no:cacheprovider",
            f"--basetemp={basetemp}", "tests/test_cli_import_cost.py",
        ],
        cwd=sdk_copy, env=env, capture_output=True, text=True, timeout=300,
    )
    assert proc.returncode == 0, proc.stdout[-3000:] + proc.stderr[-1000:]
    assert "test_status_signed_out_loads_no_network_or_crypto_package PASSED" in proc.stdout
    holding = [
        p.relative_to(basetemp).as_posix()
        for p in sorted(basetemp.rglob("*"))
        if p.is_file() and b"FAKE-REFRESH-PLACEHOLDER" in p.read_bytes()
    ]
    assert holding == []

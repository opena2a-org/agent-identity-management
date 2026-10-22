"""
`aim-sdk --version`, `aim-sdk -V` and `aim-sdk version` say whether the copy
that runs is the published release.

The published aim-sdk 2.0.3 and a build of this repository both printed
`aim-sdk 2.0.3`, while the source build has commands and flags 2.0.3 does not
(`init`, `demo`, `help`, `status --json`, `version --json`,
`login --no-browser`, `login --allow-http`). A source build now prints
`aim-sdk <v>+source (built from source; not the published <v>)`, the wording
of `aim-arp --version`, and the published release prints `aim-sdk <v>`.

`cli_version()` decides from the copy itself. The copy is the published
release when no VERSION file sits beside the imported package (a source tree
and the sdist carry one; a wheel installs the package without it) and the
aim-sdk distribution's metadata exists without a `direct_url.json`. pip writes
that file (PEP 610) for an install from a local directory, a VCS URL or an
archive URL, and never for an install from a package index.

`version --json`, `aim_sdk.__version__` and the User-Agents stay the bare
version.

Everything here runs offline.
"""

import importlib.metadata
import json
import os
import sys
from pathlib import Path
from unittest import mock

import pytest

import aim_sdk
import aim_sdk.cli as cli

SDK_ROOT = Path(__file__).resolve().parents[1]
VERSION = (SDK_ROOT / "VERSION").read_text(encoding="utf-8").strip()
SOURCE_LINE = f"aim-sdk {VERSION}+source (built from source; not the published {VERSION})"
PUBLISHED_LINE = f"aim-sdk {VERSION}"

VERSION_LINES = [
    pytest.param(["--version"], id="--version"),
    pytest.param(["-V"], id="-V"),
    pytest.param(["version"], id="version"),
]

# What pip writes for `pip install ./sdk/python` (PEP 610).
DIRECTORY_INSTALL = '{"url": "file:///x", "dir_info": {}}'


def _run(argv, capsys):
    """`aim-sdk <argv>` through cli.main(): (exit status, stdout)."""
    with mock.patch.object(sys, "argv", ["aim-sdk", *argv]):
        try:
            code = cli.main()
        except SystemExit as exit_:  # --version and -V end in parser.exit()
            code = exit_.code
    out, _ = capsys.readouterr()
    return code, out


class _Distribution:
    """Stands in for the metadata of an installed aim-sdk distribution."""

    def __init__(self, direct_url):
        self.direct_url = direct_url

    def read_text(self, filename):
        return self.direct_url if filename == "direct_url.json" else None


def _installed(monkeypatch, tmp_path, distribution):
    """Run as an installed copy: no VERSION file beside the package, and
    importlib.metadata answering "aim-sdk" with `distribution`, or with
    PackageNotFoundError when it is None."""
    # raising=False: the published case also runs against a cli.py without the
    # probe, where every copy printed the published wording.
    monkeypatch.setattr(cli, "_version_file", lambda: str(tmp_path / "VERSION"), raising=False)

    def lookup(name):
        assert name == "aim-sdk", name
        if distribution is None:
            raise importlib.metadata.PackageNotFoundError(name)
        return distribution

    monkeypatch.setattr(importlib.metadata, "distribution", lookup)


def test_the_suite_runs_from_a_source_tree():
    """The source-tree cells below read the VERSION file beside the imported package."""
    beside = Path(cli.__file__).resolve().parent.parent / "VERSION"
    assert beside.is_file(), beside
    assert os.path.samefile(cli._version_file(), beside)


@pytest.mark.parametrize("argv", VERSION_LINES)
def test_a_source_tree_prints_the_source_build_qualifier(argv, capsys):
    code, out = _run(argv, capsys)

    assert code == 0
    assert out == SOURCE_LINE + "\n"


@pytest.mark.parametrize("argv", VERSION_LINES)
def test_the_published_release_prints_the_bare_version(argv, capsys, monkeypatch, tmp_path):
    _installed(monkeypatch, tmp_path, _Distribution(direct_url=None))

    code, out = _run(argv, capsys)

    assert code == 0
    assert out == PUBLISHED_LINE + "\n"
    assert "+source" not in out


@pytest.mark.parametrize("argv", VERSION_LINES)
def test_a_copy_installed_from_a_directory_prints_the_source_build_qualifier(
    argv, capsys, monkeypatch, tmp_path
):
    _installed(monkeypatch, tmp_path, _Distribution(direct_url=DIRECTORY_INSTALL))

    code, out = _run(argv, capsys)

    assert code == 0
    assert out == SOURCE_LINE + "\n"


@pytest.mark.parametrize("argv", VERSION_LINES)
def test_a_copy_without_distribution_metadata_prints_the_source_build_qualifier(
    argv, capsys, monkeypatch, tmp_path
):
    _installed(monkeypatch, tmp_path, None)

    code, out = _run(argv, capsys)

    assert code == 0
    assert out == SOURCE_LINE + "\n"


@pytest.mark.parametrize("argv", [["--version"], ["-V"]], ids=["--version", "-V"])
def test_the_flag_prints_the_line_unwrapped_on_a_narrow_terminal(argv, capsys, monkeypatch):
    """argparse's version action wraps its text at the terminal width; the flag
    prints the same single line as `aim-sdk version`."""
    monkeypatch.setenv("COLUMNS", "40")

    code, out = _run(argv, capsys)

    assert code == 0
    assert out == SOURCE_LINE + "\n"


def test_version_json_and_the_package_version_stay_the_bare_version(capsys):
    code, out = _run(["version", "--json"], capsys)

    assert code == 0
    assert out == json.dumps({"version": VERSION}) + "\n"
    assert json.loads(out) == {"version": aim_sdk.__version__}
    assert aim_sdk.__version__ == VERSION
    assert cli.__version__ == aim_sdk.__version__

#!/usr/bin/env python3
"""Cells for scripts/lint-action-pins.py. No network.

The decision cells inject a reachability oracle, so every branch of the rule
runs without a host. The git cells exercise the oracle the check ships against
repositories this file creates in a temporary directory and fetches over
file://, including a commit no branch reaches that the server still serves by
sha, as the host serves a fork's commits under the parent repository's name.

    python3 -m unittest scripts/test_lint_action_pins.py

Needs PyYAML (scripts/requirements-ci-lint.txt) and git.
"""
from __future__ import annotations

import contextlib
import hashlib
import importlib.util
import io
import os
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path
from unittest import mock

HERE = Path(__file__).resolve().parent
_SPEC = importlib.util.spec_from_file_location("lint_action_pins", HERE / "lint-action-pins.py")
lap = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = lap
_SPEC.loader.exec_module(lap)


def fake_sha(label: str) -> str:
    """A 40-hex commit id derived from a label, so no id is written out literally."""
    return hashlib.sha1(label.encode()).hexdigest()


HEAD = fake_sha("fetched head")
ON_MAIN = fake_sha("on main")
ON_V3 = fake_sha("on releases/v3")
ON_V4 = fake_sha("on releases/v4")
FORK = fake_sha("fork only")


class FakeOracle:
    """Branches and the commits each one reaches; a repository missing here refuses."""

    def __init__(self, defaults: dict[str, str], branches: dict[tuple[str, str], set[str]]) -> None:
        self.defaults = defaults
        self.branches = branches
        self.fetched: dict[tuple[str, str], str] = {}
        self.asked: list[tuple[str, str, str]] = []

    def default_branch(self, repo: str) -> str:
        if repo not in self.defaults:
            raise lap.Refusal(f"git ls-remote --symref https://github.com/{repo}.git HEAD failed (exit 128)")
        return self.defaults[repo]

    def branch_head(self, repo: str, branch: str) -> str | None:
        return HEAD if (repo, branch) in self.branches else None

    def reachable(self, repo: str, sha: str, branch: str) -> bool:
        self.asked.append((repo, sha, branch))
        self.fetched[(repo, branch)] = HEAD
        return sha in self.branches[(repo, branch)]


def checkout_oracle() -> FakeOracle:
    return FakeOracle(
        {"actions/checkout": "main", "acme/tool": "main"},
        {
            ("actions/checkout", "main"): {ON_MAIN},
            ("actions/checkout", "releases/v3"): {ON_V3},
            ("actions/checkout", "releases/v4"): {ON_V4},
            ("acme/tool", "main"): {ON_MAIN},
        },
    )


def workflow(*steps: str, extra_jobs: str = "") -> str:
    body = "\n".join(f"      - {s}" for s in steps)
    return (
        "name: fixture\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n"
        + body + "\n" + extra_jobs
    )


class Case(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)
        (self.root / ".github" / "workflows").mkdir(parents=True)

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def write(self, rel: str, text: str) -> Path:
        path = self.root / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def lint(self, argv: list[str] | None = None, oracle=None, allowlist: dict[str, str] | None = None):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = lap.main(
                argv or [],
                root=self.root,
                oracle=oracle if oracle is not None else checkout_oracle(),
                allowlist={} if allowlist is None else allowlist,
            )
        return rc, out.getvalue(), err.getvalue()


class StaticRules(Case):
    def test_a_pin_reachable_from_the_default_branch_passes(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_MAIN} # v4"))
        rc, out, err = self.lint()
        self.assertEqual(rc, 0, out + err)

    def test_a_tag_is_a_mutable_ref(self) -> None:
        self.write(".github/workflows/ci.yml", workflow("uses: actions/checkout@v4"))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertIn(".github/workflows/ci.yml:7: [mutable-ref] uses: actions/checkout@v4", out)

    def test_a_branch_a_short_sha_and_no_ref_are_mutable_refs(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(
            "uses: acme/tool@main", f"uses: acme/tool@{ON_MAIN[:7]}", "uses: acme/tool",
        ))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertEqual(out.count("[mutable-ref]"), 3, out)

    def test_a_docker_image_needs_a_digest(self) -> None:
        digest = hashlib.sha256(b"image").hexdigest()
        self.write(".github/workflows/ci.yml", workflow(
            "uses: docker://alpine:3.21", f"uses: docker://alpine:3.21@sha256:{digest}",
        ))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertEqual(out.count("[docker-digest]"), 1, out)
        self.assertIn(":7: [docker-digest]", out)

    def test_a_flow_style_step_is_read(self) -> None:
        self.write(".github/workflows/ci.yml", workflow("{uses: actions/checkout@v4}"))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertIn("[mutable-ref] uses: actions/checkout@v4", out)

    def test_run_text_and_comments_are_not_uses_keys(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(
            "# uses: actions/checkout@v4",
            "run: |\n          cat > x.yml <<'EOF'\n          uses: actions/checkout@v4\n          EOF",
        ))
        rc, out, err = self.lint()
        self.assertEqual(rc, 0, out + err)

    def test_a_quoted_value_keeps_its_version_comment(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f'uses: "actions/checkout@{ON_V4}" # v4.4.0'))
        rc, out, err = self.lint(allowlist={"actions/checkout@4": "releases/v4"})
        self.assertEqual(rc, 0, out + err)

    def test_a_value_that_is_not_a_string_is_unreadable(self) -> None:
        self.write(".github/workflows/ci.yml", workflow("uses: [actions/checkout, v4]"))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertIn("[unreadable]", out)


class Reachability(Case):
    def test_a_fork_only_commit_is_unreachable(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{FORK} # v4"))
        rc, out, _ = self.lint(allowlist={"actions/checkout@4": "releases/v4"})
        self.assertEqual(rc, 1)
        self.assertIn(f".github/workflows/ci.yml:7: [unreachable] uses: actions/checkout@{FORK} # v4", out)
        self.assertIn("main, the default branch of github.com/actions/checkout", out)
        self.assertIn("releases/v4", out)

    def test_a_commit_on_a_branch_no_entry_names_is_unreachable(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_V3} # v3.7.0"))
        rc, out, _ = self.lint(allowlist={"actions/checkout@4": "releases/v4"})
        self.assertEqual(rc, 1)
        self.assertIn("[unreachable]", out)
        self.assertIn("ALLOWLIST names no release branch for actions/checkout@3", out)

    def test_an_allowlisted_release_branch_admits_its_own_major(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_V4} # v4"))
        oracle = checkout_oracle()
        rc, out, err = self.lint(oracle=oracle, allowlist={"actions/checkout@4": "releases/v4"})
        self.assertEqual(rc, 0, out + err)
        self.assertEqual(
            oracle.asked,
            [("actions/checkout", ON_V4, "main"), ("actions/checkout", ON_V4, "releases/v4")],
        )

    def test_an_entry_does_not_admit_another_major(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(
            f"uses: actions/checkout@{ON_V4} # v5", f"uses: actions/checkout@{ON_MAIN} # v4",
        ))
        rc, out, _ = self.lint(allowlist={"actions/checkout@4": "releases/v4"})
        self.assertEqual(rc, 1)
        self.assertIn(":7: [unreachable]", out)
        self.assertIn("ALLOWLIST names no release branch for actions/checkout@5", out)

    def test_a_pin_without_a_version_comment_is_judged_on_the_default_branch_alone(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(
            f"uses: actions/checkout@{ON_V4}", f"uses: actions/checkout@{ON_MAIN} # v4",
        ))
        rc, out, _ = self.lint(allowlist={"actions/checkout@4": "releases/v4"})
        self.assertEqual(rc, 1)
        self.assertIn(":7: [unreachable]", out)
        self.assertIn("no version in its trailing comment", out)

    def test_an_allowlisted_branch_missing_upstream_is_unreachable(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: acme/tool@{ON_V3} # v3"))
        rc, out, _ = self.lint(allowlist={"acme/tool@3": "releases/v3"})
        self.assertEqual(rc, 1)
        self.assertIn("[unreachable]", out)
        self.assertIn("does not exist upstream", out)

    def test_each_pinned_commit_is_decided_once(self) -> None:
        self.write(".github/workflows/a.yml", workflow(*[f"uses: actions/checkout@{ON_MAIN} # v4"] * 3))
        self.write(".github/workflows/b.yml", workflow(f"uses: actions/checkout@{ON_MAIN} # v4"))
        oracle = checkout_oracle()
        rc, out, err = self.lint(oracle=oracle)
        self.assertEqual(rc, 0, out + err)
        self.assertEqual(oracle.asked, [("actions/checkout", ON_MAIN, "main")])

    def test_every_line_with_an_unreachable_pin_is_named(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(*[f"uses: actions/checkout@{FORK} # v4"] * 2))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertIn(":7: [unreachable]", out)
        self.assertIn(":8: [unreachable]", out)

    def test_the_owner_and_repo_are_matched_without_case(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: Actions/Checkout@{ON_V4} # v4"))
        rc, out, err = self.lint(allowlist={"actions/checkout@4": "releases/v4"})
        self.assertEqual(rc, 0, out + err)


class Allowlist(Case):
    def test_an_entry_naming_another_branch_fails_and_is_not_honoured(self) -> None:
        for branch in ("release/v4", "releases/v5", "releases/v4-hotfix", "maint/v4"):
            with self.subTest(branch=branch):
                self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_V4} # v4"))
                oracle = checkout_oracle()
                oracle.branches[("actions/checkout", branch)] = {ON_V4}
                rc, out, _ = self.lint(oracle=oracle, allowlist={"actions/checkout@4": branch})
                self.assertEqual(rc, 1)
                self.assertIn("[allowlist-branch]", out)
                self.assertIn("[unreachable]", out)
                self.assertNotIn(("actions/checkout", ON_V4, branch), oracle.asked)

    def test_an_entry_no_workflow_uses_fails(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_MAIN} # v4"))
        rc, out, _ = self.lint(allowlist={"actions/checkout@4": "releases/v4", "acme/tool@2": "releases/v2"})
        self.assertEqual(rc, 1)
        self.assertIn("[allowlist-unused] acme/tool@2 -> releases/v2", out)
        self.assertNotIn("actions/checkout@4 -> releases/v4", out)

    def test_use_is_counted_over_the_repository_workflows_when_a_path_is_given(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_V4} # v4"))
        other = self.write(".github/workflows/other.yml", workflow(f"uses: acme/tool@{ON_MAIN} # v1"))
        rc, out, err = self.lint([str(other)], allowlist={"actions/checkout@4": "releases/v4"})
        self.assertEqual(rc, 0, out + err)

    def test_a_malformed_key_fails(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_MAIN} # v4"))
        rc, out, _ = self.lint(allowlist={"actions/checkout": "releases/v4"})
        self.assertEqual(rc, 1)
        self.assertIn("[allowlist-entry]", out)

    def test_the_shipped_entries_name_their_own_release_branch(self) -> None:
        self.assertTrue(lap.ALLOWLIST)
        for key, branch in lap.ALLOWLIST.items():
            with self.subTest(entry=key):
                repo, _, major = key.partition("@")
                self.assertRegex(repo, r"^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$")
                self.assertEqual(branch, f"releases/v{int(major)}")


class LocalAndReusable(Case):
    def test_a_local_composite_action_is_read_by_the_same_pass(self) -> None:
        self.write(".github/workflows/ci.yml", workflow("uses: ./.github/actions/setup"))
        self.write(".github/actions/setup/action.yml", textwrap.dedent("""\
            name: setup
            runs:
              using: composite
              steps:
                - uses: actions/checkout@v4
            """))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertIn(".github/actions/setup/action.yml:5: [mutable-ref] uses: actions/checkout@v4", out)

    def test_a_local_docker_action_image_needs_a_digest(self) -> None:
        self.write(".github/workflows/ci.yml", workflow("uses: ./tools/container"))
        self.write("tools/container/action.yaml", "name: c\nruns:\n  using: docker\n  image: docker://alpine:3.21\n")
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertIn("tools/container/action.yaml:4: [docker-digest]", out)

    def test_a_local_path_that_names_nothing_is_a_finding(self) -> None:
        self.write(".github/workflows/ci.yml", workflow("uses: ./.github/actions/missing", "uses: ./../outside"))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertEqual(out.count("[local-unreadable]"), 2, out)

    def test_a_local_reusable_workflow_is_followed_from_a_given_path(self) -> None:
        caller = self.write(".github/workflows/caller.yml", (
            "on: push\njobs:\n  call:\n    uses: ./.github/workflows/called.yml\n"
        ))
        self.write(".github/workflows/called.yml", workflow("uses: actions/checkout@v4"))
        rc, out, _ = self.lint([str(caller)])
        self.assertEqual(rc, 1)
        self.assertIn(".github/workflows/called.yml:7: [mutable-ref]", out)

    def test_a_remote_reusable_workflow_is_pinned_and_reachable(self) -> None:
        self.write(".github/workflows/ci.yml", (
            "on: push\njobs:\n"
            "  a:\n    uses: acme/tool/.github/workflows/build.yml@v1\n"
            f"  b:\n    uses: acme/tool/.github/workflows/build.yml@{FORK} # v1\n"
            f"  c:\n    uses: acme/tool/.github/workflows/build.yml@{ON_MAIN} # v1\n"
        ))
        rc, out, _ = self.lint()
        self.assertEqual(rc, 1)
        self.assertIn(":4: [mutable-ref]", out)
        self.assertIn(":6: [unreachable]", out)
        self.assertNotIn(":8:", out)


class NamedRefusals(Case):
    def test_a_missing_yaml_parser_is_a_named_refusal(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_MAIN} # v4"))
        with mock.patch.object(lap, "yaml", None):
            rc, out, err = self.lint()
        self.assertEqual(rc, 2)
        self.assertIn("PyYAML", err)

    def test_a_missing_git_is_a_named_refusal(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(f"uses: actions/checkout@{ON_MAIN} # v4"))
        out, err = io.StringIO(), io.StringIO()
        with mock.patch.object(lap.shutil, "which", return_value=None), \
                contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = lap.main([], root=self.root, allowlist={})
        self.assertEqual(rc, 2)
        self.assertIn("git is not on PATH", err.getvalue())

    def test_a_failed_lookup_is_a_named_refusal_and_never_a_pass(self) -> None:
        self.write(".github/workflows/ci.yml", workflow(
            f"uses: unknown/action@{ON_MAIN} # v1", "uses: actions/checkout@v4",
        ))
        rc, out, err = self.lint()
        self.assertEqual(rc, 2)
        self.assertIn("https://github.com/unknown/action.git", err)
        self.assertIn("[mutable-ref]", out)

    def test_a_workflow_that_does_not_parse_is_a_named_refusal(self) -> None:
        self.write(".github/workflows/ci.yml", "jobs:\n  a:\n    steps:\n      - run: x --only-binary :all: y\n")
        rc, _, err = self.lint()
        self.assertEqual(rc, 2)
        self.assertIn("cannot parse .github/workflows/ci.yml", err)

    def test_nothing_to_check_is_a_named_refusal(self) -> None:
        rc, _, err = self.lint()
        self.assertEqual(rc, 2)
        self.assertIn("no workflow files", err)


# ---------------------------------------------------------------------------
# The oracle the check ships, against repositories created here, over file://
# ---------------------------------------------------------------------------

GIT_ENV = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
GIT_ENV.update(
    GIT_CONFIG_NOSYSTEM="1",
    GIT_CONFIG_GLOBAL=os.devnull,
    GIT_TERMINAL_PROMPT="0",
    GIT_AUTHOR_NAME="Fixture",
    GIT_AUTHOR_EMAIL="fixture@example.invalid",
    GIT_COMMITTER_NAME="Fixture",
    GIT_COMMITTER_EMAIL="fixture@example.invalid",
)


def git(*args: str, cwd: Path | None = None, env: dict[str, str] | None = None) -> subprocess.CompletedProcess:
    return subprocess.run(["git", *args], cwd=cwd, env=env or GIT_ENV, capture_output=True, text=True, check=False)


def must(*args: str, cwd: Path | None = None) -> str:
    done = git(*args, cwd=cwd)
    if done.returncode != 0:
        raise AssertionError(f"git {' '.join(args)} failed: {done.stderr.strip()}")
    return done.stdout.strip()


class GitOracle(unittest.TestCase):
    """acme/tool: main (three commits); releases/v3 forked from the first commit with one
    of its own; and a commit no branch reaches that the server still serves by sha."""

    @classmethod
    def setUpClass(cls) -> None:
        cls._tmp = tempfile.TemporaryDirectory()
        base = Path(cls._tmp.name)
        work, cls.bare = base / "work", base / "hosts" / "acme" / "tool.git"
        must("init", "-q", "-b", "main", str(work))

        def commit(message: str) -> str:
            (work / "f.txt").write_text(message, encoding="utf-8")
            must("add", "f.txt", cwd=work)
            must("commit", "-q", "-m", message, cwd=work)
            return must("rev-parse", "HEAD", cwd=work)

        cls.first = commit("one")
        commit("two")
        cls.main_head = commit("three")
        must("checkout", "-q", "-b", "releases/v3", cls.first, cwd=work)
        cls.on_v3 = commit("v3 maintenance")
        must("checkout", "-q", "-b", "fork", cls.main_head, cwd=work)
        cls.fork = commit("on no upstream branch")
        cls.bare.parent.mkdir(parents=True)
        must("init", "-q", "--bare", str(cls.bare))
        must("push", "-q", str(cls.bare), "main", "releases/v3", "fork", cwd=work)
        must("symbolic-ref", "HEAD", "refs/heads/main", cwd=cls.bare)
        must("update-ref", "-d", "refs/heads/fork", cwd=cls.bare)
        must("config", "uploadpack.allowAnySHA1InWant", "true", cwd=cls.bare)
        must("config", "uploadpack.allowFilter", "true", cwd=cls.bare)
        cls.base_url = (base / "hosts").as_uri()
        cls.url = f"{cls.base_url}/acme/tool.git"

    @classmethod
    def tearDownClass(cls) -> None:
        cls._tmp.cleanup()

    def setUp(self) -> None:
        self._scratch = tempfile.TemporaryDirectory()
        self.oracle = lap.GitOracle(Path(self._scratch.name), base_url=self.base_url)

    def tearDown(self) -> None:
        self._scratch.cleanup()

    def test_control_the_server_serves_the_fork_commit_by_sha(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            must("init", "-q", "--bare", tmp)
            done = git("fetch", "-q", self.url, self.fork, cwd=Path(tmp))
            self.assertEqual(done.returncode, 0, done.stderr)
            self.assertEqual(git("cat-file", "-e", f"{self.fork}^{{commit}}", cwd=Path(tmp)).returncode, 0)

    def test_control_a_filtered_clone_fetches_a_missing_commit_by_sha_unless_told_not_to(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            clone = Path(tmp)
            must("init", "-q", "--bare", str(clone))
            must("fetch", "-q", "--filter=tree:0", self.url, "+refs/heads/main:refs/upstream/0", cwd=clone)
            off = dict(GIT_ENV, GIT_NO_LAZY_FETCH="1")
            self.assertNotEqual(git("cat-file", "-e", f"{self.fork}^{{commit}}", cwd=clone, env=off).returncode, 0)
            self.assertEqual(git("cat-file", "-e", f"{self.fork}^{{commit}}", cwd=clone).returncode, 0)

    def test_the_default_branch_comes_from_the_symref(self) -> None:
        self.assertEqual(self.oracle.default_branch("acme/tool"), "main")

    def test_a_commit_on_the_default_branch_is_reachable(self) -> None:
        self.assertTrue(self.oracle.reachable("acme/tool", self.first, "main"))
        self.assertTrue(self.oracle.reachable("acme/tool", self.main_head, "main"))
        self.assertEqual(self.oracle.fetched[("acme/tool", "main")], self.main_head)

    def test_a_release_branch_commit_is_reachable_from_that_branch_only(self) -> None:
        self.assertFalse(self.oracle.reachable("acme/tool", self.on_v3, "main"))
        self.assertTrue(self.oracle.reachable("acme/tool", self.on_v3, "releases/v3"))

    def test_a_commit_served_by_sha_but_on_no_branch_is_unreachable_and_never_fetched(self) -> None:
        self.assertFalse(self.oracle.reachable("acme/tool", self.fork, "main"))
        self.assertFalse(self.oracle.reachable("acme/tool", self.fork, "releases/v3"))
        clone = self.oracle.clone("acme/tool")
        off = dict(GIT_ENV, GIT_NO_LAZY_FETCH="1")
        self.assertNotEqual(git("cat-file", "-e", f"{self.fork}^{{commit}}", cwd=clone, env=off).returncode, 0)
        self.assertEqual(git("config", "--get-regexp", "promisor|partialclone", cwd=clone).stdout, "")

    def test_a_branch_head_is_read_without_a_fetch_and_a_missing_branch_is_none(self) -> None:
        self.assertEqual(self.oracle.branch_head("acme/tool", "releases/v3"), self.on_v3)
        self.assertIsNone(self.oracle.branch_head("acme/tool", "releases/v9"))
        self.assertEqual(self.oracle.fetched, {})

    def test_a_repository_that_cannot_be_read_is_a_refusal(self) -> None:
        with self.assertRaises(lap.Refusal):
            self.oracle.default_branch("acme/missing")
        with self.assertRaises(lap.Refusal):
            self.oracle.reachable("acme/tool", self.first, "no-such-branch")

    def test_a_failed_lookup_is_not_retried_for_each_pin(self) -> None:
        calls = []
        real = self.oracle.run

        def counted(*args, **kwargs):
            calls.append(args[0])
            return real(*args, **kwargs)

        with mock.patch.object(self.oracle, "run", side_effect=counted):
            for _ in range(3):
                with self.assertRaises(lap.Refusal):
                    self.oracle.default_branch("acme/missing")
        self.assertEqual(calls, ["ls-remote"])

    def test_main_judges_three_pins_with_the_shipped_oracle(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            flow = root / ".github" / "workflows" / "ci.yml"
            flow.parent.mkdir(parents=True)
            flow.write_text(workflow(
                f"uses: acme/tool@{self.first} # v1",
                f"uses: acme/tool@{self.on_v3} # v3.0.0",
                f"uses: acme/tool@{self.fork} # v1",
            ), encoding="utf-8")
            out, err = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                rc = lap.main([], root=root, oracle=self.oracle, allowlist={"acme/tool@3": "releases/v3"})
        self.assertEqual(rc, 1, out.getvalue() + err.getvalue())
        self.assertEqual(out.getvalue().count("[unreachable]"), 1, out.getvalue())
        self.assertIn(f":9: [unreachable] uses: acme/tool@{self.fork} # v1", out.getvalue())


if __name__ == "__main__":
    unittest.main()

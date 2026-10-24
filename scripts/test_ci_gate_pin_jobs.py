#!/usr/bin/env python3
"""CI Gate fails unless the action-pin, runtime-image-pin and Go toolchain jobs succeed.

Branch protection requires one context, CI Gate. A check that CI Gate does not
need is advisory: it can go red and the pull request still merges. These cells
read `.github/workflows/ci.yml` and assert, for each of the three jobs:

  * it exists, runs its script, and has no `if:`, no `continue-on-error` and no
    `|| true`, so it runs on every pull request and its exit code is its result;
  * CI Gate lists it in `needs`;
  * CI Gate's own script, executed under bash with each `needs.<job>.result`
    substituted, exits 1 when that job's result is failure, cancelled or skipped,
    and exits 0 only when every job it reads succeeded.

    python3 -m unittest scripts/test_ci_gate_pin_jobs.py

Needs PyYAML (scripts/requirements-ci-lint.txt) and bash.
"""
from __future__ import annotations

import re
import subprocess
import unittest
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parent.parent
CI = REPO_ROOT / ".github" / "workflows" / "ci.yml"

# job id -> a command its steps must run
JOBS = {
    "action-pin-lint": "python3 scripts/lint-action-pins.py",
    "runtime-image-pin-lint": "python3 scripts/lint-runtime-image-pins.py",
    "go-toolchain-check": "scripts/check-go-toolchain.sh",
}
RESULT = re.compile(r"\$\{\{\s*needs\.([A-Za-z0-9_-]+)\.result\s*\}\}")
PINNED = re.compile(r"^[^@\s]+@[0-9a-f]{40}$")


def load() -> dict:
    with CI.open(encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def gate_script(workflow: dict) -> str:
    steps = workflow["jobs"]["ci-gate"]["steps"]
    scripts = [s["run"] for s in steps if "run" in s]
    if len(scripts) != 1:
        raise AssertionError(f"ci-gate has {len(scripts)} run steps; expected exactly one")
    return scripts[0]


def run_gate(script: str, results: dict[str, str]) -> subprocess.CompletedProcess:
    """CI Gate's script with each needs.<job>.result replaced, run as Actions runs it."""
    def value(match: re.Match) -> str:
        return results[match.group(1)]

    body = RESULT.sub(value, script)
    return subprocess.run(
        ["bash", "--noprofile", "--norc", "-eo", "pipefail", "-c", body],
        capture_output=True, text=True, check=False,
    )


class CiGateNeedsThePinJobs(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.workflow = load()
        cls.jobs = cls.workflow["jobs"]
        cls.script = gate_script(cls.workflow)
        cls.read = sorted(set(RESULT.findall(cls.script)))

    def test_each_job_exists_and_runs_its_script(self) -> None:
        for job_id, command in JOBS.items():
            with self.subTest(job=job_id):
                self.assertTrue(job_id in self.jobs, f"ci.yml has no job {job_id}")
                runs = "\n".join(s.get("run", "") for s in self.jobs[job_id].get("steps", []))
                self.assertTrue(command in runs, f"{job_id} does not run {command}")

    def test_each_job_is_unconditional_and_keeps_its_exit_code(self) -> None:
        for job_id in JOBS:
            with self.subTest(job=job_id):
                job = self.jobs[job_id]
                self.assertNotIn("if", job, f"{job_id} has an if:, so it can be skipped")
                self.assertNotIn("continue-on-error", job, f"{job_id} has continue-on-error")
                for step in job.get("steps", []):
                    self.assertNotIn("if", step, f"{job_id} step {step.get('name') or step.get('uses')} has an if:")
                    self.assertNotIn("continue-on-error", step, f"{job_id} step has continue-on-error")
                    self.assertNotRegex(step.get("run", ""), r"\|\|\s*true", f"{job_id} step swallows its exit code")

    def test_each_job_pins_its_actions_by_sha(self) -> None:
        for job_id in JOBS:
            for step in self.jobs[job_id].get("steps", []):
                if "uses" in step:
                    with self.subTest(job=job_id, uses=step["uses"]):
                        self.assertRegex(step["uses"], PINNED)

    def test_ci_gate_needs_each_job_and_runs_always(self) -> None:
        gate = self.jobs["ci-gate"]
        self.assertEqual(gate.get("if"), "always()", "ci-gate must keep if: always()")
        for job_id in JOBS:
            with self.subTest(job=job_id):
                self.assertTrue(job_id in gate["needs"], f"ci-gate does not need {job_id}")
                self.assertTrue(job_id in self.read, f"ci-gate's script never reads needs.{job_id}.result")

    def test_gate_passes_when_every_job_succeeded(self) -> None:
        done = run_gate(self.script, {job: "success" for job in self.read})
        self.assertEqual(done.returncode, 0, done.stdout + done.stderr)

    def test_gate_fails_when_a_pin_or_toolchain_job_did_not_succeed(self) -> None:
        for job_id in JOBS:
            for outcome in ("failure", "cancelled", "skipped"):
                with self.subTest(job=job_id, result=outcome):
                    results = {job: "success" for job in self.read}
                    results[job_id] = outcome
                    done = run_gate(self.script, results)
                    self.assertEqual(done.returncode, 1, f"gate exited {done.returncode} with {job_id}={outcome}")
                    self.assertTrue(f"::error::{job_id}" in done.stdout, f"gate output names no error for {job_id}")


if __name__ == "__main__":
    unittest.main()

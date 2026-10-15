"""
docs/demo/render.sh keeps a run's secrets in a directory it created itself.

The script writes the run's database, Redis, JWT, key-vault and administrator
secrets to a directory under TMPDIR and deletes that directory when the run
ends. A name another account can predict, and so create first, would hand that
account the directory the secrets are written into, and the teardown would
delete a directory the script did not create.

The run here stops right after the secrets are written: the copied script sits
outside a git checkout, so its first precondition fails, and a stand-in docker
answers every call without doing anything.
"""

import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[3]
DEMO = REPO_ROOT / "docs" / "demo"


@pytest.mark.skipif(
    sys.platform == "win32" or shutil.which("bash") is None or shutil.which("openssl") is None,
    reason="render.sh runs under bash and generates its secrets with openssl",
)
def test_render_writes_secrets_only_into_a_directory_it_created(tmp_path):
    demo = tmp_path / "tree" / "docs" / "demo"
    demo.mkdir(parents=True)
    shutil.copy2(DEMO / "render.sh", demo / "render.sh")
    shutil.copytree(DEMO / "stack", demo / "stack")

    docker = tmp_path / "bin" / "docker"
    docker.parent.mkdir()
    docker.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
    docker.chmod(0o755)

    run_root = tmp_path / "t"
    run_root.mkdir()
    # Every name the old scheme, aimdemo-<UTC second>, could pick while this
    # run starts, created ahead of it as another account could.
    now = time.time()
    planted = set()
    for offset in range(-2, 30):
        stamp = time.strftime("%Y%m%d-%H%M%S", time.gmtime(now + offset))
        directory = run_root / f"aimdemo-{stamp}"
        directory.mkdir()
        planted.add(directory)

    proc = subprocess.run(
        ["bash", str(demo / "render.sh"), "no-such-scene", "--rehearsal"],
        env={
            "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
            "HOME": str(tmp_path / "home"),
            "TMPDIR": str(run_root),
            "DOCKER": str(docker),
        },
        capture_output=True,
        text=True,
        timeout=60,
    )

    assert proc.returncode != 0, "the copied script outside a checkout must stop at its first precondition"
    for directory in sorted(planted):
        assert directory.is_dir(), f"render.sh deleted {directory.name}, which it did not create"
        assert [p.name for p in directory.iterdir()] == [], (
            f"render.sh wrote into {directory.name}, which it did not create"
        )
    assert "run secrets deleted" in proc.stderr, proc.stderr
    left = sorted(p.name for p in run_root.iterdir() if p not in planted)
    assert left == [], f"the run directory outlived the run: {left}"

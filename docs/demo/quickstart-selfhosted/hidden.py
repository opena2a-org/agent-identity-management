# Helpers the tape loads into the Python session inside a Hide block. They
# prepare and wait; nothing here grants, verifies or decides an outcome.
import os
import time

OUT = "/work/out"


def cue(name):
    """Write a wall-clock stamp the browser and the editor read."""
    with open(os.path.join(OUT, "cue-" + name), "w") as f:
        f.write(repr(time.time()))


def wait_for(name, timeout=300):
    """Block until the browser has written its cue, then print a marker the
    tape's Wait matches (the marker is assembled so the typed line does not match)."""
    deadline = time.time() + timeout
    while not os.path.exists(os.path.join(OUT, "cue-" + name)):
        if time.time() > deadline:
            raise SystemExit("timed out waiting for cue-" + name)
        time.sleep(0.5)
    print("cue" + "-" + name + "-seen")


def prepare_db():
    """The README's example calls `db.query` and `db.execute` on an object it
    does not define; this stand-in makes the lines runnable. It is preparation
    for the recording, and a gap in the README to fix (a fixed README makes
    this function unnecessary)."""
    global db

    class _DB:
        def query(self, sql, *args):
            return {"id": args[0], "name": "Ada Lovelace", "plan": "team"}

        def execute(self, sql, *args):
            return 1

    db = _DB()


def clear_screen():
    print("\033[2J\033[H", end="", flush=True)

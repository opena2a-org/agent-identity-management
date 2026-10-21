#!/usr/bin/env python3
"""The text version of docs/demo/quickstart.tape: the tape's typed lines run
once more, in order, and what they print, written to stdout.

docs/demo/record-quickstart.sh runs this inside the terminal container after
the recording, in the emptied home directory, with NO_COLOR=1, TERM=dumb and
COLUMNS=110, under the virtual environment that holds the SDK the recording
used, with stdout and stderr redirected to one file.

Every line the tape sends with Enter is replayed. A shown shell line is printed
after `$ ` and run with bash; its exit code follows as `[exit N]`. A shown
`python3` starts an interactive console in this process, and every later line
is printed after the console's prompt (`>>> ` or `... `) and pushed to it, as
the recorded session typed it. A hidden line typed into the console (the
sample record, the wait for the off-screen verification, a screen clear) runs
too, with its output discarded; a hidden shell line (the session log, the
prompt) does not run.

The second argument names a JSON file for the record script's pace check: for
each shown line, its step, the line and what it printed.

    quickstart-capture.py <tape> <steps.json> > capture.txt 2>&1
"""
import code
import contextlib
import io
import json
import os
import re
import subprocess
import sys

TYPE = re.compile(r"^Type(?:@\S+)?\s+([\"'`])(.*)\1$")
ENTER = re.compile(r"^Enter(?:@\S+)?$")
STEP = re.compile(r"^#\s*step:\s*(\S+)")


def submissions(tape):
    """(hidden, text, step) for every Enter the tape sends, in order."""
    hidden, text, step, sent = False, "", None, []
    for raw in open(tape, encoding="utf-8"):
        line = raw.strip()
        step_match = STEP.match(line)
        typed = TYPE.match(line)
        if step_match:
            step = step_match.group(1)
        elif line == "Hide":
            hidden = True
        elif line == "Show":
            hidden = False
        elif typed:
            text += typed.group(2)
        elif ENTER.match(line):
            sent.append((hidden, text, step))
            text = ""
    return sent


def say(text):
    sys.stdout.write(text)
    sys.stdout.flush()


def offset():
    """The byte offset of the output file, shared with every child process."""
    sys.stdout.flush()
    sys.stderr.flush()
    return os.lseek(1, 0, os.SEEK_CUR)


def printed_since(start):
    """What the output file received since `start`, read through a second open file."""
    end = offset()
    with open("/proc/self/fd/1", "rb") as f:
        f.seek(start)
        return f.read(end - start).decode("utf-8", errors="replace")


def main():
    tape, steps_file = sys.argv[1], sys.argv[2]
    console, prompt, shown = None, ">>> ", []
    for hidden, text, step in submissions(tape):
        if console is None and hidden:
            continue
        if console is None:
            say(f"$ {text}\n")
            start = offset()
            if text == "python3":
                say(f"Python {sys.version} on {sys.platform}\n"
                    'Type "help", "copyright", "credits" or "license" for more information.\n')
                console = code.InteractiveConsole(locals={"__name__": "__console__"}, filename="<stdin>")
                shown.append({"step": step, "line": text, "printed": printed_since(start)})
                continue
            rc = subprocess.run(["bash", "-c", text], stdin=subprocess.DEVNULL).returncode
            printed = printed_since(start)
            say(f"[exit {rc}]\n")
            shown.append({"step": step, "line": text, "printed": printed, "exit": rc})
            continue
        if hidden:
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                more = console.push(text)
            prompt = "... " if more else ">>> "
            continue
        say(prompt + text + "\n")
        start = offset()
        more = console.push(text)
        prompt = "... " if more else ">>> "
        shown.append({"step": step, "line": text, "printed": printed_since(start)})
    with open(steps_file, "w", encoding="utf-8") as f:
        json.dump(shown, f, indent=2)


if __name__ == "__main__":
    main()

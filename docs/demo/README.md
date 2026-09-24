# Demo videos

Recorded walkthroughs of the README's Quick start, generated from the
documentation on a throwaway stack of released images with fixture data only.
The instrument is the source of truth for every video: re-running it
re-creates the recording, and the typed lines are extracted from the README at
the recorded commit, so a video cannot drift from the docs it shows.

## One command

```bash
docs/demo/render.sh quickstart-selfhosted --rehearsal
```

Prerequisites: Docker (Desktop or Engine with Compose v2), Node 22 on the
host for the lint and the census, and the VHS base image built once from a
hackmyagent checkout:

```bash
docker build -t hma-vhs:0.33.0 <hackmyagent>/docs/vhs/docker
```

Output: `docs/demo/out/quickstart-selfhosted/<video>.mp4`, its sidecar
`recording.json` (versions, digests, README commit, scene boundaries, edits,
hashes), `captions.srt`, and the census report. The `out/` directory is
ignored by git; videos are never committed.

## How a render works

1. `render.sh` generates the run's secrets (0600, under `$TMPDIR`, deleted at
   the end) and starts `stack/compose.yml` as its own compose project
   `aimdemo-<utc-stamp>`: the quickstart's Postgres, Redis, AIM server and
   dashboard images pinned by digest (`stack/images.env`), a loopback
   forwarder (`lib/gw.mjs`), and the two recorders. Nothing is published on
   the host and no container has a fixed name, so it cannot collide with a
   quickstart stack on the same machine.
2. The terminal recorder (`terminal/`, VHS) plays `<slug>/tape.tape`. Every
   visible typed line is a Quick start line (`lib/lint.mjs` refuses anything
   else); Hide blocks prepare and wait, and never change an outcome.
3. The browser recorder (`browser/`, Playwright) follows the terminal through
   its transcript and cue files, performs every step the viewer would
   perform on the real dashboard, and writes its scene timestamps and a text
   snapshot of every scene.
4. `lib/edit.mjs` joins the threads at the recorded boundaries, holds frames
   where captions need reading time, burns the captions from
   `<slug>/narration.md` into their own band under the 1600x900 content, adds
   the title and end cards (`lib/card.html`), and writes an H.264 MP4 at
   1920x1080, 30 fps, no audio, plus the sidecar.
5. `lib/census.mjs` scans the transcript, the dashboard text snapshots, the
   captions and the cards for the run's secrets, credential shapes, local
   paths, foreign emails and hosts, with a planted-canary control. A
   tab-separated file of extra classes (`class`, regex, canary) can be added
   through `DEMO_FORBIDDEN_FILE`; the lint reads it too.
6. The stack is stopped and removed with its volumes and network.

## Rehearsal and counted renders

A rehearsal (`--rehearsal`) installs this checkout's `sdk/python` instead of
the published package, may type the lines listed in
`<slug>/rehearsal-lines.txt` (each one a gap the README owes), and burns a
REHEARSAL watermark on every frame. It is for measuring the walkthrough and is
never uploaded.

A counted render types `pip install aim-sdk` against PyPI and refuses to run
(exit 2) unless the README shows the self-hosted login, PyPI carries a release
whose login is the device grant, and the pinned images were built at or after
the commit that added it. Exit 3 is a lint or census failure, exit 4 a
stack-safety refusal.

## Narration

`<slug>/narration.md` (format `aim-demo-narration/1`) is the single source of
the on-screen captions and of the voice track a later change adds: one scene
header per scene, one caption per line, plain sentences, at most 84 characters
wrapped at 42, never faster than 15 characters per second. The cards' text
comes from the same file.

## Fixture identities

Organization and administrator as the server creates them, the administrator's
email `admin@example.com`, a password generated per run and only ever typed
masked, the agent `my-first-agent` with `db:read` granted and `db:write`
refused, hosts `localhost:3000` and `localhost:8080`. No token, code, log line
or credential file content appears on screen; the census enforces it.

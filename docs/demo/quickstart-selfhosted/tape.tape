# The terminal thread of the quick-start walkthrough, recorded by VHS inside
# docs/demo/terminal. Every visible `Type` line is a line of the README's
# Quick start (lib/lint.mjs checks). Hide/Show blocks prepare and wait; they
# never change what the viewer would get. Scene boundaries are the screen
# clears inside Hide blocks (uniform frames); the browser is driven by the
# transcript this session writes and by the cue files written here.
Output terminal.mp4
Set Shell "bash"
Set FontSize 26
Set FontFamily "JetBrains Mono"
Set Width 1600
Set Height 900
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set Framerate 30

Hide
Type "script -q -f --log-timing /work/out/transcript.timing /work/out/transcript.txt"
Enter
Sleep 500ms
Type "export PS1='$ ' PYTHON_BASIC_REPL=1; python3 -m venv /home/demo/.venv && export PATH=/home/demo/.venv/bin:$PATH && clear"
Enter
Sleep 4s
Type "date +%s.%N > /work/out/cue-start; clear"
Enter
Sleep 500ms
Show

# s03 · install
Sleep 3s
# @install
Sleep 3s

# s04 · login
Hide
Type "date +%s.%N > /work/out/cue-s04; clear"
Enter
Sleep 500ms
Show
Sleep 1s
Type "aim-sdk login --url http://localhost:8080"
Enter
Sleep 4s
Hide
Wait+Line@300s /^\$\s*$/
Show
# s08 · signed in (the cut above is the browser's approval; its length is measured from the cues)
Sleep 5s

# s09 · register
Hide
Type "date +%s.%N > /work/out/cue-s09; clear"
Enter
Sleep 500ms
Type "python3"
Enter
Sleep 1500ms
Type "exec(open('/work/scene/hidden.py').read())"
Enter
Sleep 500ms
Type "cue('s09')"
Enter
Sleep 300ms
Show
Sleep 1s
Type "from aim_sdk import secure"
Enter
Sleep 2s
Type 'agent = secure("my-first-agent")'
Enter
Sleep 7s
Hide
Type "cue('registered')"
Enter
Type "wait_for('verified')"
Enter
Wait+Line@300s /^>>>\s*$/
Type "prepare_db()"
Enter
Type "cue('s11'); clear_screen()"
Enter
Sleep 500ms
Show

# s11 · the allowed call
Sleep 1s
Type '@agent.perform_action(capability="db:read")'
Enter
Type "def get_customer(customer_id):"
Enter
Type '    return db.query("SELECT * FROM customers WHERE id = ?", customer_id)'
Enter
Enter
Sleep 1s
Type "get_customer(42)"
Enter
Sleep 5s

# s12 · the refused call
Hide
Type "cue('s12'); clear_screen()"
Enter
Sleep 500ms
Show
Sleep 1s
Type '@agent.perform_action(capability="db:write")'
Enter
Type "def delete_customer(customer_id):"
Enter
Type '    return db.execute("DELETE FROM customers WHERE id = ?", customer_id)'
Enter
Enter
Sleep 1s
Type "delete_customer(42)"
Enter
Sleep 7s
Hide
Type "cue('denied'); cue('s12-end')"
Enter
Sleep 500ms
Show
Sleep 1s

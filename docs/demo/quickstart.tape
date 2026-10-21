# The README's Quick start as one terminal recording, on a local AIM stack with fixture data only:
# sign in, register an agent, then one call it holds and one it does not.
# Record: docs/demo/record-quickstart.sh (starts the stack, renders this tape in the terminal
# container, writes the text version docs/demo/quickstart-capture.txt and the sidecar
# docs/demo/quickstart.json, then removes the stack).
# Each step is named by a `# step:` line; its hold after the output is sized from
# docs/demo/quickstart.narration.json and the step's lines on screen (the record script checks it).
# Hidden lines start the session log, define the sample record, wait for the off-screen
# verification and clear the screen at a step boundary; none of them types a command whose output
# is shown. What happens off screen is listed under `edits` in the sidecar.
Output docs/demo/quickstart.gif
Output docs/demo/out/quickstart.mp4
Require aim-sdk
Require python3
Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 14
Set Columns 110
Set Rows 28
Set Theme "Catppuccin Mocha"
Set TypingSpeed 100ms
Set Padding 20
Set Framerate 10
Set CursorBlink false

Hide
Type@5ms "script -q -f /work/out/session.txt"
Enter
Sleep 1s
Type@5ms "export PS1='$ ' && cd ~ && clear"
Enter
Wait+Line@30s /^\$\s*$/
Show
Sleep 500ms

# step: s1 Sign in
Type "aim-sdk login --url http://localhost:8080"
Sleep 700ms
Enter
Wait+Line@120s /^\$\s*$/
Screenshot docs/demo/quickstart-s1.png
Sleep 7500ms

Hide
Type@5ms "clear"
Enter
Wait+Line@30s /^\$\s*$/
Show

# step: s2 Register an agent
Type "python3"
Sleep 700ms
Enter
Wait+Line@60s /^>>>\s*$/
Sleep 800ms
Type "from aim_sdk import secure"
Sleep 700ms
Enter
Wait+Line@60s /^>>>\s*$/
Sleep 300ms
Type 'agent = secure("my-first-agent", capabilities=["db:read"])'
Sleep 700ms
Enter
Wait+Line@60s /^>>>\s*$/
Screenshot docs/demo/quickstart-s2.png
Sleep 5s

Hide
Type@5ms 'customers = {42: {"name": "Example Co", "plan": "team"}}'
Enter
Wait+Line@30s /^>>>\s*$/
Type@5ms '_ = [__import__("time").sleep(0.2) for _i in range(600) if not __import__("os").path.exists("/work/out/cue-verified")]'
Enter
Wait+Line@150s /^>>>\s*$/
Type@5ms 'print(chr(27) + "[2J" + chr(27) + "[H", end="")'
Enter
Wait+Line@30s /^>>>\s*$/
Show

# step: s3 A call the agent holds
Type '@agent.perform_action(capability="db:read")'
Sleep 700ms
Enter
Wait+Line@30s /^\.\.\.\s*$/
Sleep 300ms
Type "def get_customer(customer_id):"
Sleep 700ms
Enter
Wait+Line@30s /^\.\.\.\s*$/
Sleep 300ms
Type "    return customers[customer_id]"
Sleep 700ms
Enter
Wait+Line@30s /^\.\.\.\s*$/
Sleep 700ms
Enter
Wait+Line@30s /^>>>\s*$/
Sleep 300ms
Type "get_customer(42)"
Sleep 700ms
Enter
Wait+Line@60s /^>>>\s*$/
Screenshot docs/demo/quickstart-s3.png
Sleep 6s

Hide
Type@5ms 'print(chr(27) + "[2J" + chr(27) + "[H", end="")'
Enter
Wait+Line@30s /^>>>\s*$/
Show

# step: s4 A call the agent does not hold
Type '@agent.perform_action(capability="db:write")'
Sleep 700ms
Enter
Wait+Line@30s /^\.\.\.\s*$/
Sleep 300ms
Type "def delete_customer(customer_id):"
Sleep 700ms
Enter
Wait+Line@30s /^\.\.\.\s*$/
Sleep 300ms
Type "    del customers[customer_id]"
Sleep 700ms
Enter
Wait+Line@30s /^\.\.\.\s*$/
Sleep 700ms
Enter
Wait+Line@30s /^>>>\s*$/
Sleep 300ms
Type "delete_customer(42)"
Sleep 700ms
Enter
Wait+Line@60s /^>>>\s*$/
Screenshot docs/demo/quickstart-s4.png
Sleep 6500ms

# aim-core -- Local-first agent identity
# Records: Node.js REPL session creating identity and checking capabilities

Output docs/vhs/aim-core.gif

Set Shell "bash"
Set FontSize 14
Set Width 900
Set Height 650
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set Padding 20
Set FontFamily "JetBrains Mono"

Type "# Local agent identity -- no server required"
Enter
Sleep 1.5s

Type "node -e '"
Enter
Sleep 200ms

Type "const { createIdentity } = require(\"@opena2a/aim-core\");"
Enter
Sleep 500ms

Type ""
Enter
Sleep 200ms

Type "// Create an Ed25519 identity for your agent"
Enter
Sleep 500ms

Type "const agent = createIdentity(\"my-assistant\");"
Enter
Sleep 500ms

Type "console.log(\"Agent ID:\", agent.agentId);"
Enter
Sleep 500ms

Type "console.log(\"Public key:\", agent.publicKey.slice(0, 16) + \"...\");"
Enter
Sleep 500ms

Type "console.log(\"Ready for capability enforcement and audit logging.\");"
Enter
Sleep 500ms

Type "'"
Enter
Sleep 3s

# Pause for viewers
Sleep 6s

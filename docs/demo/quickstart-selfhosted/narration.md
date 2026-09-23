# AIM quick start, self-hosted
format: aim-demo-narration/1
video: quickstart-selfhosted
target: 146 s (ceiling 175 s)

## s01 · card · 5 s · The result
AIM quick start, self-hosted
Sign in, register an agent, see one call allowed and one refused.

## s02 · terminal · 6 s · The result
The end result: AIM refuses a call the agent was not granted, before it runs.

## s03 · terminal · 11 s · Install and sign in
A self-hosted AIM stack is running here: dashboard on port 3000, API on 8080.
Install the Python SDK.

## s04 · terminal · 9 s · Install and sign in
Sign the command line in. It prints a one-time code and opens the device page.

## s05 · browser · 9 s · Install and sign in
The page shows the same code. Approve only if it matches your terminal.

## s06 · browser · 10 s · Install and sign in
Sign in to the dashboard as the administrator.

## s07 · browser · 8 s · Install and sign in
Authorize it. The page confirms the command line is signed in.

## s08 · terminal · 8 s · Install and sign in
The command line is signed in. Its session is saved in `~/.aim`.

## s09 · terminal · 12 s · Register and verify the agent
Register an agent: `secure()` generates its key pair and registers it.
A new agent starts pending until an administrator verifies it.

## s10 · browser · 14 s · Register and verify the agent
In the dashboard: Agents, then my-first-agent.
Verify the agent before its first call.

## s11 · terminal · 14 s · An allowed and a refused call
`get_customer` needs `db:read`, which the agent holds.
AIM checks the call, allows it and records it.

## s12 · terminal · 16 s · An allowed and a refused call
`delete_customer` needs `db:write`, which the agent does not hold.
AIM refuses it before the function body runs, and says why.

## s13 · browser · 16 s · Where the refusal is recorded
The agent's activity records the refused `db:write` call.
Under Security, it is listed as a capability violation.

## s14 · card · 8 s · Run it yourself
Run it yourself
github.com/opena2a-org/agent-identity-management#quick-start
pip install aim-sdk

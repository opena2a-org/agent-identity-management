# MCP and execution status request contract fixtures

One file per route. Each holds the request body the Python SDK sends and the
backend reads:

| File | SDK call | Route | Backend request type |
|---|---|---|---|
| `register_mcp.json` | `AIMClient.register_mcp` | `PUT /api/v1/agents/{id}/mcp-servers` | `AddMCPServersRequest` |
| `report_execution_status.json` | `AIMClient.report_execution_status` | `POST /api/v1/sdk-api/verifications/{id}/execution-status` | `UpdateExecutionStatusRequest` |
| `register_single_mcp.json` | `_register_single_mcp`, which `_register_mcp_servers` calls for each MCP server | `POST /api/v1/sdk-api/agents/{id}/mcp-servers` | `CreateMCPServerRequest` |

Two tests read the same files, so renaming a member on either side fails a test:

- `sdk/python/tests/test_sdk_request_contract.py` asserts the SDK sends exactly `body`.
- `apps/backend/internal/interfaces/http/handlers/sdk_request_contract_test.go`
  decodes `body` into the request type, rejecting unknown members. For
  `AddMCPServersRequest` and `UpdateExecutionStatusRequest` it also requires
  every member of the type to be present and posts `body` to the handler. The
  SDK sends none of the `publicKey`, `verificationUrl` and `tagIds` members of
  `CreateMCPServerRequest`, and its handler calls a concrete service, so that
  body is checked at the decode only.

`registeredByAgent` in `register_single_mcp.json` is the agent ID both tests
put in the path.

# AIM roadmap

Last updated: 2026-10-10

This file lists what AIM provides today and what may come next. Only Shipped lines carry versions and dates, each taken from the linked release or image. Requests and bug reports go through GitHub issues.

- **Shipped:** available now in the release or image that the line links.
- **In progress:** a linked pull request is open, or merged but not yet released.
- **Planned:** intended work with no open pull request; scope and order may change.
- **Under consideration:** not scheduled; listed because earlier versions of this file named it.

[Releases](https://github.com/opena2a-org/agent-identity-management/releases) | [Changelog](CHANGELOG.md) | [Issues](https://github.com/opena2a-org/agent-identity-management/issues) | [Security policy](SECURITY.md)

## Shipped

- **SDKs:** The TypeScript SDK installs an `aim-arp` command to read its telemetry status and send log, opt out or back in, and request deletion of data already sent. [@opena2a/aim-sdk 1.3.0 for TypeScript](https://github.com/opena2a-org/agent-identity-management/releases/tag/sdk-ts-v1.3.0), on npm, August 2026
- **SDKs:** The TypeScript SDK's runtime-protection telemetry stays off until the user turns it on. [@opena2a/aim-sdk 1.2.0 for TypeScript](https://github.com/opena2a-org/agent-identity-management/releases/tag/sdk-ts-v1.2.0), on npm, August 2026
- **SDKs:** In the Python SDK, an action that AIM denies raises an error and does not run, in every enforcement mode. [aim-sdk 2.0.0 for Python](https://github.com/opena2a-org/agent-identity-management/releases/tag/sdk-py-v2.0.0), on PyPI, August 2026
- **SDKs:** A Node.js agent registers with AIM and has each action verified through the TypeScript SDK, with middleware for Express and Fastify. [@opena2a/aim-sdk 1.0.0 for TypeScript](https://github.com/opena2a-org/agent-identity-management/releases/tag/sdk-ts-v1.0.0), on npm, July 2026
- **SDKs:** The TypeScript SDK can verify an agent's signed trust credential offline, against issuer keys it has cached, with no call to AIM for each action. [@opena2a/aim-sdk 1.0.0 for TypeScript](https://github.com/opena2a-org/agent-identity-management/releases/tag/sdk-ts-v1.0.0), on npm, July 2026
- **SDKs:** A Python agent registers with AIM in one call, and a decorator has AIM authorize each action and record it in the audit log. [aim-sdk 1.23.0 for Python](https://github.com/opena2a-org/agent-identity-management/releases/tag/sdk-py-v1.23.0), on PyPI, June 2026
- **Registering agents:** An agent registers with the capabilities it declares, and a request for any other capability is recorded as a violation and refused when the organization's security policy blocks it. [AIM platform 1.0.0](https://github.com/opena2a-org/agent-identity-management/releases/tag/platform-v1.0.0), June 2026
- **Dashboard:** The dashboard manages agents, API keys, MCP servers, tags and webhooks, and lists security alerts. [AIM platform 1.0.0](https://github.com/opena2a-org/agent-identity-management/releases/tag/platform-v1.0.0), June 2026
- **API:** AIM sends events to webhook endpoints, signs each delivery with the endpoint's secret, and retries failed deliveries. [AIM platform 1.0.0](https://github.com/opena2a-org/agent-identity-management/releases/tag/platform-v1.0.0), June 2026
- **Deployment:** The API server and the dashboard are published as signed, multi-architecture images on GHCR and Docker Hub, with SBOMs attached to the release. [AIM platform 1.0.0](https://github.com/opena2a-org/agent-identity-management/releases/tag/platform-v1.0.0), June 2026
- **Deployment:** A quickstart script runs the API server, the dashboard, PostgreSQL and Redis on one machine with Docker Compose. [AIM platform 1.0.0](https://github.com/opena2a-org/agent-identity-management/releases/tag/platform-v1.0.0), June 2026
- **Security:** Repeated failed password sign-ins lock the account for a period and raise a security alert. [AIM platform 1.0.0](https://github.com/opena2a-org/agent-identity-management/releases/tag/platform-v1.0.0), June 2026

## In progress

- **Dashboard:** Showing on the agent page why a call was refused and where an administrator grants the missing capability (#602)

## Planned

- **Deployment:** Publishing a platform release with the changes the changelog lists as unreleased, so self-hosters can run them from a versioned image.

## Under consideration

- **SDKs:** Publishing the Java SDK to Maven Central, so a JVM project can add it as a dependency instead of building it from source.
- **SDKs:** Adding agent registration and API key creation to the `aim-sdk` command-line tool, so scripts and pipelines can manage agents.
- **Registering agents:** Integrating with GitHub Copilot, so an organization can audit the actions Copilot takes.
- **Dashboard:** Adding PDF reports of agent activity and security alerts, so auditors can review them outside the dashboard.
- **Dashboard:** Recording video walkthroughs of common dashboard tasks, such as configuring security policies.
- **API:** Publishing an interactive API reference generated from an OpenAPI description of the REST API.
- **API:** Adding a GraphQL endpoint alongside the REST API.
- **Security:** Adding custom roles with fine-grained permissions, so an organization can grant access beyond the built-in roles.
- **Security:** Adding multi-factor sign-in for dashboard accounts, such as one-time codes from an authenticator app.

## How to follow and request changes

- Watch [releases](https://github.com/opena2a-org/agent-identity-management/releases); each release page lists the changes in that version. The [changelog](CHANGELOG.md) also records platform changes that are merged but not yet released.
- Open an [issue](https://github.com/opena2a-org/agent-identity-management/issues) to request a capability or report a bug.
- Report a vulnerability as described in [SECURITY.md](SECURITY.md), not in a public issue.

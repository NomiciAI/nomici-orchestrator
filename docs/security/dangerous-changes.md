# Dangerous Changes

Some changes require extra review because mistakes can expose a user's machine, secrets, runtimes, or agent tools.

This document is a maintainer checklist. It should be referenced in pull requests that touch security-sensitive code or docs.

## Always Security-Sensitive

Changes in these areas require maintainer review:

- Gateway authentication
- Gateway tokens
- OpenAI-compatible `/v1/*` endpoints
- Remote access
- TLS or trusted proxy behavior
- Secrets storage or redaction
- Runtime process execution
- Shell execution
- Filesystem read or write
- MCP server execution or tool invocation
- A2A remote agent communication
- Policy and approval decisions
- Trace export
- Debug bundles
- Install, update, or uninstall scripts
- Release signing, checksums, SBOM, or provenance
- GitHub Actions workflows
- Dependency management and package publishing

## Required PR Notes

For dangerous changes, the PR description must answer:

- What new capability is introduced?
- What can now execute code or call tools?
- What secrets may be read or transmitted?
- What is the default behavior?
- Does the change affect loopback-only defaults?
- Does it affect approval requirements?
- Does it affect audit logs?
- How was redaction tested?
- How can a user disable or restrict the behavior?

## Default-Deny Bias

When behavior is ambiguous, choose the safer default:

- Deny instead of allow.
- Require approval instead of auto-execute.
- Bind to loopback instead of a network interface.
- Avoid transmitting data instead of sending by default.
- Store references instead of raw secrets.

## Install Script Requirements

Install and update scripts must:

- Avoid `sudo` by default.
- Avoid overwriting user config.
- Install to user-writable locations by default.
- Verify checksums when release artifacts exist.
- Support uninstall.
- Redact tokens in logs.
- Print clear failure remediation.

Install scripts must not:

- Read user secrets.
- Upload local config.
- Modify shell profile files without clear consent.
- Run remote code beyond the intended installer path.

## Workflow Requirements

GitHub Actions workflows must:

- Use least-privilege `permissions`.
- Avoid secrets on pull requests from forks.
- Avoid `pull_request_target` unless there is a documented reason.
- Pin third-party actions to immutable refs before stable release hardening.
- Keep release credentials out of normal CI jobs.

## Adapter Requirements

Adapters must:

- Declare their capabilities.
- Declare whether they can run tools.
- Declare whether they can access filesystem, shell, browser, or network.
- Emit trace events for calls.
- Respect cancellation where possible.
- Avoid receiving secrets unless explicitly configured.

Adapters for untrusted or remote systems should default to untrusted policy.

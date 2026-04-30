# Supply Chain Security

Nomici Orchestrator is an agent control plane. Its release artifacts, install scripts, adapters, templates, and dependencies must be treated as security-sensitive.

## Goals

The project should make it possible for users to answer:

- What source produced this binary?
- What commit was used?
- Which build system produced it?
- Which dependencies were used?
- Were checksums and signatures published?
- Were release artifacts produced by CI rather than a developer laptop?

## Dependency Management

Planned defaults:

- Use Go modules for Go dependencies.
- Use pnpm lockfiles for JavaScript dependencies.
- Use Dependabot for Go, npm, and GitHub Actions updates.
- Review dependency updates before merging.
- Treat dependency updates touching install, release, auth, network, or parsing code as security-sensitive.

## GitHub Actions

Default workflow permissions should be read-only:

```yaml
permissions:
  contents: read
```

Release workflows may request additional permissions only for the jobs that require them.

Workflows should avoid:

- Unnecessary repository write permissions.
- Secrets in pull request workflows from forks.
- `pull_request_target` without a documented reason.
- Unpinned third-party actions in release-sensitive jobs.

## Release Artifacts

Release artifacts should eventually include:

- Platform binaries.
- SHA256 checksums.
- Signature or certificate-based verification.
- SBOM.
- Provenance attestation.
- Changelog entry.

Release binaries should be built from CI.

## Install Scripts

Install scripts must verify release artifacts once official releases exist.

Requirements:

- Detect OS and architecture.
- Download from official release locations.
- Verify SHA256.
- Support explicit version selection.
- Support install directory override.
- Support uninstall.
- Avoid `sudo` by default.
- Avoid overwriting config.
- Print clear remediation on failure.

## Adapters and Templates

Adapters and templates can become code execution paths.

Future registries must support:

- Signed packages or checksums.
- Source URL and version metadata.
- Clear trust labels.
- Reviewable manifests.
- No automatic execution from untrusted sources.

## OpenSSF Scorecard

The project should enable OpenSSF Scorecard once the repository is public and CI exists.

Important checks for Nomici:

- Branch protection
- Code review
- Token permissions
- Pinned dependencies
- Dangerous workflows
- Dependency update tool
- Security policy
- Signed releases
- Packaging

## SLSA Direction

The project should work toward release provenance after the first binary release pipeline exists.

Initial target:

- CI-built binaries.
- Checksums.
- Reproducible build commands.
- Documented source commit.

Later target:

- Provenance attestations.
- Signature verification in install scripts.
- SBOM publication.

## Current Status

The project is in RFC and early foundation phase. No release pipeline exists yet.

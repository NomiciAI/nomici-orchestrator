# Open Source Publication Checklist

This checklist must be completed before making `NomiciAI/nomici-orchestrator` public.

Nomici product code may live in a parent workspace that contains multiple sibling projects. Do not publish from the parent directory. The only intended public repository root for this project is a Git repository whose directory basename is:

```text
nomici-orchestrator
```

## Publication Stages

### Stage 1: Private Remote

Safe to do once local checks pass.

Goal:

- Create `NomiciAI/nomici-orchestrator` as a private GitHub repository.
- Push only this repository's tracked files.
- Review GitHub contents in private.

### Stage 2: Quiet Public

Safe when:

- Private GitHub contents have been reviewed.
- README accurately says which alpha bootstrap features are implemented and which are planned.
- No secrets or unrelated project files are present.
- The team accepts that the repo is public but not announced.

Goal:

- Change visibility to public.
- Do not promote broadly yet.

### Stage 3: Public Announcement

Safe when:

- `nomici --version` works.
- `nomici gateway run` works.
- `GET /api/health` works on `127.0.0.1:8787`.
- README quickstart reflects implemented behavior.
- Initial CI passes.
- At least one demo path is honest and reproducible.

## Local Boundary Checks

Run from the repository root:

```bash
git rev-parse --show-toplevel
test "$(basename "$(git rev-parse --show-toplevel)")" = "nomici-orchestrator"
git status --short
git remote -v
git ls-files | sort
```

Expected:

- `git rev-parse --show-toplevel` prints the `nomici-orchestrator` directory.
- The root directory basename check passes.
- `git status --short` is clean before publication.
- `git remote -v` points only to `NomiciAI/nomici-orchestrator` once configured.
- `git ls-files` contains only intended open-source files.

Never run publication commands from a parent workspace or any directory that contains sibling Nomici product repositories.

## Secret Checks

Before pushing or changing visibility, search for likely secrets:

```bash
rg -n --hidden --glob '!.git/**' \
  'sk-[A-Za-z0-9_-]+|ghp_[A-Za-z0-9_]+|gho_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|BEGIN (RSA|OPENSSH|EC|PRIVATE) KEY|api[_-]?key|secret|token|password'
```

Review every match. Documentation may mention words like `token`, `secret`, or `password`, but no real values should appear.

Do not publish:

- `.env` files
- API keys
- Bearer tokens
- Private SSH keys
- Internal customer data
- Private traces
- Private prompts
- Internal Nomici product docs unrelated to Orchestrator

## Repository Creation

Create the private repository from the correct root:

```bash
gh repo create NomiciAI/nomici-orchestrator \
  --private \
  --source . \
  --remote origin \
  --push \
  --description "Open-source long-horizon agent harness for orchestrating, observing, and governing multi-agent AI work."
```

Important:

- Use `--source .`, not `--source ..`.
- Keep the repository private first.
- Review the GitHub file tree before changing visibility.

## Required GitHub Settings Before Public

Configure:

- Branch protection for `main`.
- Disable force-pushes to `main`.
- Disable deletion of `main`.
- Enable Dependabot alerts and security updates.
- Enable private vulnerability reporting or GitHub Security Advisories.
- Keep Actions workflow token permissions read-only by default.
- Ensure `.github/workflows/ci.yml` passes on `main`.
- Ensure `CODEOWNERS` references an existing GitHub team.

## Install Script Review

Current alpha bootstrap installer:

```bash
scripts/install.sh --from-source .
```

Before quiet public, verify:

- It does not use `sudo` by default.
- It installs to `~/.local/bin/nomici` by default.
- It supports `--install-dir`.
- It supports `--version <version>` for release selection.
- It supports `--uninstall`.
- It backs up an existing binary before replacement.
- It does not read or upload user configuration or secrets.
- Release download mode verifies `checksums.txt` unless `--skip-checksum` is explicitly passed.
- Hosted `curl` install instructions are not advertised as live until release artifacts and the install endpoint exist.

## Release Artifact Workflow

Tag pushes matching `v*` run `.github/workflows/release.yml`.

The workflow currently:

- builds Console assets once per target job
- cross-compiles `nomici` for Linux and macOS on amd64 and arm64
- uploads `.tar.gz` archives and SHA-256 files as workflow artifacts
- publishes a GitHub release with archives and a combined `checksums.txt` for tag builds

Before announcing stable releases, verify:

- release artifacts install cleanly with `scripts/install.sh --version <tag>`
- `nomici setup`, `nomici doctor`, and `nomici dev --no-open` work from the downloaded binary
- signing/notarization requirements are documented for macOS if needed

## Bootstrap Mode

During alpha bootstrap, `main` may allow direct maintainer pushes so the project can move quickly before there are external contributors.

Bootstrap mode may keep:

- `main` as the default branch.
- Force-push protection.
- Branch deletion protection.
- Dependabot alerts and security updates.
- GitHub Actions default token permissions set to read-only.

Bootstrap mode may temporarily disable:

- Required pull request reviews.
- Required CODEOWNERS reviews.
- Admin enforcement.

Alpha bootstrap verification commands:

```bash
gh api orgs/NomiciAI/teams --jq '.[].slug'
gh api repos/NomiciAI/nomici-orchestrator/branches/main/protection \
  --jq '{required_status_checks, required_pull_request_reviews, enforce_admins, allow_force_pushes, allow_deletions, required_conversation_resolution}'
gh api repos/NomiciAI/nomici-orchestrator/actions/permissions/workflow --jq .
gh api repos/NomiciAI/nomici-orchestrator/vulnerability-alerts --method GET --include
```

Expected alpha bootstrap state:

- `maintainers` exists as a real team in `NomiciAI`.
- `main` disallows force-pushes.
- `main` disallows branch deletion.
- Required PR review, CODEOWNERS review, and admin enforcement may be disabled until quiet public.
- Actions default workflow permissions are `read`.
- Dependabot vulnerability alerts return `204 No Content`.

If GitHub Advanced Security is not enabled for the private repository, GitHub secret scanning may be unavailable while private. Run local secret scans before every push and enable GitHub secret scanning if it becomes available before or after publication.

## Solo Maintainer Public Mode

For quiet public with one active maintainer, use a lighter public protection mode:

- Require the CI status check before merge.
- Require conversation resolution.
- Enforce branch protection for admins.
- Keep force-push protection enabled.
- Keep branch deletion protection enabled.
- Keep required pull request review disabled.
- Keep required CODEOWNERS review disabled.

This keeps direct pushes to `main` blocked and keeps CI mandatory, but allows the solo maintainer to merge their own PR after CI passes.

## Team Strict Mode Later

When Nomici has at least two active maintainers or begins receiving external contributions, restore stricter review rules:

- Require pull request review before merge.
- Require CODEOWNERS review.
- Dismiss stale reviews.
- Keep required CI, conversation resolution, admin enforcement, force-push protection, and branch deletion protection enabled.

## Naming Checks

Use:

```text
NomiciAI/nomici-orchestrator
```

Avoid old placeholder names:

```text
nomici-ai/orchestrator
NomiciAI/orchestrator
nomici-ai
```

## Final Public Visibility Check

Before changing visibility to public, verify:

```bash
gh repo view NomiciAI/nomici-orchestrator --json nameWithOwner,visibility,url
git remote -v
git ls-files | sort
```

Then review the private GitHub repository in the browser.

Only change visibility to public after explicit approval from the project owner.

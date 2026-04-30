# Open Source Publication Checklist

This checklist must be completed before making `NomiciAI/nomici-orchestrator` public.

Nomici product code lives in a parent workspace that contains multiple sibling projects. Do not publish from the parent directory. The only intended public repository root for this project is:

```text
/Users/stephen/Documents/nomici/code/nomici-orchestrator
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
- README accurately says the project is in design/RFC phase.
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
cd /Users/stephen/Documents/nomici/code/nomici-orchestrator
git rev-parse --show-toplevel
git status --short
git remote -v
git ls-files | sort
```

Expected:

- `git rev-parse --show-toplevel` prints the `nomici-orchestrator` directory.
- `git status --short` is clean before publication.
- `git remote -v` points only to `NomiciAI/nomici-orchestrator` once configured.
- `git ls-files` contains only intended open-source files.

Never run publication commands from:

```text
/Users/stephen/Documents/nomici/code
/Users/stephen/Documents/nomici
```

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
cd /Users/stephen/Documents/nomici/code/nomici-orchestrator
gh repo create NomiciAI/nomici-orchestrator \
  --private \
  --source . \
  --remote origin \
  --push \
  --description "Open-source control plane for local and remote AI agents."
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

## Bootstrap Mode

During private bootstrap, `main` may allow direct maintainer pushes so the project can move quickly before there are external contributors.

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

## Strict Mode Before Quiet Public

Before changing the repository from private to quiet public, restore strict branch protection:

- Require pull request review before merge.
- Require CODEOWNERS review.
- Dismiss stale reviews.
- Require conversation resolution.
- Enforce branch protection for admins.
- Keep force-push protection enabled.
- Keep branch deletion protection enabled.

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

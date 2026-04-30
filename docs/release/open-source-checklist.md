# Open Source Publication Checklist

This checklist must be completed before making `NomiciAI/orchestrator` public.

Nomici product code lives in a parent workspace that contains multiple sibling projects. Do not publish from the parent directory. The only intended public repository root for this project is:

```text
/Users/stephen/Documents/nomici/code/nomici-orchestrator
```

## Publication Stages

### Stage 1: Private Remote

Safe to do once local checks pass.

Goal:

- Create `NomiciAI/orchestrator` as a private GitHub repository.
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
- `git remote -v` points only to `NomiciAI/orchestrator` once configured.
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
gh repo create NomiciAI/orchestrator \
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

- Branch protection for `master` or `main`.
- Require pull request review before merge.
- Require CODEOWNERS review once the `NomiciAI/maintainers` team exists.
- Disable force-pushes to the protected branch.
- Enable Dependabot alerts and security updates.
- Enable private vulnerability reporting or GitHub Security Advisories.
- Keep Actions workflow token permissions read-only by default.

## Naming Checks

Use:

```text
NomiciAI/orchestrator
```

Avoid old placeholder names:

```text
nomici-ai/orchestrator
nomici-ai
```

## Final Public Visibility Check

Before changing visibility to public, verify:

```bash
gh repo view NomiciAI/orchestrator --json nameWithOwner,visibility,url
git remote -v
git ls-files | sort
```

Then review the private GitHub repository in the browser.

Only change visibility to public after explicit approval from the project owner.

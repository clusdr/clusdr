## What

<!-- One or two sentences. The commit subject should already say this. -->

## Why

<!-- User-visible reason, or "docs only" / "tests only". -->

## Testing

- [ ] `make test`, `make vet`, and `make lint` (skip if docs-only)
- [ ] New behavior covered by automated tests (or N/A with reason)
- [ ] `make proto` and `make proto-lint` if `proto/` moved
- [ ] Docs under `docs/` updated in this change if config, CLI, proto, SDK, or defaults moved
- [ ] CI green on this PR before merge (do not tag from red `main`)

## Commits

This PR uses [Conventional Commits](https://www.conventionalcommits.org/). Prefer one commit, or a short stack of `feat` / `fix` / `docs` / `ci` / `chore` subjects. Header and body lines at most 100 characters. GitHub squash-merge title must stay conventional. `make hooks` installs the local checks.

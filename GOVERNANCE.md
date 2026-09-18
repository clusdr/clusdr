# Governance

clusdr is a small public project. Decisions that affect the daemon, API, or
releases are made in this repository.

## Roles

**Project lead** (`@durguto`) owns the `clusdr` GitHub org, production
secrets, and release tags. The lead may merge a pull request after required CI
is green. That includes their own changes: there is one maintainer today.

**Maintainer** is anyone listed in [MAINTAINERS.md](MAINTAINERS.md). Maintainers
review pull requests, cut tags from green `main`, and rotate secrets after a
leak.

**Contributor** opens pull requests. No org admin is required.

Language SDKs and clusdr.io live in sibling repositories. They follow the same
security policy ([SECURITY.md](SECURITY.md)) and the same pull-request + CI
bar as this repo.

## How a change lands

Every change to `main` is a pull request. Required checks must pass. Prefer
squash-merge; the squash title stays conventional ([CONTRIBUTING.md](CONTRIBUTING.md)).

## Escalated access

Org owner, repo admin, or a new maintainer is escalated permission. Do not
grant it in chat. Open a pull request (or a documented issue linked from a PR)
that names the person, the role, and why. Wait **seven days** after that
write-up is on `main` before flipping GitHub org/repo roles, unless a
compromised account forces an immediate revoke.

Revoking access does not wait.

## Secrets

CI secrets (registry tokens, Buf, and the like) live in GitHub Actions secrets,
never in git. Only maintainers can read or change them. Rotate a secret when it
leaks, when a maintainer leaves, or when the provider says so. Personal access
tokens used as secrets must be scoped to the job and expire.

## Continuity

There is one maintainer. If that person is unreachable, GitHub org recovery
follows GitHub’s account-recovery process; there is no second owner to hand
keys to yet. That gap is known.

## Related

[MAINTAINERS.md](MAINTAINERS.md) · [SECURITY.md](SECURITY.md) · [CONTRIBUTING.md](CONTRIBUTING.md)

# Security Policy

Do not file a public issue for a vulnerability.

Language SDKs ([clusdr-python](https://github.com/clusdr/clusdr-python),
[clusdr-rust](https://github.com/clusdr/clusdr-rust),
[clusdr-js](https://github.com/clusdr/clusdr-js),
[clusdr-java](https://github.com/clusdr/clusdr-java)) and
[clusdr-site](https://github.com/clusdr/clusdr-site) use this policy. Report
there if the bug is only in that repo; otherwise report here.

## Report

Use [GitHub private vulnerability reporting](https://github.com/clusdr/clusdr/security/advisories/new).

Include:

- Affected version (`clusdr version`, image tag, or module version)
- What you did
- What happened
- Whether you have a patch

## Response times

We acknowledge a report within **5 days**. Within **14 days** of a
reproducible report we either describe a fix path, ship a patch on the current
release train, or explain why it is not a vulnerability. Conversation stays in
the advisory until then.

There is one maintainer. If those clocks slip (travel, illness), we say so in
the advisory.

## Support window

The wire API is **v1alpha1**. We patch the current release train. There is no
long-term support window yet.

## Vulnerabilities in dependencies

Dependabot runs weekly on Go modules, GitHub Actions, and Docker base images.
`govulncheck` is welcome in a PR.

Before a GA tag (`vX.Y.Z` with no hyphen):

- **Critical or high** known issues in what we ship must be fixed or written
  off in the release notes (why it does not apply).
- **Medium or low** may ship with a follow-up issue.

CI SAST is CodeQL plus `golangci-lint` (`max-issues-per-linter` / `max-same-issues`
are 0). A new CodeQL or lint finding that is a real bug blocks merge. Findings
we dismiss are commented on the PR.

## Secrets

How secrets are stored and who may touch them is in [GOVERNANCE.md](GOVERNANCE.md).

## What we trust

Trust boundaries for the daemon (join token, mTLS, Raft, local SDK) are in
[docs/concepts/security.md](docs/concepts/security.md). Releases are built by
GitHub Actions on a tag, not from a laptop; how to check that is in
[docs/guide/install.md](docs/guide/install.md).

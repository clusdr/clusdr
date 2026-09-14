# `clusdr members`

Lists all known members: `ID`, `ADDRESS`, `STATUS`, `ROLE`.

Role is `leader`, `voter`, or `observer`.

## Synopsis

```bash
clusdr members
```

Empty cluster (should not happen on a running seed): prints `no members`.

Error if the Runtime API is down.

## See also

- [Start the first member](../../guide/first-member.md)
- [Membership](../../concepts/membership.md)
- [MembershipService](../api/membership.md)

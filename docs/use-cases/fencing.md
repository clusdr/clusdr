# Fence off a stale writer

A lock tells a process it may write. It cannot stop a process that **was** told it may write, then stalled long enough for the cluster to give the lock to someone else, and then woke up and wrote anyway. Fencing tokens close that hole: every grant comes with a number higher than any before it, and the store refuses writes that carry a lower one.

<UseMeta />

## Why a lock is not enough

A process does not notice its own pauses. A long garbage collection, a VM migration, swapping, a `SIGSTOP`, or a slow disk can freeze it for longer than the TTL. While it is frozen its renewals stop, the grant expires, and the cluster hands the name to the next waiter. When the frozen process resumes, it is still in the middle of the code that runs "while holding the lock" and has no reason to check again before its next write.

<UseTimeline name="fence" fig="Seq. A" caption="Writer a holds the lock with token 41, then freezes. The grant expires and writer b gets token 42 and writes. When a wakes and writes with 41, the store has already seen 42 and refuses it." />

Checking the deadline before each write, as [Run one active replica](leader-election.md) does, narrows the window but cannot close it: the pause can land between the check and the write. Only the receiving side can make the final call, because it sees writes in the order they actually arrive.

## The token

Every lock and lease grant carries a fencing token (`lk.Token`, `lease.Token`). Tokens come from the Raft log, so for a given name each new grant's token is higher than every earlier grant's. The token does not change when the grant is renewed, only when the name changes hands.

<UseFigure name="fencing" fig="Fig. A" caption="Writer b's token 42 passes. The store has recorded 42 as the highest token it has seen, so writer a's late write with 41 is stopped." />

The rule for the store is short: **remember the highest token you have accepted for this resource, and reject any write with a lower one.**

## In a SQL database

Keep the token next to the data it protects, and make the comparison part of the write so it is atomic:

```sql
-- once
ALTER TABLE accounts ADD COLUMN fence bigint NOT NULL DEFAULT 0;

-- every fenced write: $1 is lk.Token
UPDATE accounts
   SET balance = $2,
       fence   = $1
 WHERE id = $3
   AND fence <= $1;
```

If the statement updates zero rows, a newer holder has written since this one got its token. Treat that as "I am no longer the holder": stop, and do not retry with the same token.

For writes that span several rows, keep one fence row per lock name and lock it in the transaction:

```sql
BEGIN;
SELECT token FROM fences WHERE name = 'jobs.active' FOR UPDATE;
-- abort if token > $1
UPDATE fences SET token = $1 WHERE name = 'jobs.active';
-- … the real writes …
COMMIT;
```

## In code

::: code-group

```go [Go]
lk, err := c.Lock(ctx, "accounts.rebalance", 15*time.Second)
if err != nil {
	return err
}
res, err := db.ExecContext(ctx,
	`UPDATE accounts SET balance = $2, fence = $1 WHERE id = $3 AND fence <= $1`,
	int64(lk.Token), balance, id)
if err != nil {
	return err
}
if n, _ := res.RowsAffected(); n == 0 {
	return errors.New("fenced: a newer holder has written") // stand down
}
```

```python [Python]
lk = c.lock("accounts.rebalance", ttl=15)
cur.execute(
    "UPDATE accounts SET balance = %s, fence = %s WHERE id = %s AND fence <= %s",
    (balance, lk.token, account_id, lk.token),
)
if cur.rowcount == 0:
    raise RuntimeError("fenced: a newer holder has written")  # stand down
```

```ts [TypeScript]
const lk = await c.lock("accounts.rebalance", 15);
const res = await db.query(
  "UPDATE accounts SET balance = $2, fence = $1 WHERE id = $3 AND fence <= $1",
  [lk.token, balance, id],
);
if (res.rowCount === 0) {
  throw new Error("fenced: a newer holder has written"); // stand down
}
```

:::

## Other stores

| Store | Where the token goes |
|---|---|
| SQL | A `fence` column with `WHERE fence <= $token`, or a fence row locked `FOR UPDATE` |
| Object storage | Write `key.<token>` and have readers take the highest, or use a conditional put on a metadata field |
| Key-value with compare-and-set | Store `{token, value}` and CAS only when the stored token is lower or equal |
| An HTTP API you own | Send `Fencing-Token: 42`; the service keeps the highest per resource and returns `409` for lower ones |
| A queue or topic | Put the token in the message; consumers drop messages below the highest token they have processed |

If a downstream system cannot compare tokens, it cannot be fenced. In that case keep the work inside one unit shorter than the TTL margin, and accept that a long enough pause can still produce a duplicate.

## Things to get right

- **Compare in the store, not in your process.** Reading the stored token, comparing in code, then writing has the same gap the fence is meant to close.
- **Use `<=`, not `<`.** The same holder writes many times with the same token. Equal is allowed; lower is refused.
- **One fence per lock name.** Tokens are ordered for one name. Do not compare tokens from `jobs.a` with tokens from `jobs.b`.
- **Leases fence too.** A lease's token works the same way when a lease guards the resource, as in [worker liveness](worker-liveness.md).
- **Unlock is already fenced.** An `Unlock` carries the holder and token, so a stale holder cannot release a newer grant by accident. You do not need to guard that yourself.

## Related

- [Run one active replica](leader-election.md) and [Run each scheduled job once](scheduled-jobs.md), which both rely on this
- [Locks](../concepts/locks.md) and [Leases](../concepts/leases.md) for where tokens come from
- [Consistency](../concepts/consistency.md) for what the Raft log does and does not order

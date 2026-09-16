/**
 * Print cluster membership, then follow Watch until Ctrl-C.
 *
 * The process is an application. It does not vote. It talks only to the
 * daemon on this host (CLUSDR_GRPC_ADDR or 127.0.0.1:7947).
 *
 *     clusdr init && clusdr start --bootstrap
 *     npx tsx examples/who/typescript/main.ts
 *     npx tsx examples/who/typescript/main.ts --once
 */

import { ClusdrError, local, type Cluster, type Event } from "clusdr";

function parseOnce(argv: string[]): boolean {
  return argv.includes("--once");
}

async function main(): Promise<void> {
  let c;
  try {
    c = await local();
  } catch (err) {
    console.error(`connect failed: ${err}`);
    process.exitCode = 1;
    return;
  }

  try {
    await printMembers(c);
    if (parseOnce(process.argv.slice(2))) {
      return;
    }
    console.error("watching cluster events (Ctrl-C to stop)");
    for await (const ev of c.watch()) {
      console.log(formatEvent(ev));
    }
  } catch (err) {
    if (err instanceof ClusdrError) {
      console.error(err.message);
      process.exitCode = 1;
    } else if ((err as { name?: string }).name !== "AbortError") {
      throw err;
    }
  } finally {
    await c.close();
  }
}

async function printMembers(c: Cluster): Promise<void> {
  const members = await c.members();
  console.log(
    `${"ID".padEnd(24)} ${"ADDRESS".padEnd(22)} ${"STATUS".padEnd(8)} ${"ROLE".padEnd(10)} LEADER`,
  );
  for (const m of members) {
    const role = m.role || "voter";
    console.log(
      `${m.id.padEnd(24)} ${m.address.padEnd(22)} ${m.status.padEnd(8)} ${role.padEnd(10)} ${m.leader}`,
    );
  }
  const leader = await c.leader();
  console.log(`leader ${leader.id} at ${leader.address}`);
}

function formatEvent(ev: Event): string {
  let kind = "bus";
  if (ev.type === "member.join" || ev.type === "member.left" || ev.type === "leader.changed") {
    kind = "cluster";
  } else if (ev.type.startsWith("custom.")) {
    kind = "gossip";
  } else if (ev.type === "watch.sync" || ev.type === "watch.gap") {
    kind = "watch";
  }
  let payload = "";
  if (ev.payload.length > 0) {
    payload = " " + Buffer.from(ev.payload).toString("utf8");
  }
  return `${kind} seq=${ev.seq} ${ev.type} src=${ev.source}${payload}`;
}

process.on("SIGINT", () => {
  console.error("stopping");
  process.exit(0);
});

await main();

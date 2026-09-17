/**
 * Watch the local daemon: membership snapshot, a worker lease, then the bus.
 *
 *     clusdr init && clusdr start --bootstrap
 *     npx tsx examples/watch/typescript/main.ts --name edge-1
 *
 * In another terminal:
 *
 *     clusdr publish ping '{"from":"cli"}'
 *
 * custom.* is gossip (not Raft, not replayed). member.dead is crash (still listed).
 * member.left is clusdr leave (gone).
 * close() revokes the worker lease.
 */

import { ClusdrError, local, type Cluster, type Event } from "clusdr";

function arg(name: string, fallback: string): string {
  const i = process.argv.indexOf(name);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1]! : fallback;
}

async function main(): Promise<void> {
  const name = arg("--name", `pid-${process.pid}`);
  const once = process.argv.includes("--once");

  let c;
  try {
    c = await local();
  } catch (err) {
    console.error(`connect failed: ${err}`);
    console.error(
      "need a running daemon and TLS certs in ~/.clusdr (or CLUSDR_TLS=disabled on daemon and client)",
    );
    process.exitCode = 1;
    return;
  }

  const leaseName = `worker.${name}`;
  try {
    await printMembers(c);
    if (once) {
      return;
    }
    const ls = await c.lease(leaseName, 15);
    const until = ls.deadline?.toISOString() ?? "?";
    console.log(`lease ${ls.name} owner=${ls.owner} token=${ls.token} until ${until}`);
    await c.publish("hello", { worker: name, pid: process.pid, lease: leaseName });
    console.log('watching (Ctrl-C to stop); try: clusdr publish ping \'{"from":"cli"}\'');
    for await (const ev of c.watch()) {
      console.log(formatEvent(ev));
    }
  } catch (err) {
    if (err instanceof ClusdrError) {
      console.error(err.message);
      process.exitCode = 1;
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
  if (ev.type === "member.join" || ev.type === "leader.changed") {
    kind = "cluster";
  } else if (ev.type === "member.dead") {
    kind = "dead";
  } else if (ev.type === "member.left") {
    kind = "left";
  } else if (ev.type.startsWith("custom.")) {
    kind = "gossip";
  } else if (ev.type === "watch.sync" || ev.type === "watch.gap") {
    kind = "watch";
  }
  const ts = ev.timestamp.toISOString().slice(11, 19);
  let payload = "";
  if (ev.payload.length > 0) {
    payload = " " + Buffer.from(ev.payload).toString("utf8");
  }
  return `${kind} ${ts} seq=${ev.seq} ${ev.type} src=${ev.source}${payload}`;
}

process.on("SIGINT", () => {
  console.error("stopping");
  process.exit(0);
});

await main();

/**
 * Agents signalling each other over the local daemon — no broker.
 *
 * Publish is 1-hop gossip, not Raft. It is not a queue. A subscriber
 * that connects later will not see old custom events.
 *
 *     clusdr init && clusdr start --bootstrap
 *     npx tsx examples/agent/typescript/main.ts --mode listen
 *     npx tsx examples/agent/typescript/main.ts --mode emit --from mapper
 */

import { ClusdrError, local, type Cluster } from "clusdr";

const TOPIC = "agent.task";

function arg(name: string, fallback: string): string {
  const i = process.argv.indexOf(name);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1]! : fallback;
}

async function main(): Promise<void> {
  const mode = arg("--mode", "both");
  const source = arg("--from", `pid-${process.pid}`);
  const every = Number(arg("--every", "2"));

  let c;
  try {
    c = await local();
  } catch (err) {
    console.error(`connect failed: ${err}`);
    process.exitCode = 1;
    return;
  }

  const stop = new AbortController();
  try {
    if (mode === "emit" || mode === "both") {
      void emitLoop(c, source, every, stop.signal);
      console.log(`emitting custom.${TOPIC} every ${every}s from=${source}`);
    }
    if (mode === "listen" || mode === "both") {
      console.log(`listening for custom.${TOPIC} (Ctrl-C to stop)`);
      for await (const ev of c.watch({ topics: [TOPIC] })) {
        const body = ev.payload.length ? Buffer.from(ev.payload).toString("utf8") : "";
        const ts = ev.timestamp.toISOString().slice(11, 19);
        console.log(`${ts} seq=${ev.seq} ${ev.type} src=${ev.source} ${body}`);
      }
    } else {
      console.log("emitting (Ctrl-C to stop)");
      await new Promise(() => {
        // hold until SIGINT
      });
    }
  } catch (err) {
    if (err instanceof ClusdrError) {
      console.error(err.message);
      process.exitCode = 1;
    }
  } finally {
    stop.abort();
    await c.close();
  }
}

async function emitLoop(c: Cluster, source: string, every: number, signal: AbortSignal): Promise<void> {
  let n = 0;
  while (!signal.aborted) {
    await new Promise((resolve) => setTimeout(resolve, every * 1000));
    if (signal.aborted) {
      return;
    }
    n += 1;
    try {
      await c.publish(TOPIC, { from: source, n, pid: process.pid });
    } catch (err) {
      if (err instanceof ClusdrError) {
        console.error(`publish failed: ${err.message}`);
      }
      return;
    }
    console.log(`sent n=${n} from=${source}`);
  }
}

process.on("SIGINT", () => {
  console.error("stopping");
  process.exit(0);
});

await main();

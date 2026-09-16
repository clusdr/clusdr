/**
 * Hold a named lease the way a shard worker would.
 *
 * A lease does not wait. If the name is taken, grant fails immediately
 * (unlike scheduler, which retries tryLock). close() revokes.
 *
 *     clusdr init && clusdr start --bootstrap
 *     npx tsx examples/worker/typescript/main.ts --name shard-7 --owner worker-a
 *     npx tsx examples/worker/typescript/main.ts --name shard-7 --owner worker-b
 */

import { ClusdrError, local } from "clusdr";

function arg(name: string, fallback: string): string {
  const i = process.argv.indexOf(name);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1]! : fallback;
}

function num(name: string, fallback: number): number {
  const v = Number(arg(name, String(fallback)));
  return Number.isFinite(v) ? v : fallback;
}

async function main(): Promise<void> {
  const name = arg("--name", "shard-7");
  const owner = arg("--owner", "");
  const ttl = num("--ttl", 15);

  let c;
  try {
    c = await local({ holder: owner });
  } catch (err) {
    console.error(`connect failed: ${err}`);
    process.exitCode = 1;
    return;
  }

  try {
    const ls = await c.lease(name, ttl);
    const until = ls.deadline?.toISOString() ?? "?";
    console.error(`granted name=${ls.name} owner=${ls.owner} token=${ls.token} until=${until}`);
    console.error("holding shard (Ctrl-C to stop; close revokes)");
    await new Promise(() => {
      // hold until SIGINT
    });
  } catch (err) {
    if (err instanceof ClusdrError) {
      console.error(`lease failed: ${err.message}`);
      process.exitCode = 1;
    }
  } finally {
    await c.close();
  }
}

process.on("SIGINT", () => {
  console.error("stopping");
  process.exit(0);
});

await main();

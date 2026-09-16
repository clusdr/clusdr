/**
 * Hold one exclusive cluster lock and “dispatch” work.
 *
 * Run two copies with different --holder values. Only one replica holds
 * "scheduler" at a time. close() unlocks.
 *
 *     clusdr init && clusdr start --bootstrap
 *     npx tsx examples/scheduler/typescript/main.ts --holder replica-a
 *     npx tsx examples/scheduler/typescript/main.ts --holder replica-b
 *
 * tryLock returns null when the name is taken (no current-holder object).
 */

import { ClusdrError, local, type Cluster } from "clusdr";

function arg(name: string, fallback: string): string {
  const i = process.argv.indexOf(name);
  return i >= 0 && process.argv[i + 1] ? process.argv[i + 1]! : fallback;
}

function num(name: string, fallback: number): number {
  const v = Number(arg(name, String(fallback)));
  return Number.isFinite(v) ? v : fallback;
}

async function main(): Promise<void> {
  const name = arg("--name", "scheduler");
  const holder = arg("--holder", "");
  const work = num("--work", 3);
  const wait = num("--wait", 1);

  let c;
  try {
    c = await local({ holder });
  } catch (err) {
    console.error(`connect failed: ${err}`);
    process.exitCode = 1;
    return;
  }

  console.error(`scheduler replica lock=${name} work=${work}s`);
  try {
    while (true) {
      await shift(c, name, work, wait);
    }
  } catch (err) {
    if (err instanceof ClusdrError) {
      console.error(`shift failed: ${err.message}`);
      process.exitCode = 1;
    }
  } finally {
    await c.close();
  }
}

async function shift(c: Cluster, name: string, work: number, wait: number): Promise<void> {
  const lk = await c.tryLock(name, 15, 5);
  if (lk === null) {
    console.error("waiting held_by=another replica");
    await sleep(wait);
    return;
  }

  const until = lk.deadline?.toISOString() ?? "?";
  console.error(`held name=${lk.name} holder=${lk.holder} token=${lk.token} until=${until}`);
  console.log(`dispatch job=rollout fence=${lk.token} holder=${lk.holder}`);
  await sleep(work);
  await c.unlock(name);
  console.error(`released name=${name}`);
  await sleep(wait);
}

function sleep(seconds: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, seconds * 1000));
}

process.on("SIGINT", () => {
  console.error("stopping");
  process.exit(0);
});

await main();

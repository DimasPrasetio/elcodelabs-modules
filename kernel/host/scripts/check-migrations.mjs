#!/usr/bin/env node
// golang-migrate keeps ONE version per database and only applies files above it. A migration
// added on a branch with a timestamp older than one already merged would never run anywhere
// that ran the newer one — silently. CI runs this against the base commit; it fails on such a
// file, and --fix gives it (and its .down) a fresh timestamp after the newest one.
//
// Usage: node scripts/check-migrations.mjs <base-commit> [--fix]
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync, renameSync, writeFileSync } from "node:fs";

const DIR = "apps/api/migrations";
const [base, flag] = process.argv.slice(2);
const git = (...args) => execFileSync("git", args, { encoding: "utf8" });
const version = (f) => f.match(/^(\d{14})_/)?.[1];

if (!base || /^0+$/.test(base)) {
  console.log("check-migrations: no base commit (first push of a branch) — skipped");
  process.exit(0);
}
try {
  git("cat-file", "-e", `${base}^{commit}`);
} catch {
  console.log(`check-migrations: ${base} is not in this checkout (fetch-depth 0?) — skipped`);
  process.exit(0);
}
const before = git("ls-tree", "--name-only", `${base}:${DIR}`).split("\n").map(version).filter(Boolean);
const newest = before.sort().at(-1) ?? "0";
const added = git("diff", "--name-only", "--diff-filter=A", base, "--", DIR)
  .split("\n").filter(Boolean).map((p) => p.split("/").at(-1)).filter((f) => version(f));
const late = [...new Set(added.filter((f) => version(f) <= newest).map((f) => f.replace(/\.(up|down)\.sql$/, "")))];

if (late.length === 0) {
  console.log(`check-migrations: ${added.length} new migration file(s), all after ${newest}`);
  process.exit(0);
}
if (flag !== "--fix") {
  console.error(`✗ older than ${newest}, the newest migration on the base branch — they would never run:`);
  for (const b of late) console.error(`  ${b}`);
  console.error("  Fix: node scripts/check-migrations.mjs " + base + " --fix   (renames them; they have not run anywhere yet)");
  process.exit(1);
}
// Fresh, ordered timestamps after everything that exists — on the base branch and here.
const all = readdirSync(DIR).map(version).filter(Boolean).concat(newest).sort();
let t = Math.max(Date.now(), Date.parse(all.at(-1).replace(/^(\d{4})(\d\d)(\d\d)(\d\d)(\d\d)(\d\d)$/, "$1-$2-$3T$4:$5:$6Z")) + 1000);
const lockFile = "modules.lock.json";
let lock = existsSync(lockFile) ? readFileSync(lockFile, "utf8") : null;
for (const oldBase of late.sort()) {
  const stamp = new Date(t).toISOString().replace(/[-:T]/g, "").slice(0, 14);
  const newBase = stamp + oldBase.slice(14);
  for (const dir of ["up", "down"]) {
    if (existsSync(`${DIR}/${oldBase}.${dir}.sql`)) renameSync(`${DIR}/${oldBase}.${dir}.sql`, `${DIR}/${newBase}.${dir}.sql`);
  }
  if (lock) lock = lock.replaceAll(`"${oldBase}"`, `"${newBase}"`);
  console.log(`  ${oldBase} → ${newBase}`);
  t += 1000;
}
if (lock) writeFileSync(lockFile, lock);
console.log("✓ renamed — commit the change");

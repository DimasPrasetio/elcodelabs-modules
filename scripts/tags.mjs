#!/usr/bin/env node
// Tags every module and preset version that has no release tag yet: <id>/v<version> at HEAD.
// A version's files never change after it is released (catalog.mjs --since refuses that), so
// any commit since the bump holds the same files — HEAD included. Projects record the commit
// they installed from; the tag is the fallback when that commit is gone (never pushed, or
// history rewritten). The post-commit hook runs this; push with `git push --follow-tags`.
//
// Usage: node scripts/tags.mjs [--check]   --check: list missing tags, exit 1 if any
import { spawnSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const git = (...args) => spawnSync("git", args, { cwd: ROOT, encoding: "utf8" });
const CHECK = process.argv.includes("--check");

if (git("status", "--porcelain", "--", "modules", "themes").stdout.trim()) {
  console.log("tags: modules/ or themes/ have uncommitted changes — tag after committing");
  process.exit(CHECK ? 1 : 0);
}
const missing = [];
for (const [dir, manifest] of [["modules", "module.json"], ["themes", "preset.json"]]) {
  if (!existsSync(join(ROOT, dir))) continue;
  for (const id of readdirSync(join(ROOT, dir)).sort()) {
    const p = join(ROOT, dir, id, manifest);
    if (!existsSync(p)) continue;
    const tag = `${id}/v${JSON.parse(readFileSync(p, "utf8")).version}`;
    if (git("rev-parse", "--verify", "--quiet", `refs/tags/${tag}`).status !== 0) missing.push(tag);
  }
}
if (CHECK) {
  if (missing.length) console.error(`missing release tags: ${missing.join(", ")} — run node scripts/tags.mjs`);
  process.exit(missing.length ? 1 : 0);
}
for (const tag of missing) {
  const r = git("tag", "-a", tag, "-m", tag);
  console.log(r.status === 0 ? `tagged ${tag}` : `could not tag ${tag}: ${r.stderr.trim()}`);
}
if (!missing.length) console.log("tags: every version is tagged");

#!/usr/bin/env node
// check — the library's check as a branch needs it: what `npm run check`, the pre-commit hook,
// and CI on a pushed branch run.
//
//   on main (or a detached HEAD)   every commit is a release: --since HEAD, every gate enforced
//   on a work branch               the branch is one release in the making: --since where it
//                                  left main (one version bump for the whole branch), and
//                                  testbed-covered and status-current only warn
//
// A pull request is checked by CI against its base with every gate enforced; that is the gate.
//
// Usage: node scripts/check.mjs [--staged]
//   --staged  (the pre-commit hook) also refuse when files the check reads have unstaged or
//             untracked changes — the check reads the working tree, the commit takes the index.

import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const git = (...args) => spawnSync("git", args, { cwd: ROOT, encoding: "utf8" });
const READ = ["modules", "themes", "kernel", "scripts", "testbed", "AGENTS.md", "STATUS.md", "CATALOG.md", "catalog.json"];

if (process.argv.includes("--staged")) {
  const unstaged = git("diff", "--name-only", "--", ...READ).stdout.trim();
  const untracked = git("ls-files", "--others", "--exclude-standard", "--", ...READ).stdout.trim();
  if (unstaged || untracked) {
    process.stderr.write(`check: these files differ from what is staged — git add them, or stash them, so the check sees what the commit takes:\n${[unstaged, untracked].filter(Boolean).join("\n")}\n`);
    process.exit(1);
  }
}

// CI names the branch (its checkout may be detached); locally git does.
const branch = process.env.GITHUB_REF_NAME || git("rev-parse", "--abbrev-ref", "HEAD").stdout.trim();
let args = ["--check", "--since", "HEAD"];
if (branch !== "main" && branch !== "HEAD") {
  const base = ["origin/main", "main"].map((ref) => git("merge-base", "HEAD", ref).stdout.trim()).find(Boolean);
  if (base) args = ["--check", "--since", base, "--branch-work"];
}
const r = spawnSync(process.execPath, [join(ROOT, "scripts/catalog.mjs"), ...args], { cwd: ROOT, stdio: "inherit" });
process.exit(r.status ?? 1);

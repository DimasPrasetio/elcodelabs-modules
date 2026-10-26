#!/usr/bin/env node
// kernel-sync — pins the library to a released kernel: scaffolds a project with
// monorepo-elcodelabs exactly as committed at a release tag (never from a working tree), and
// writes it to kernel/host — with the kernel references (kernel/reference), a README, and
// kernel/kernel.lock.json. For whoever maintains the skill; module
// developers never run it. Commit the result by itself (the catalog check refuses a commit that
// mixes kernel/ with anything else), after the testbed passes on it.
//
// Usage: node scripts/kernel-sync.mjs --skill <monorepo-elcodelabs folder in a git clone> [--ref <tag|commit>]
//   --ref defaults to the newest <folder>/kernel/v<N>.<R> release tag in that clone.

import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { HOST_DIR, HOST_SCAFFOLD, KERNEL_DIR, LOCK_FILE, REFERENCES, kernelTree, listFiles, projectKernel, readKernelLock } from "./lib/kernel.mjs";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const argv = process.argv.slice(2);
const flag = (name) => {
  const i = argv.indexOf(name);
  return i === -1 ? undefined : argv[i + 1];
};
const die = (msg) => {
  process.stderr.write(`kernel-sync: ${msg}\n`);
  process.exit(1);
};
const git = (cwd, args, input) => {
  const r = spawnSync("git", args, { cwd, encoding: "utf8", input });
  if (r.status !== 0) die(`git ${args.join(" ")}: ${(r.stderr || r.stdout).trim()}`);
  return r.stdout.trim();
};

const skillArg = flag("--skill") ?? die("--skill <path to the monorepo-elcodelabs folder in a git clone> is required");
const skill = resolve(skillArg);
if (!existsSync(join(skill, "scripts/scaffold.mjs"))) die(`${skill} is not monorepo-elcodelabs (no scripts/scaffold.mjs)`);
const top = git(skill, ["rev-parse", "--show-toplevel"]);
const prefix = git(skill, ["rev-parse", "--show-prefix"]); // "monorepo-elcodelabs/" or ""
const tagBase = `${prefix || "monorepo-elcodelabs/"}kernel/v`;

const ref = flag("--ref") ?? (() => {
  const tags = git(top, ["tag", "--list", `${tagBase}*`, "--sort=-version:refname"]).split("\n").filter(Boolean);
  return tags[0] ?? die(`no ${tagBase}<N>.<R> tag in ${top} — tag the kernel release there first, or pass --ref`);
})();
const commit = git(top, ["rev-parse", "--verify", `${ref}^{commit}`]);
const tag = git(top, ["tag", "--points-at", commit]).split("\n").find((t) => t.startsWith(tagBase)) ?? null;
const remote = spawnSync("git", ["remote", "get-url", "origin"], { cwd: top, encoding: "utf8" }).stdout.trim() || null;

// The skill as released: git archive of that commit, so uncommitted edits never leak in.
const work = mkdtempSync(join(tmpdir(), "kernel-sync-"));
try {
  const archive = spawnSync("git", ["archive", "--format=tar", commit, prefix || "."], { cwd: top, maxBuffer: 256 << 20 });
  if (archive.status !== 0) die(`git archive ${commit}: ${archive.stderr}`);
  const untar = spawnSync("tar", ["-x", "-C", work], { input: archive.stdout });
  if (untar.status !== 0) die(`tar: ${untar.stderr}`);
  const exported = join(work, prefix);
  const host = join(work, "host");
  const r = spawnSync(process.execPath, [join(exported, "scripts/scaffold.mjs"), "--name", HOST_SCAFFOLD.name, "--backend", HOST_SCAFFOLD.backend, "--dir", host, "--force"], { encoding: "utf8" });
  if (r.status !== 0) die(`scaffold failed: ${r.stderr || r.stdout}`);
  const kernel = projectKernel(host) ?? die("the scaffolded project has no kernel version (apps/api/internal/kernel/kernel.go)");

  const files = listFiles(host);
  const before = readKernelLock(ROOT);
  rmSync(join(ROOT, KERNEL_DIR), { recursive: true, force: true });
  cpSync(host, join(ROOT, HOST_DIR), { recursive: true });
  mkdirSync(join(ROOT, KERNEL_DIR, "reference"), { recursive: true });
  for (const f of REFERENCES) {
    const src = join(exported, "references", f);
    if (!existsSync(src)) die(`the skill at ${commit.slice(0, 7)} has no references/${f}`);
    cpSync(src, join(ROOT, KERNEL_DIR, "reference", f));
  }
  writeFileSync(join(ROOT, KERNEL_DIR, "README.md"), readme(kernel, tag ?? commit.slice(0, 7)));
  // Every host file must be committable here: a .gitignore rule — the library's or the host's
  // own — would silently leave it out of a clone.
  const ignored = spawnSync("git", ["check-ignore", "--no-index", "--stdin"], { cwd: ROOT, encoding: "utf8", input: files.map((f) => `${HOST_DIR}/${f}`).join("\n") }).stdout.trim();
  if (ignored) die(`these host files would be ignored by git here — fix the .gitignore rules, then sync again:\n${ignored}`);
  const lock = {
    kernel,
    source: { repository: remote, path: prefix.replace(/\/$/, "") || ".", ref, tag, commit },
    scaffold: HOST_SCAFFOLD,
    files: kernelTree(ROOT),
  };
  writeFileSync(join(ROOT, LOCK_FILE), JSON.stringify(lock, null, 2) + "\n");

  const changed = before ? Object.keys({ ...before.files, ...lock.files }).filter((f) => before.files[f] !== lock.files[f]).length : files.length;
  process.stdout.write(`✓ kernel ${kernel} pinned from ${tag ?? ref} (${commit.slice(0, 7)}): ${files.length} files, ${changed} changed\n`);
  if (!tag) process.stdout.write(`  ! ${commit.slice(0, 7)} has no ${tagBase}${kernel}.<R> tag — commit in the skill (its post-commit hook tags releases) and sync again before committing\n`);
  if (before && before.kernel !== kernel) process.stdout.write(`  kernel ${before.kernel} → ${kernel}: modules may now set requires_kernel ${kernel}\n`);
  process.stdout.write(`\nNext:\n  npm run testbed\n  git add kernel && git commit -m "kernel: ${kernel} dari ${tag ?? commit.slice(0, 7)}"   # kernel/ only\n`);
} finally {
  rmSync(work, { recursive: true, force: true });
}

function readme(kernel, from) {
  return `# Kernel ${kernel} (pinned)

The kernel every module in this library builds on, pinned from monorepo-elcodelabs ${from}.
**Read-only**: nothing here is edited by hand — \`npm run check\` refuses any change that
kernel.lock.json does not record, and a kernel update arrives only through
\`scripts/kernel-sync.mjs\`, in a commit of its own.

- \`host/\` — a project scaffolded at this kernel: what modules install into. The testbed and
  \`npm run sandbox\` start from a copy of it. The kernel itself is
  \`host/apps/api/internal/kernel/\` and \`host/apps/api/internal/shared/\` (Go doc comments are
  its API reference), and \`host/apps/web/src/shared/\` for the web.
- \`reference/\` — the kernel contract and architecture, as the skill states them:
  \`backend-go.md\` (kernel.Deps, module contract, kernel versions), \`backend-modular-monolith.md\`
  (modules never import each other), \`frontend.md\` (FrontendModule, slots, tokens),
  \`api-standard.md\` (/api/v1 conventions). Paths there are the project's (\`apps/api/…\`).
- \`kernel.lock.json\` — where this came from and every file's hash.

Need something the kernel does not offer? Build it inside the module, or behind a port. If it
cannot live in a module folder and many modules need it, open a "Kernel request" issue.
`;
}

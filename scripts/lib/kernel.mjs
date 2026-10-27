// The kernel this library builds on, pinned. From one released monorepo-elcodelabs:
//   kernel/host/       a project scaffolded at that kernel — what every module installs into
//   kernel/reference/  the skill's references for the kernel contract and the architecture
//   kernel/README.md   what this folder is
// and kernel/kernel.lock.json, which records where they came from and every file's hash.
// Modules are developed and tested against it, so the library needs nothing outside this
// repository. Only scripts/kernel-sync.mjs writes kernel/.

import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { cpSync, existsSync, readFileSync, readdirSync } from "node:fs";
import { join, relative } from "node:path";

export const KERNEL_DIR = "kernel";
export const HOST_DIR = "kernel/host";
export const LOCK_FILE = "kernel/kernel.lock.json";
// The skill references copied next to the host: what a module author needs to read.
export const REFERENCES = ["backend-go.md", "backend-modular-monolith.md", "frontend.md", "api-standard.md"];
// What every testbed project and sandbox is scaffolded as — kernel-sync uses the same.
export const HOST_SCAFFOLD = { name: "Testbed App", backend: "go" };

const sha256 = (buf) => createHash("sha256").update(buf).digest("hex");

/** Every file under dir (dotfiles included), as sorted relative paths with forward slashes. */
export function listFiles(dir) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { recursive: true, withFileTypes: true })
    .filter((e) => e.isFile())
    .map((e) => relative(dir, join(e.parentPath ?? e.path, e.name)).split("\\").join("/"))
    .filter((f) => !f.split("/").includes(".DS_Store"))
    .sort();
}

/** { path: sha256 } for every file under dir. */
export function hashTree(dir) {
  return Object.fromEntries(listFiles(dir).map((f) => [f, sha256(readFileSync(join(dir, f)))]));
}

/** The kernel version a scaffolded project carries (const Version in kernel.go). */
export function projectKernel(dir) {
  const file = join(dir, "apps/api/internal/kernel/kernel.go");
  if (!existsSync(file)) return undefined;
  const m = readFileSync(file, "utf8").match(/const Version = (\d+)/);
  return m ? Number(m[1]) : undefined;
}

export function readKernelLock(root) {
  const file = join(root, LOCK_FILE);
  return existsSync(file) ? JSON.parse(readFileSync(file, "utf8")) : undefined;
}

/** { path: sha256 } for everything under kernel/ except the lock itself. */
export function kernelTree(root) {
  const tree = hashTree(join(root, KERNEL_DIR));
  delete tree["kernel.lock.json"];
  return tree;
}

/**
 * Checks the pinned kernel: the lock exists, kernel/ is exactly what it records (no file
 * edited, added, or removed), and no module asks for a newer kernel than the one pinned.
 */
export function checkKernel(root, modules, problem) {
  const lock = readKernelLock(root);
  if (!lock) {
    problem("kernel", `pinned-kernel: ${LOCK_FILE} is missing — restore it with git checkout -- kernel (only the maintainer pins a new kernel, with scripts/kernel-sync.mjs)`);
    return;
  }
  const host = join(root, HOST_DIR);
  const now = kernelTree(root);
  const want = lock.files ?? {};
  const edited = Object.keys(want).filter((f) => now[f] !== undefined && now[f] !== want[f]);
  const missing = Object.keys(want).filter((f) => now[f] === undefined);
  const extra = Object.keys(now).filter((f) => want[f] === undefined);
  const list = (xs) => xs.slice(0, 5).join(", ") + (xs.length > 5 ? `, … (${xs.length})` : "");
  const why = "the kernel is changed only in monorepo-elcodelabs and brought here by kernel-sync — open a \"Kernel request\" issue";
  if (edited.length) problem("kernel", `pinned-kernel: kernel/ differs from ${LOCK_FILE} in ${list(edited)} — ${why}`);
  if (missing.length) problem("kernel", `pinned-kernel: kernel/ is missing ${list(missing)} — restore it (git checkout -- kernel)`);
  if (extra.length) problem("kernel", `pinned-kernel: kernel/ has files the lock does not: ${list(extra)} — ${why}`);
  const version = projectKernel(host);
  if (version !== lock.kernel) problem("kernel", `pinned-kernel: ${HOST_DIR} carries kernel ${version}, the lock says ${lock.kernel}`);
  for (const { id, m } of modules) {
    if (typeof m.requires_kernel === "number" && m.requires_kernel > lock.kernel) {
      problem(`modules/${id}`, `kernel-pinned: requires_kernel ${m.requires_kernel}, but this library is pinned to kernel ${lock.kernel} — a module builds only on the kernel it can be tested against; ask for the kernel change first`);
    }
  }
}

/** Copies the pinned host to dir — a fresh project, as if scaffolded at the pinned kernel. */
export function materializeHost(root, dir) {
  const problems = [];
  checkKernel(root, [], (where, msg) => problems.push(`${where}: ${msg}`));
  if (problems.length) throw new Error(`the pinned kernel is not intact:\n  ${problems.join("\n  ")}`);
  cpSync(join(root, HOST_DIR), dir, { recursive: true });
  return dir;
}

const git = (root, args) => spawnSync("git", args, { cwd: root, encoding: "utf8" });

/** Paths a commit (or the working tree, for commit "WORKTREE") changes. */
function changedPaths(root, commit) {
  if (commit === "WORKTREE") {
    const tracked = git(root, ["diff", "--name-only", "HEAD"]).stdout;
    const untracked = git(root, ["ls-files", "--others", "--exclude-standard"]).stdout;
    return [...tracked.split("\n"), ...untracked.split("\n")].filter(Boolean);
  }
  return git(root, ["diff-tree", "--no-commit-id", "--name-only", "-r", "--root", commit]).stdout.split("\n").filter(Boolean);
}

// Generated from kernel/ and modules/: the catalog names the pinned kernel, so a kernel sync
// regenerates it and it travels with the sync.
const SYNC_GENERATED = new Set(["CATALOG.md", "catalog.json", "STATUS.md"]); // and STATUS.md's log line

/**
 * A kernel change travels alone: every commit after ref (and the uncommitted work) that touches
 * kernel/ touches nothing else but the catalog it regenerates, so a kernel update is reviewed —
 * and reverted — on its own. The first commit that brings the pinned kernel in is exempt.
 */
export function checkKernelCommits(root, ref, problem) {
  const commits = git(root, ["rev-list", "--reverse", `${ref}..HEAD`]).stdout.split("\n").filter(Boolean);
  for (const c of [...commits, "WORKTREE"]) {
    const parent = c === "WORKTREE" ? "HEAD" : `${c}^`;
    if (git(root, ["cat-file", "-e", `${parent}:${LOCK_FILE}`]).status !== 0) continue; // bootstrap
    const paths = changedPaths(root, c);
    const kernel = paths.filter((p) => p.startsWith("kernel/"));
    const other = paths.filter((p) => !p.startsWith("kernel/") && !SYNC_GENERATED.has(p));
    if (kernel.length && other.length) {
      const what = c === "WORKTREE" ? "the uncommitted changes" : `commit ${c.slice(0, 7)}`;
      problem("kernel", `kernel-alone: ${what} change${c === "WORKTREE" ? "" : "s"} kernel/ together with ${other.slice(0, 3).join(", ")}${other.length > 3 ? ", …" : ""} — commit the kernel sync by itself`);
    }
  }
}

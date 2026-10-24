// Reading the library — either its working tree (installing now) or a past commit (the base
// of a 3-way upgrade). Both expose the same small interface, so rendering a module is the same
// code for "what the library gives today" and "what it gave when this was installed".

import { spawnSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join, relative } from "node:path";

function git(libRoot, args, input) {
  const r = spawnSync("git", ["-C", libRoot, ...args], { encoding: "utf8", input, maxBuffer: 64 * 1024 * 1024 });
  return r.status === 0 ? r.stdout : undefined;
}

/** The library's commit and whether its tree differs from it. commit is null outside git. */
export function libraryState(libRoot) {
  const commit = git(libRoot, ["rev-parse", "HEAD"])?.trim() ?? null;
  const dirty = commit ? (git(libRoot, ["status", "--porcelain", "--", "modules", "themes"]) ?? "").trim() !== "" : true;
  return { commit, dirty };
}

/** The library as it is on disk. */
export function workingTree(libRoot) {
  return {
    label: "working tree",
    exists: (rel) => existsSync(join(libRoot, rel)),
    read: (rel) => readFileSync(join(libRoot, rel), "utf8"),
    /** Every file under rel, as paths relative to rel. */
    list(rel) {
      const root = join(libRoot, rel);
      if (!existsSync(root)) return [];
      return readdirSync(root, { recursive: true, withFileTypes: true })
        .filter((e) => e.isFile())
        .map((e) => relative(root, join(e.parentPath ?? e.path, e.name)))
        .sort();
    },
  };
}

/** The library as it was at commit. */
export function atCommit(libRoot, commit) {
  if (git(libRoot, ["cat-file", "-e", `${commit}^{commit}`]) === undefined) {
    throw new Error(`commit ${commit} is not in the library at ${libRoot} — fetch it, or reinstall the module`);
  }
  const files = (git(libRoot, ["ls-tree", "-r", "--name-only", commit]) ?? "").split("\n").filter(Boolean);
  const set = new Set(files);
  return {
    label: `commit ${commit.slice(0, 7)}`,
    exists: (rel) => set.has(rel) || files.some((f) => f.startsWith(`${rel}/`)),
    read(rel) {
      const out = git(libRoot, ["show", `${commit}:${rel}`]);
      if (out === undefined) throw new Error(`${rel} is not in commit ${commit.slice(0, 7)}`);
      return out;
    },
    list: (rel) => files.filter((f) => f.startsWith(`${rel}/`)).map((f) => f.slice(rel.length + 1)).sort(),
  };
}

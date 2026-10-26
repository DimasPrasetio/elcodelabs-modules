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

/**
 * The library's commit, whether its tree differs from it, and whether the commit is on a
 * remote branch — one that is not exists on this machine only, so another machine cannot
 * upgrade from it (the release tags cover that). commit is null outside git.
 */
export function libraryState(libRoot) {
  const commit = git(libRoot, ["rev-parse", "HEAD"])?.trim() ?? null;
  const dirty = commit ? (git(libRoot, ["status", "--porcelain", "--", "modules", "themes"]) ?? "").trim() !== "" : true;
  const pushed = commit ? (git(libRoot, ["branch", "-r", "--contains", commit]) ?? "").trim() !== "" : false;
  return { commit, dirty, pushed };
}

// Files no module ever ships, even from a library outside git.
const JUNK = /(^|\/)(\.DS_Store|Thumbs\.db|desktop\.ini)$/;

/**
 * The library as it is on disk. Inside git, only files git would commit count (tracked, or new
 * and not ignored) — so .DS_Store and other ignored files never reach a project.
 */
export function workingTree(libRoot) {
  const inGit = git(libRoot, ["rev-parse", "--is-inside-work-tree"])?.trim() === "true";
  return {
    label: "working tree",
    exists: (rel) => existsSync(join(libRoot, rel)),
    read: (rel) => readFileSync(join(libRoot, rel), "utf8"),
    /** Every file under rel, as paths relative to rel. */
    list(rel) {
      const root = join(libRoot, rel);
      if (!existsSync(root)) return [];
      let files;
      if (inGit) {
        const out = git(libRoot, ["ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", rel]) ?? "";
        files = [...new Set(out.split("\0").filter(Boolean))]
          .filter((f) => existsSync(join(libRoot, f)))
          .map((f) => f.slice(rel.length + 1));
      } else {
        files = readdirSync(root, { recursive: true, withFileTypes: true })
          .filter((e) => e.isFile())
          .map((e) => relative(root, join(e.parentPath ?? e.path, e.name)));
      }
      return files.filter((f) => !JUNK.test(f)).sort();
    },
  };
}

/**
 * The library as it was at commit. When that commit is gone (never pushed from the machine
 * that installed, or history rewritten), fallbackTag — the module's <id>/v<version> release
 * tag — stands in for it: the same version, so the same files.
 */
export function atCommit(libRoot, commit, fallbackTag) {
  if (git(libRoot, ["cat-file", "-e", `${commit}^{commit}`]) === undefined) {
    const tagged = fallbackTag && git(libRoot, ["rev-parse", "--verify", "--quiet", `refs/tags/${fallbackTag}^{commit}`])?.trim();
    if (!tagged) {
      throw new Error(`commit ${commit.slice(0, 12)} is not in the library at ${libRoot}${fallbackTag ? ` and there is no tag ${fallbackTag}` : ""} — git fetch --tags in the library, or remove and add the module again`);
    }
    commit = tagged;
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

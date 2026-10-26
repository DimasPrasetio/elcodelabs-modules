#!/usr/bin/env node
// Validates every module.json and preset.json, then writes CATALOG.md (what an AI reads to
// pick modules) and catalog.json (what tooling reads). Zero dependencies.
//
// Usage: node scripts/catalog.mjs                       write the catalog
//        node scripts/catalog.mjs --check               fail if a manifest or module breaks a rule,
//                                                       or the catalog is stale
//        node scripts/catalog.mjs --check --since <ref> also check versions and migrations against
//                                                       <ref> (the pre-commit hook and CI do this)

import { spawnSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { checkKernel, checkKernelCommits, readKernelLock } from "./lib/kernel.mjs";
import { checkHistory, checkModules } from "./lib/rules.mjs";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const CHECK = process.argv.includes("--check");
// --since given empty or as the all-zero "before" of a new branch still checks history: against
// where this branch left main, else the previous commit.
const SINCE = (() => {
  if (!process.argv.includes("--since")) return undefined;
  const ref = process.argv[process.argv.indexOf("--since") + 1] ?? "";
  const ok = (r) => r && !/^0+$/.test(r) && spawnSync("git", ["rev-parse", "--verify", "--quiet", `${r}^{commit}`], { cwd: ROOT }).status === 0;
  if (ok(ref)) return ref;
  const base = spawnSync("git", ["merge-base", "HEAD", "origin/main"], { cwd: ROOT, encoding: "utf8" }).stdout?.trim();
  const fallback = ok(base) && base !== spawnSync("git", ["rev-parse", "HEAD"], { cwd: ROOT, encoding: "utf8" }).stdout.trim() ? base : "HEAD~1";
  console.warn(`--since ${JSON.stringify(ref)} is not a commit here — checking history against ${fallback}`);
  return fallback;
})();
const KINDS = ["fullstack", "backend", "web", "ui", "gokit"];
const problems = [];
const problem = (where, msg) => problems.push(`${where}: ${msg}`);

const dirs = (p) => (existsSync(p) ? readdirSync(p, { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => e.name).sort() : []);
const modules = dirs(join(ROOT, "modules")).map((id) => ({ id, m: JSON.parse(readFileSync(join(ROOT, "modules", id, "module.json"), "utf8")) }));
const presets = dirs(join(ROOT, "themes")).map((id) => ({ id, p: JSON.parse(readFileSync(join(ROOT, "themes", id, "preset.json"), "utf8")) }));
const ids = new Set(modules.map((x) => x.id));

for (const { id, m } of modules) {
  const at = `modules/${id}`;
  for (const f of ["id", "version", "kind", "title", "summary", "when_to_use", "when_not_to_use", "requires_kernel", "install"]) {
    if (m[f] === undefined) problem(at, `missing "${f}"`);
  }
  if (m.id !== id) problem(at, `id "${m.id}" must equal the folder name`);
  if (!/^[a-z][a-z0-9-]*$/.test(id)) problem(at, "id must be lowercase letters, digits, dashes");
  if (!/^\d+\.\d+\.\d+$/.test(m.version ?? "")) problem(at, "version must be semver x.y.z");
  if (!KINDS.includes(m.kind)) problem(at, `kind must be one of ${KINDS.join(", ")}`);
  if (!existsSync(join(ROOT, at, "README.md"))) problem(at, "missing README.md");
  for (const folder of Object.keys(m.install ?? {})) {
    if (!existsSync(join(ROOT, at, folder))) problem(at, `install folder "${folder}" does not exist`);
  }
  for (const kit of m.ui ?? []) {
    if (!ids.has(kit)) problem(at, `ui kit "${kit}" is not in the library`);
    else if (modules.find((x) => x.id === kit).m.kind !== "ui") problem(at, `"${kit}" is not a ui kit`);
  }
  for (const kit of m.go_kits ?? []) {
    if (!ids.has(kit)) problem(at, `Go kit "${kit}" is not in the library`);
    else if (modules.find((x) => x.id === kit).m.kind !== "gokit") problem(at, `"${kit}" is not a Go kit`);
  }
  if (m.scoped !== undefined && typeof m.scoped !== "boolean") problem(at, `"scoped" must be true or false`);
  if (m.scoped && !(m.owns_tables ?? []).length) problem(at, `"scoped": true but the module owns no tables`);
  for (const [field, keys] of [["emits", ["name", "version", "description"]], ["listens", ["event", "subscriber", "handler"]]]) {
    if (m[field] !== undefined && !Array.isArray(m[field])) problem(at, `"${field}" must be a list`);
    for (const e of m[field] ?? []) for (const k of keys) if (e[k] === undefined || e[k] === "") problem(at, `${field}: an entry is missing "${k}"`);
  }
  if ((m.emits?.length || m.listens?.length || m.scoped) && m.requires_kernel < 6) problem(at, `events and scope need kernel 6 — set "requires_kernel": 6`);
  for (const port of m.ports ?? []) {
    for (const [provider, a] of Object.entries(port.adapters ?? {})) {
      if (!ids.has(provider)) problem(at, `port ${port.name}: adapter for unknown module "${provider}"`);
      if (!existsSync(join(ROOT, at, a.file))) problem(at, `port ${port.name}: adapter file ${a.file} missing`);
    }
  }
  for (const p of m.permissions ?? []) {
    if (!p.name?.startsWith(`${id}.`)) problem(at, `permission "${p.name}" must be named ${id}.<action>`);
    if (!p.description) problem(at, `permission "${p.name}" needs a description`);
  }
  const hasApi = Boolean(m.install?.api);
  if (hasApi !== Boolean(m.register?.go)) problem(at, "a module with api/ must have register.go, and only then");
  if (["fullstack", "web"].includes(m.kind) && !m.register?.web) problem(at, `a ${m.kind} module needs register.web`);
  if (m.kind === "ui" && m.register?.go) problem(at, "a ui kit has no backend registration (at most a web provider)");
  for (const line of m.register?.go?.build ?? []) {
    for (const [, name] of line.matchAll(/\{\{port:(\w+)\}\}/g)) {
      if (!(m.ports ?? []).some((p) => p.name === name)) problem(at, `register uses undeclared port ${name}`);
    }
  }
}

// The architecture rules (layout, layers, imports, kits, colors, HTTP, tables) — and, with
// --since <ref>, what a change may not do to released modules.
checkModules(ROOT, modules, problem);
// The pinned kernel is intact and every module fits it; a kernel update travels alone.
checkKernel(ROOT, modules, problem);
if (SINCE) {
  checkHistory(ROOT, SINCE, problem);
  checkKernelCommits(ROOT, SINCE, problem);
}

for (const { id, p } of presets) {
  const at = `themes/${id}`;
  for (const f of ["id", "version", "title", "summary", "when_to_use"]) if (p[f] === undefined) problem(at, `missing "${f}"`);
  if (p.id !== id) problem(at, "id must equal the folder name");
  for (const f of ["theme.css", "fonts.ts", "DESIGN.md"]) if (!existsSync(join(ROOT, at, f))) problem(at, `missing ${f}`);
}

const uniq = [...new Set(problems)];
if (uniq.length) {
  console.error("Architecture check failed (rules: AGENTS.md):\n  - " + uniq.join("\n  - "));
  process.exit(1);
}

// --- outputs ----------------------------------------------------------------------------

const pinned = readKernelLock(ROOT)?.kernel ?? null;
const catalog = {
  kernel: pinned,
  modules: modules.map(({ m }) => ({
    id: m.id, version: m.version, kind: m.kind, title: m.title, summary: m.summary,
    when_to_use: m.when_to_use, when_not_to_use: m.when_not_to_use, stacks: m.stacks ?? [],
    requires_kernel: m.requires_kernel, provides: m.provides ?? {},
    permissions: (m.permissions ?? []).map((p) => p.name),
    ports: (m.ports ?? []).map((p) => ({ name: p.name, required: p.required, description: p.description, adapters: Object.keys(p.adapters ?? {}) })),
    ui: m.ui ?? [], go_kits: m.go_kits ?? [], owns_tables: m.owns_tables ?? [], scoped: Boolean(m.scoped),
    emits: (m.emits ?? []).map((e) => ({ name: e.name, version: e.version, description: e.description })),
    listens: (m.listens ?? []).map((l) => l.event),
  })),
  presets: presets.map(({ p }) => ({ id: p.id, version: p.version, title: p.title, summary: p.summary, when_to_use: p.when_to_use, when_not_to_use: p.when_not_to_use ?? [] })),
};

const cell = (s) => String(s).replaceAll("|", "\\|");
const md = [
  "# Catalog",
  "",
  "> Generated by `node scripts/catalog.mjs` from every `module.json` and `preset.json` — do not edit.",
  "> Read this to choose; open a module's `README.md` and `module.json` before installing it.",
  "",
  `Built and tested on **kernel ${pinned}** (pinned in [kernel/](kernel/README.md)). A project on an older kernel moves its kernel up before installing a module that needs a newer one.`,
  "",
  "## Modules",
  "",
  "| Module | Kind | Version | Kernel ≥ | What it does | Offers | Needs (ports) | Permissions | Builds on |",
  "|---|---|---|---|---|---|---|---|---|",
  ...catalog.modules.map((m) => {
    const offers = [...(m.provides.contracts ?? []), ...(m.provides.kernel ?? []).map((k) => `kernel ${k}`), ...m.emits.map((e) => `event ${e.name}`)].join(", ") || "—";
    const needs = [...m.ports.map((p) => `${p.name}${p.required ? "" : " (optional)"}${p.adapters.length ? ` ← ${p.adapters.join(", ")}` : ""}`), ...m.listens.map((e) => `hears ${e}`)].join("; ") || "—";
    const kind = m.scoped ? `${m.kind}, scoped` : m.kind;
    return `| [${m.id}](modules/${m.id}/README.md) | ${kind} | ${m.version} | ${m.requires_kernel} | ${cell(m.summary)} | ${offers} | ${needs} | ${m.permissions.join(", ") || "—"} | ${[...m.ui, ...m.go_kits].join(", ") || "—"} |`;
  }),
  "",
  "### When to use",
  "",
  ...catalog.modules.flatMap((m) => [
    `**${m.id}** — use: ${m.when_to_use.map(cell).join(" ")} Not when: ${m.when_not_to_use.map(cell).join(" ")}`,
    "",
  ]),
  "## Theme presets",
  "",
  "| Preset | Version | Look | Use for |",
  "|---|---|---|---|",
  ...catalog.presets.map((p) => `| [${p.id}](themes/${p.id}/DESIGN.md) | ${p.version} | ${cell(p.summary)} | ${p.when_to_use.map(cell).join(" ")} |`),
  "",
].join("\n");

const json = JSON.stringify(catalog, null, 2) + "\n";
if (CHECK) {
  const stale = [["CATALOG.md", md], ["catalog.json", json]].filter(([f, c]) => !existsSync(join(ROOT, f)) || readFileSync(join(ROOT, f), "utf8") !== c);
  if (stale.length) {
    console.error(`Stale: ${stale.map(([f]) => f).join(", ")} — run node scripts/catalog.mjs`);
    process.exit(1);
  }
  console.log(`Catalog up to date: ${catalog.modules.length} modules, ${catalog.presets.length} presets.`);
} else {
  writeFileSync(join(ROOT, "CATALOG.md"), md);
  writeFileSync(join(ROOT, "catalog.json"), json);
  console.log(`Wrote CATALOG.md and catalog.json: ${catalog.modules.length} modules, ${catalog.presets.length} presets.`);
}

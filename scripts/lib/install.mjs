// Core of installing, checking, and upgrading library modules and presets in a project
// scaffolded by monorepo-elcodelabs. Zero dependencies (git is used for 3-way merges).
// Used by scripts/module.mjs (the CLI) and scripts/testbed.mjs.
//
// Everything a module changes outside its own files goes between the project's elco:* markers
// (registry.go, sqlc.yaml, app/modules.ts), so installing never rewrites code the project owns.
//
// modules.lock.json records, per module: the version and library commit it came from, how
// its ports were wired, the hash of every file as the library rendered it (to tell what the
// project changed), its migrations, and its registry lines. An upgrade re-renders the module
// at the recorded commit (the base) and at today's library (theirs) and merges both into the
// project's files (ours) — so customizations survive and only real overlaps conflict.

import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { atCommit, libraryState, workingTree } from "./source.mjs";

const read = (p) => readFileSync(p, "utf8");
const readJSON = (p) => JSON.parse(read(p));
const writeJSON = (p, v) => writeFileSync(p, JSON.stringify(v, null, 2) + "\n");
const sha = (s) => createHash("sha256").update(s).digest("hex");

function writeFile(p, content) {
  mkdirSync(dirname(p), { recursive: true });
  writeFileSync(p, content);
}

// --- project facts -----------------------------------------------------------------------

function goModulePath(project) {
  const p = join(project, "apps/api/go.mod");
  return existsSync(p) ? read(p).match(/^module (\S+)/m)[1] : undefined;
}

/** The project's kernel version: the Go kernel's, else the frontend's shared/kernel.ts. */
function kernelVersion(project) {
  for (const [file, re] of [
    ["apps/api/internal/kernel/kernel.go", /const Version = (\d+)/],
    ["apps/web/src/shared/kernel.ts", /KERNEL_VERSION = (\d+)/],
  ]) {
    const p = join(project, file);
    const m = existsSync(p) && read(p).match(re);
    if (m) return Number(m[1]);
  }
  return undefined;
}

/** requires_kernel is a minimum: a newer kernel only adds to what modules may use. */
function assertKernel(project, m) {
  const kv = kernelVersion(project);
  if (m.register?.go && !existsSync(join(project, "apps/api/internal/kernel/kernel.go"))) {
    throw new Error(`${m.id} needs the Go kernel, but this project has none (apps/api/internal/kernel)`);
  }
  if ((kv ?? 1) < (m.requires_kernel ?? 1)) {
    throw new Error(`${m.id} ${m.version} needs kernel ${m.requires_kernel} or newer; this project has kernel ${kv ?? 1} — move the project's kernel up first (monorepo-elcodelabs references/backend-go.md, "Kernel versions")`);
  }
}

/** Throws unless dir looks like a monorepo-elcodelabs project. */
export function assertProject(project) {
  if (!existsSync(join(project, "apps/web/src/app/modules.ts"))) {
    throw new Error(`${project} is not a monorepo-elcodelabs project (no apps/web/src/app/modules.ts) — or it predates the module registry`);
  }
}

// --- lock --------------------------------------------------------------------------------

const lockPath = (project) => join(project, "modules.lock.json");

export function readLock(project) {
  return existsSync(lockPath(project)) ? readJSON(lockPath(project)) : { lockVersion: 1, preset: null, modules: {} };
}

function writeLock(project, lock) {
  writeJSON(lockPath(project), lock);
}

// --- manifests ---------------------------------------------------------------------------

function manifestFrom(src, id) {
  const rel = `modules/${id}/module.json`;
  if (!src.exists(rel)) throw new Error(`no module "${id}" in the library (${src.label})`);
  return JSON.parse(src.read(rel));
}

function presetFrom(src, id) {
  const rel = `themes/${id}/preset.json`;
  if (!src.exists(rel)) throw new Error(`no preset "${id}" in the library (${src.label})`);
  return JSON.parse(src.read(rel));
}

export const loadManifest = (libRoot, id) => manifestFrom(workingTree(libRoot), id);
export const loadPreset = (libRoot, id) => presetFrom(workingTree(libRoot), id);

/** Every module and preset in the library's working tree. */
export function libraryIndex(libRoot) {
  const ids = (dir, manifest) => (existsSync(join(libRoot, dir))
    ? readdirSync(join(libRoot, dir)).filter((d) => existsSync(join(libRoot, dir, d, manifest))).sort()
    : []);
  return {
    modules: ids("modules", "module.json").map((id) => loadManifest(libRoot, id)),
    presets: ids("themes", "preset.json").map((id) => loadPreset(libRoot, id)),
  };
}

// --- rendering: what a module puts into the project ---------------------------------------

/**
 * The files module m owns once installed, as project path → content: its install folders and
 * the ready-made adapters chosen for its ports.
 */
function renderFiles(src, m, goModule, ports) {
  const out = new Map();
  const put = (path, content) => out.set(path, path.endsWith(".go") && goModule ? content.replaceAll("__GO_MODULE__", goModule) : content);
  for (const [folder, target] of Object.entries(m.install ?? {})) {
    for (const rel of src.list(`modules/${m.id}/${folder}`)) put(`${target}/${rel}`, src.read(`modules/${m.id}/${folder}/${rel}`));
  }
  for (const port of m.ports ?? []) {
    const choice = ports?.[port.name];
    if (typeof choice === "string" && port.adapters?.[choice]) {
      put(adapterPath(m.id, port.name), src.read(`modules/${m.id}/${port.adapters[choice].file}`));
    }
  }
  return out;
}

const adapterPath = (id, port) => `apps/api/internal/modules/adapter_${id}_${port.toLowerCase()}.go`;

/** Migration name (NNN_<name>) → { up, down } contents. */
function renderMigrations(src, id) {
  const out = new Map();
  for (const f of src.list(`modules/${id}/migrations`)) {
    const m = f.match(/^(\d+_.+)\.(up|down)\.sql$/);
    if (!m) continue;
    const entry = out.get(m[1]) ?? {};
    entry[m[2]] = src.read(`modules/${id}/migrations/${f}`);
    out.set(m[1], entry);
  }
  return new Map([...out.entries()].sort(([a], [b]) => a.localeCompare(b)));
}

function presetFiles(src, id) {
  return new Map([
    ["apps/web/src/theme/theme.css", src.read(`themes/${id}/theme.css`)],
    ["apps/web/src/theme/fonts.ts", src.read(`themes/${id}/fonts.ts`)],
  ]);
}

/** Checks the port choices and returns each port's Go expression. */
function resolvePorts(m, ports, lock) {
  const expr = {};
  for (const port of m.ports ?? []) {
    const choice = ports?.[port.name];
    if (choice === undefined) {
      if (port.required) {
        const ready = Object.keys(port.adapters ?? {});
        throw new Error(`${m.id} needs port ${port.name} (${port.description})\n  wire it with --port ${port.name}=<provider>${ready.length ? ` (ready-made: ${ready.join(", ")})` : ""}, or --port ${port.name}=expr:<Go expression> for an adapter the project writes`);
      }
      continue;
    }
    if (typeof choice === "string") {
      const adapter = port.adapters?.[choice];
      if (!adapter) throw new Error(`${m.id}: no ready-made ${port.name} adapter for "${choice}" — write one and use --port ${port.name}=expr:<Go expression>`);
      if (!lock.modules[choice]) throw new Error(`${m.id}: port ${port.name} is wired to "${choice}", which is not installed — add it first`);
      expr[port.name] = adapter.expr;
    } else {
      expr[port.name] = choice.expr;
    }
  }
  return expr;
}

function goRegisterLines(m, portExpr) {
  if (!m.register?.go) return [];
  return [...m.register.go.build, ...m.register.go.routes, ...(m.register.go.after ?? [])].map((line) =>
    line.replace(/\{\{port:(\w+)\}\}/g, (_, name) => {
      if (!portExpr[name]) throw new Error(`${m.id}: register line needs port ${name}, which is not wired`);
      return portExpr[name];
    }));
}

// --- editing between markers ----------------------------------------------------------------

function insertBefore(file, marker, text) {
  const src = read(file);
  const i = src.indexOf(marker);
  if (i === -1) throw new Error(`${file} lost its marker "${marker}" — re-add it`);
  const lineStart = src.lastIndexOf("\n", i) + 1;
  writeFileSync(file, src.slice(0, lineStart) + text + src.slice(lineStart));
}

/** Adds line between two markers, keeping that block sorted (gofmt sorts Go imports). */
function insertSorted(file, begin, end, line) {
  const src = read(file);
  const b = src.indexOf(begin);
  const e = src.indexOf(end);
  if (b === -1 || e === -1) throw new Error(`${file} lost its markers "${begin}" / "${end}"`);
  const start = src.indexOf("\n", b) + 1;
  const stop = src.lastIndexOf("\n", e) + 1;
  const lines = src.slice(start, stop).split("\n").filter(Boolean);
  if (!lines.includes(line)) lines.push(line);
  const key = (l) => l.trim().replace(/^\w+ /, "");
  lines.sort((x, y) => key(x).localeCompare(key(y)));
  writeFileSync(file, src.slice(0, start) + lines.join("\n") + "\n" + src.slice(stop));
}

function sortedObject(o) {
  return Object.fromEntries(Object.entries(o).sort(([a], [b]) => a.localeCompare(b)));
}

function addNpmDeps(project, deps) {
  if (!deps || Object.keys(deps).length === 0) return [];
  const p = join(project, "apps/web/package.json");
  const pkg = readJSON(p);
  const added = Object.keys(deps).filter((d) => pkg.dependencies?.[d] !== deps[d]);
  pkg.dependencies = sortedObject({ ...pkg.dependencies, ...deps });
  writeJSON(p, pkg);
  return added;
}

function addEnv(project, id, env) {
  const file = join(project, ".env.example");
  if (!existsSync(file) || !(env ?? []).length) return [];
  const text = read(file);
  const missing = env.filter((e) => !new RegExp(`^${e.name}=`, "m").test(text));
  if (missing.length) {
    writeFileSync(file, text + `\n# --- ${id} module ---\n` + missing.map((e) => `# ${e.description}\n${e.name}=`).join("\n") + "\n");
  }
  return missing.map((e) => e.name);
}

function addSqlcBlock(project, id) {
  const file = join(project, "apps/api/sqlc.yaml");
  const base = `internal/modules/${id}/infrastructure`;
  if (read(file).includes(`queries: "${base}/queries"`)) return;
  insertBefore(file, "# elco:sqlc:end", [
    `  - engine: "mysql"`,
    `    schema: "migrations"`,
    `    queries: "${base}/queries"`,
    `    gen:`,
    `      go:`,
    `        package: "sqlc"`,
    `        out: "${base}/sqlc"`,
    "",
  ].join("\n"));
}

const utcStamp = (d) => d.toISOString().replace(/\D/g, "").slice(0, 14);

function parseStamp(v) {
  const m = /^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})$/.exec(v);
  if (!m) return undefined;
  const d = new Date(Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6]));
  return Number.isNaN(d.getTime()) ? undefined : d;
}

/**
 * Writes migrations with fresh timestamp versions (yyyymmddhhmmss, UTC), each a second after
 * the latest in the project or now, whichever is later. Returns name → file base.
 */
function addMigrations(project, id, migrations) {
  const dir = join(project, "apps/api/migrations");
  const latest = readdirSync(dir)
    .map((f) => f.match(/^(\d{14})_/)?.[1])
    .filter(Boolean)
    .map(parseStamp)
    .filter(Boolean)
    .reduce((a, b) => (b > a ? b : a), new Date(0));
  let t = Math.max(Date.now(), latest.getTime() + 1000);
  const written = {};
  for (const [name, { up, down }] of migrations) {
    const base = `${utcStamp(new Date(t))}_${id}_${name.replace(/^\d+_/, "")}`;
    if (up !== undefined) writeFileSync(join(dir, `${base}.up.sql`), up);
    if (down !== undefined) writeFileSync(join(dir, `${base}.down.sql`), down);
    written[name] = base;
    t += 1000;
  }
  return written;
}

function addModuleMapRow(project, m) {
  const file = join(project, "knowledge/MODULE_MAP.md");
  if (!existsSync(file) || m.kind === "ui") return;
  let map = read(file).replace(/^\| _none yet_ \|.*\n/m, "");
  if (new RegExp(`^\\| ${m.id} \\|`, "m").test(map)) return;
  const contract = [...(m.provides?.contracts ?? []), ...(m.provides?.kernel ?? []).map((k) => `kernel ${k}`)].join(", ") || "—";
  const row = `| ${m.id} | ${m.summary} | ${contract} | ${(m.owns_tables ?? []).join(", ") || "—"} | — |\n`;
  map = map.replace(/(\|---\|---\|---\|---\|---\|\n(?:\|.*\|\n)*)/, (table) => table + row);
  writeFileSync(file, map);
}

// --- add ----------------------------------------------------------------------------------

/**
 * Installs module id from the library's working tree. options.ports maps each port name to a
 * provider module id (its ready-made adapter is used) or { expr } — a Go expression for an
 * adapter the project writes itself in apps/api/internal/modules/.
 */
export function installModule(libRoot, project, id, options = {}) {
  assertProject(project);
  const src = workingTree(libRoot);
  const m = manifestFrom(src, id);
  const lock = readLock(project);
  if (lock.modules[id]) throw new Error(`${id} is already installed (${lock.modules[id].version}) — use upgrade`);

  assertKernel(project, m);
  for (const kit of m.ui ?? []) {
    if (!lock.modules[kit]) throw new Error(`${id} builds on the ui kit "${kit}" — add it first`);
  }
  const portExpr = resolvePorts(m, options.ports, lock);
  const goLines = goRegisterLines(m, portExpr);
  const goModule = goModulePath(project);

  const files = renderFiles(src, m, goModule, options.ports);
  // A path that exists may be taken over only from another installed module that still has it
  // exactly as the library rendered it (a file moved between kits); anything else is the project's.
  const takeovers = [];
  for (const path of files.keys()) {
    if (!existsSync(join(project, path)) || (m.replaces ?? []).some((r) => path.startsWith(`${r}/`))) continue;
    const owner = Object.entries(lock.modules).find(([, e]) => e.files?.[path]);
    if (owner && owner[1].files[path] === sha(read(join(project, path)))) {
      takeovers.push([owner[0], path]);
      continue;
    }
    throw new Error(`${id}: ${path} already exists in the project${owner ? ` and ${owner[0]}'s copy was changed here` : ""} — refusing to overwrite`);
  }

  // Nothing above wrote anything: a refused install leaves the project as it was.
  for (const p of m.replaces ?? []) rmSync(join(project, p), { recursive: true, force: true });
  const hashes = {};
  for (const [path, content] of files) {
    writeFile(join(project, path), content);
    hashes[path] = sha(content);
  }

  const migrations = addMigrations(project, id, renderMigrations(src, id));
  if (m.register?.go) {
    const reg = join(project, "apps/api/internal/modules/registry.go");
    insertSorted(reg, "// elco:imports:begin", "// elco:imports:end", `\t"${goModule}/${m.register.go.import}"`);
    insertBefore(reg, "// elco:modules:end", goLines.map((l) => `\t${l}\n`).join(""));
    if (src.exists(`modules/${id}/api/infrastructure/queries`)) addSqlcBlock(project, id);
  }
  if (m.register?.web) {
    const reg = join(project, "apps/web/src/app/modules.ts");
    const { import: from, export: name } = m.register.web;
    // A module replacing the scaffold's starter version is already registered under the same
    // import: keep that line.
    if (!read(reg).includes(`from "${from}"`)) {
      insertBefore(reg, "// elco:modules:imports:end", `import { ${name} } from "${from}";\n`);
      insertBefore(reg, "// elco:modules:end", `  ${name},\n`);
    }
  }
  const npm = addNpmDeps(project, m.npm_dependencies);
  const env = addEnv(project, id, m.env);
  addModuleMapRow(project, m);

  for (const [owner, path] of takeovers) delete lock.modules[owner].files[path];
  const state = libraryState(libRoot);
  lock.modules[id] = {
    version: m.version,
    commit: state.commit,
    dirty: state.dirty,
    status: "managed",
    ports: options.ports ?? {},
    files: hashes,
    migrations,
    register: { go: goLines, goImport: m.register?.go?.import ?? null, web: m.register?.web ?? null },
  };
  writeLock(project, lock);

  return {
    id,
    version: m.version,
    files: files.size,
    migrations: Object.keys(migrations).length,
    npm,
    env: (m.env ?? []).map((e) => ({ ...e, added: env.includes(e.name) })),
    ports: Object.fromEntries(Object.entries(options.ports ?? {}).map(([k, v]) => [k, typeof v === "string" ? v : `expr ${v.expr}`])),
    dirty: state.dirty,
    tookOver: takeovers.map(([owner, path]) => `${path} (from ${owner})`),
  };
}

/**
 * The ui kits id needs (per the library's current manifest) that are not installed yet,
 * deepest first — what add and upgrade install before it.
 */
export function missingKits(libRoot, project, id) {
  const lock = readLock(project);
  const order = [];
  const visit = (kit) => {
    if (lock.modules[kit] || order.includes(kit)) return;
    for (const dep of loadManifest(libRoot, kit).ui ?? []) visit(dep);
    order.push(kit);
  };
  for (const kit of loadManifest(libRoot, id).ui ?? []) visit(kit);
  return order;
}

// --- preset -------------------------------------------------------------------------------

export function applyPreset(libRoot, project, id) {
  assertProject(project);
  const src = workingTree(libRoot);
  const preset = presetFrom(src, id);
  const lock = readLock(project);
  if (lock.preset && lock.preset.id !== id) {
    throw new Error(`preset ${lock.preset.id} is applied — switching presets replaces theme.css by hand; remove "preset" from modules.lock.json first`);
  }
  if (lock.preset) throw new Error(`preset ${id} is already applied — use upgrade preset`);
  const files = presetFiles(src, id);
  const hashes = {};
  for (const [path, content] of files) {
    writeFile(join(project, path), content);
    hashes[path] = sha(content);
  }
  const npm = addNpmDeps(project, preset.npm_dependencies);
  const state = libraryState(libRoot);
  lock.preset = { id, version: preset.version, commit: state.commit, dirty: state.dirty, files: hashes };
  writeLock(project, lock);
  return { id, version: preset.version, npm, dirty: state.dirty };
}

// --- status -------------------------------------------------------------------------------

function changedFiles(project, hashes) {
  const modified = [];
  const missing = [];
  for (const [path, hash] of Object.entries(hashes ?? {})) {
    const p = join(project, path);
    if (!existsSync(p)) missing.push(path);
    else if (sha(read(p)) !== hash) modified.push(path);
  }
  return { modified, missing };
}

/** Each installed module (and the preset): versions, and what the project changed. */
export function status(libRoot, project) {
  const lock = readLock(project);
  const src = workingTree(libRoot);
  const rows = [];
  if (lock.preset) {
    let available;
    try { available = presetFrom(src, lock.preset.id).version; } catch { available = null; }
    rows.push({ id: `preset:${lock.preset.id}`, installed: lock.preset.version, available, status: "managed", ...changedFiles(project, lock.preset.files) });
  }
  for (const [id, entry] of Object.entries(lock.modules)) {
    let available;
    try { available = manifestFrom(src, id).version; } catch { available = null; }
    rows.push({ id, installed: entry.version, available, status: entry.status, ...changedFiles(project, entry.files) });
  }
  return rows;
}

// --- upgrade ------------------------------------------------------------------------------

/** git merge-file: ours ← (base → theirs). Returns { content, conflicts }. */
function merge3(ours, base, theirs, label) {
  const dir = mkdtempSync(join(tmpdir(), "elco-merge-"));
  try {
    const [o, b, t] = ["ours", "base", "theirs"].map((n) => join(dir, n));
    writeFileSync(o, ours);
    writeFileSync(b, base);
    writeFileSync(t, theirs);
    const r = spawnSync("git", ["merge-file", "-p", "-L", `${label} (project)`, "-L", "base", "-L", `${label} (library)`, o, b, t], { encoding: "utf8" });
    if (r.status < 0 || r.status === null) throw new Error(`git merge-file failed: ${r.stderr}`);
    return { content: r.stdout, conflicts: r.status };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

/** Merges base → theirs into the project's files. */
function mergeFiles(project, baseFiles, newFiles) {
  const report = { updated: [], added: [], removed: [], keptChanged: [], conflicts: [], unchanged: 0 };
  const paths = new Set([...baseFiles.keys(), ...newFiles.keys()]);
  for (const path of [...paths].sort()) {
    const full = join(project, path);
    const base = baseFiles.get(path);
    const theirs = newFiles.get(path);
    const ours = existsSync(full) ? read(full) : undefined;
    if (theirs === undefined) {
      // The library dropped this file.
      if (ours === undefined) continue;
      if (ours === base) { rmSync(full); report.removed.push(path); }
      else report.keptChanged.push(`${path} (removed in the library, kept because the project changed it)`);
      continue;
    }
    if (base === theirs) { report.unchanged++; continue; }
    if (ours === undefined || ours === base) {
      writeFile(full, theirs);
      (base === undefined ? report.added : report.updated).push(path);
      continue;
    }
    if (ours === theirs) { report.unchanged++; continue; }
    const merged = merge3(ours, base ?? "", theirs, path);
    writeFile(full, merged.content);
    (merged.conflicts > 0 ? report.conflicts : report.updated).push(path);
  }
  return report;
}

function replaceLines(file, oldLines, newLines) {
  const src = read(file);
  const oldText = oldLines.map((l) => `\t${l}\n`).join("");
  if (!oldText || !src.includes(oldText)) return false;
  writeFileSync(file, src.replace(oldText, newLines.map((l) => `\t${l}\n`).join("")));
  return true;
}

/**
 * Upgrades module id (or "preset") from its recorded commit to the library's working tree.
 * options.ports wires ports the new version adds. Conflicting files get git conflict markers.
 */
export function upgrade(libRoot, project, id, options = {}) {
  assertProject(project);
  const lock = readLock(project);
  const theirsSrc = workingTree(libRoot);
  const state = libraryState(libRoot);
  const goModule = goModulePath(project);

  if (id === "preset") {
    const entry = lock.preset;
    if (!entry) throw new Error("no preset is applied");
    if (!entry.commit) throw new Error("the preset was applied from a library outside git — its base cannot be recovered");
    const baseSrc = atCommit(libRoot, entry.commit);
    const preset = presetFrom(theirsSrc, entry.id);
    const report = mergeFiles(project, presetFiles(baseSrc, entry.id), presetFiles(theirsSrc, entry.id));
    report.npm = addNpmDeps(project, preset.npm_dependencies);
    lock.preset = { ...entry, version: preset.version, commit: state.commit, dirty: state.dirty, files: Object.fromEntries([...presetFiles(theirsSrc, entry.id)].map(([p, c]) => [p, sha(c)])) };
    writeLock(project, lock);
    return { id: `preset:${entry.id}`, from: entry.version, to: preset.version, ...report, notes: [] };
  }

  const entry = lock.modules[id];
  if (!entry) throw new Error(`${id} is not installed`);
  if (entry.status === "detached") throw new Error(`${id} is detached: the project owns it now and it is never upgraded`);
  if (!entry.commit) throw new Error(`${id} was installed from a library outside git — its base cannot be recovered`);

  const baseSrc = atCommit(libRoot, entry.commit);
  const mBase = manifestFrom(baseSrc, id);
  const mNew = manifestFrom(theirsSrc, id);
  assertKernel(project, mNew);
  for (const kit of mNew.ui ?? []) {
    if (!lock.modules[kit]) throw new Error(`${id} ${mNew.version} builds on the ui kit "${kit}" — add it first (module.mjs upgrade does)`);
  }
  const ports = { ...entry.ports, ...(options.ports ?? {}) };
  const portExpr = resolvePorts(mNew, ports, lock);
  const notes = [];

  if (entry.dirty) notes.push("installed from uncommitted library changes: the merge base is the commit before them, so expect more conflicts than usual");
  const report = mergeFiles(project, renderFiles(baseSrc, mBase, goModule, entry.ports), renderFiles(theirsSrc, mNew, goModule, ports));

  // Migrations are forward-only: new ones are added, applied ones are never rewritten.
  const baseMig = renderMigrations(baseSrc, id);
  const newMig = renderMigrations(theirsSrc, id);
  const fresh = new Map([...newMig].filter(([name]) => !entry.migrations?.[name]));
  const migrations = { ...entry.migrations, ...addMigrations(project, id, fresh) };
  for (const [name, c] of newMig) {
    const b = baseMig.get(name);
    if (b && entry.migrations?.[name] && (b.up !== c.up || b.down !== c.down)) {
      notes.push(`the library changed migration ${name}, which this project already has — not applied; ask for a new migration instead`);
    }
  }

  // Registry lines.
  const goLines = goRegisterLines(mNew, portExpr);
  if (mNew.register?.go) {
    const reg = join(project, "apps/api/internal/modules/registry.go");
    if (JSON.stringify(goLines) !== JSON.stringify(entry.register?.go ?? [])) {
      if (!replaceLines(reg, entry.register?.go ?? [], goLines)) {
        notes.push(`registry.go: ${id}'s lines changed in the library but were edited here — set them by hand to:\n${goLines.map((l) => `\t${l}`).join("\n")}`);
      }
    }
    if (theirsSrc.exists(`modules/${id}/api/infrastructure/queries`)) addSqlcBlock(project, id);
  }
  const npm = addNpmDeps(project, mNew.npm_dependencies);
  const env = addEnv(project, id, mNew.env);

  lock.modules[id] = {
    ...entry,
    version: mNew.version,
    commit: state.commit,
    dirty: state.dirty,
    ports,
    files: Object.fromEntries([...renderFiles(theirsSrc, mNew, goModule, ports)].map(([p, c]) => [p, sha(c)])),
    migrations,
    register: { go: goLines, goImport: mNew.register?.go?.import ?? null, web: mNew.register?.web ?? null },
  };
  writeLock(project, lock);
  return { id, from: entry.version, to: mNew.version, ...report, migrationsAdded: [...fresh.keys()], npm, env, notes };
}

// --- detach -------------------------------------------------------------------------------

/** Marks id as owned by the project from now on: status stops tracking it, upgrade refuses. */
export function detach(project, id) {
  const lock = readLock(project);
  if (!lock.modules[id]) throw new Error(`${id} is not installed`);
  lock.modules[id].status = "detached";
  writeLock(project, lock);
}

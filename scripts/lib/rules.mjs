// The architecture every module obeys, as checks. The rules come from the monorepo-elcodelabs
// skill (backend-modular-monolith.md, backend-go.md, frontend.md) — the skill defines the
// architecture, this file only refuses modules that break it. catalog.mjs runs these on every
// check, so a module that breaks a rule never reaches the catalog, a commit, or CI.
//
// Each message starts with the rule's name (see AGENTS.md › Rules) so a contributor — person or
// AI — can look up why it exists.

import { spawnSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const TOP_ENTRIES = new Set(["module.json", "README.md", "api", "web", "shared", "migrations", "adapters"]);
const INSTALL_TARGET = {
  api: (id) => `apps/api/internal/modules/${id}`,
  web: (id) => `apps/web/src/modules/${id}`,
  shared: () => "apps/web/src/shared",
};
const FOLDERS = {
  fullstack: { must: ["api", "web"], may: ["migrations", "adapters"] },
  backend: { must: ["api"], may: ["migrations", "adapters"] },
  web: { must: ["web"], may: ["shared"] },
  ui: { must: ["shared"], may: ["web"] },
};
const GO_LAYERS = ["contracts", "ports", "application", "domain", "infrastructure", "presentation"];
// What a layer may not import from its own module (backend-modular-monolith.md › Strict
// module structure): the domain and the public surfaces depend on nothing, use cases never on
// delivery or storage, handlers reach storage only through use cases.
const LAYER_FORBIDS = {
  domain: ["contracts", "ports", "application", "infrastructure", "presentation"],
  contracts: ["ports", "application", "domain", "infrastructure", "presentation"],
  ports: ["contracts", "application", "domain", "infrastructure", "presentation"],
  application: ["infrastructure", "presentation"],
  infrastructure: ["presentation"],
  presentation: ["infrastructure"],
};
const GO_PROJECT_ALLOWED = /^__GO_MODULE__\/internal\/(kernel|config|shared\/[a-z0-9_/]+)$/;
const GO_FORBIDDEN_DEPS = /^(gorm\.io\/|github\.com\/jmoiron\/sqlx|entgo\.io\/|xorm\.io\/|github\.com\/go-pg\/|github\.com\/uptrace\/bun)/;
const PALETTE = "red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|slate|gray|zinc|neutral|stone";
const RAW_COLOR = new RegExp(
  `\\b(?:bg|text|border|ring|outline|fill|stroke|from|via|to|shadow|divide|decoration|accent|caret|placeholder)-(?:(?:${PALETTE})-\\d{2,3}|white|black)\\b|\\[(?:#|rgb|hsl|oklch)[^\\]]*\\]|["'\`]#[0-9a-fA-F]{3,8}["'\`]|\\b(?:rgba?|hsla?)\\(`,
);
const RAW_HTTP = /\bfetch\(|\baxios\.(?:create|get|post|put|patch|delete|request|head)\(|\bXMLHttpRequest\b/;
const MIGRATION = /^(\d{3})_[a-z0-9_]+\.(up|down)\.sql$/;
const PROSE_FIELDS = new Set(["title", "summary", "when_to_use", "when_not_to_use", "version"]);

/** Every file under dir, as paths relative to it. */
export function walk(dir) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { recursive: true, withFileTypes: true })
    .filter((e) => e.isFile() && e.name !== ".DS_Store")
    .map((e) => relative(dir, join(e.parentPath ?? e.path, e.name)));
}

const goImports = (src) => {
  const out = [];
  for (const [, block] of src.matchAll(/^import\s*\(([\s\S]*?)^\)/gm)) for (const [, p] of block.matchAll(/"([^"]+)"/g)) out.push(p);
  for (const [, p] of src.matchAll(/^import\s+(?:\w+\s+)?"([^"]+)"/gm)) out.push(p);
  return out;
};
const tsImports = (src) => [...src.matchAll(/(?:from\s+|import\s*\(\s*|import\s+)["']([^"']+)["']/g)].map((m) => m[1]);

/** Resolves an @/shared/... import to the file it names, relative to apps/web/src/shared. */
function sharedCandidates(spec) {
  const rest = spec.slice("@/shared/".length);
  return [rest, `${rest}.ts`, `${rest}.tsx`, `${rest}/index.ts`, `${rest}/index.tsx`];
}

/**
 * Checks every module against the architecture. `modules` is [{ id, m }] for the whole
 * library (some rules look across modules); `problem(where, message)` records a violation.
 */
export function checkModules(root, modules, problem) {
  const byId = new Map(modules.map((x) => [x.id, x.m]));
  // Who installs each project path — two modules must never write the same file.
  const owners = new Map();
  // Which module provides each file under apps/web/src/shared.
  const sharedOwner = new Map();
  for (const { id, m } of modules) {
    for (const [folder, target] of Object.entries(m.install ?? {})) {
      for (const f of walk(join(root, "modules", id, folder))) {
        const path = `${target}/${f}`;
        if (owners.has(path)) problem(`modules/${id}`, `one-owner-per-file: ${path} is also installed by ${owners.get(path)}`);
        else owners.set(path, id);
        if (folder === "shared") sharedOwner.set(f, id);
      }
    }
  }

  for (const { id, m } of modules) {
    const at = `modules/${id}`;
    const dir = join(root, "modules", id);
    const kind = FOLDERS[m.kind];
    if (!kind) continue; // catalog.mjs reports the bad kind

    // --- layout --------------------------------------------------------------------------
    for (const e of readdirSync(dir)) {
      if (e === ".DS_Store") continue;
      if (!TOP_ENTRIES.has(e)) problem(at, `module-layout: unexpected "${e}" — a module holds only ${[...TOP_ENTRIES].join(", ")}`);
    }
    const present = [...TOP_ENTRIES].filter((e) => !e.includes(".") && existsSync(join(dir, e)));
    for (const f of kind.must) if (!present.includes(f)) problem(at, `module-layout: a ${m.kind} module needs ${f}/`);
    for (const f of present) {
      if (!kind.must.includes(f) && !kind.may.includes(f)) problem(at, `module-layout: a ${m.kind} module has no ${f}/ (${kindWhy(m.kind, f)})`);
    }
    for (const [folder, target] of Object.entries(m.install ?? {})) {
      const want = INSTALL_TARGET[folder];
      if (!want) problem(at, `install-targets: "${folder}" cannot be installed — only api, web, shared`);
      else if (target !== want(id)) problem(at, `install-targets: ${folder} must install to ${want(id)}, not ${target}`);
    }
    for (const f of ["api", "web", "shared"]) {
      if (existsSync(join(dir, f)) && !m.install?.[f]) problem(at, `install-targets: ${f}/ exists but install has no "${f}"`);
    }

    // --- backend -------------------------------------------------------------------------
    if (existsSync(join(dir, "api"))) checkGo(root, id, m, problem);
    if (existsSync(join(dir, "adapters"))) checkAdapters(root, id, m, byId, problem);
    checkMigrations(root, id, m, problem);

    // --- frontend ------------------------------------------------------------------------
    for (const folder of ["web", "shared"]) {
      for (const f of walk(join(dir, folder))) {
        if (!/\.(tsx?|css)$/.test(f)) continue;
        const src = readFileSync(join(dir, folder, f), "utf8");
        const file = `${folder}/${f}`;
        const line = (re) => src.split("\n").findIndex((l) => re.test(l)) + 1;
        if (RAW_COLOR.test(src)) problem(at, `semantic-colors: ${file}:${line(RAW_COLOR)} uses a raw color — use theme tokens (bg-primary, text-muted-foreground, …)`);
        if (!/\.css$/.test(f) && RAW_HTTP.test(src)) problem(at, `one-http-client: ${file}:${line(RAW_HTTP)} — requests go through @/shared/services/http-client`);
        for (const spec of tsImports(src)) {
          const mod = spec.match(/^@\/modules\/([a-z0-9-]+)/)?.[1];
          if (mod && (folder === "shared" || mod !== id)) {
            problem(at, folder === "shared"
              ? `domain-agnostic-shared: ${file} imports ${spec} — shared code never imports a module`
              : `no-module-imports: ${file} imports module "${mod}"`);
          }
          if (spec.startsWith("@/shared/")) {
            const owner = sharedCandidates(spec).map((c) => sharedOwner.get(c)).find(Boolean);
            if (owner && owner !== id) {
              if (byId.get(owner)?.kind !== "ui") problem(at, `declared-kits: ${file} imports ${spec}, which ${owner} installs — only ui kits are shared`);
              else if (!(m.ui ?? []).includes(owner)) problem(at, `declared-kits: ${file} imports ${spec} from ${owner} — add "${owner}" to "ui" in module.json`);
            }
          }
        }
      }
    }
    if (m.register?.web) {
      const entry = m.register.web.import?.replace(`@/modules/${id}/`, "");
      if (!m.register.web.import?.startsWith(`@/modules/${id}/`) || !["tsx", "ts"].some((x) => existsSync(join(dir, "web", `${entry}.${x}`)))) {
        problem(at, `frontend-registration: register.web.import must name a file in web/ (@/modules/${id}/…)`);
      }
    }
  }

  // Kits build on kits; the chain must end.
  const seen = new Set();
  const visit = (id, stack) => {
    if (stack.includes(id)) return problem(`modules/${id}`, `declared-kits: kit cycle ${[...stack, id].join(" → ")}`);
    if (seen.has(id)) return;
    seen.add(id);
    for (const k of byId.get(id)?.ui ?? []) visit(k, [...stack, id]);
  };
  for (const { id } of modules) visit(id, []);
}

function kindWhy(kind, folder) {
  if (kind === "ui") return "ui kits are frontend-only";
  if (kind === "web") return "web modules have no backend";
  if (kind === "backend") return "backend modules have no frontend";
  if (folder === "shared") return "only ui kits and web modules install into shared/ — put module UI in web/";
  return "";
}

function checkGo(root, id, m, problem) {
  const at = `modules/${id}`;
  const api = join(root, "modules", id, "api");
  if (!existsSync(join(api, `${id}.module.go`))) problem(at, `go-module-contract: api/${id}.module.go is missing (New, Routes, Client live there)`);
  for (const e of readdirSync(api)) {
    if (e === ".DS_Store" || e === `${id}.module.go`) continue;
    if (!statSync(join(api, e)).isDirectory() || !GO_LAYERS.includes(e)) {
      problem(at, `go-layers: api/${e} — a module's api/ holds ${id}.module.go and ${GO_LAYERS.join("/, ")}/ only`);
    }
  }
  for (const f of walk(api)) {
    if (!f.endsWith(".go")) continue;
    const src = readFileSync(join(api, f), "utf8");
    const layer = f.includes("/") ? f.split("/")[0] : null;
    for (const imp of goImports(src)) {
      const own = imp.match(new RegExp(`^__GO_MODULE__/internal/modules/${id}(?:/([a-z]+))?`));
      if (own) {
        if (layer && LAYER_FORBIDS[layer]?.includes(own[1])) problem(at, `go-layers: api/${f} (${layer}) imports ${own[1]} — ${layer} may not depend on ${own[1]}`);
        continue;
      }
      if (/^__GO_MODULE__\/internal\/modules\//.test(imp)) problem(at, `no-module-imports: api/${f} imports ${imp.replace("__GO_MODULE__/", "")} — declare a port instead`);
      else if (imp.startsWith("__GO_MODULE__/") && !GO_PROJECT_ALLOWED.test(imp)) problem(at, `kernel-only: api/${f} imports ${imp.replace("__GO_MODULE__/", "")} — a module builds on kernel, config, and shared only`);
      else if (GO_FORBIDDEN_DEPS.test(imp)) problem(at, `sqlc-not-orm: api/${f} imports ${imp} — data access is database/sql + sqlc`);
      else if (imp === "database/sql" && layer !== "infrastructure") problem(at, `go-layers: api/${f} imports database/sql — only infrastructure/ touches the database`);
    }
    if (/\bos\.(Getenv|LookupEnv|Environ)\(/.test(src)) problem(at, `config-through-kernel: api/${f} reads the environment directly — use config.Get / config.Require`);
  }
}

function checkAdapters(root, id, m, byId, problem) {
  const at = `modules/${id}`;
  const declared = new Map();
  for (const port of m.ports ?? []) for (const [provider, a] of Object.entries(port.adapters ?? {})) declared.set(a.file, provider);
  for (const f of walk(join(root, "modules", id, "adapters"))) {
    const provider = declared.get(`adapters/${f}`);
    if (!provider) {
      problem(at, `adapters-declared: adapters/${f} is not an adapter of any port in module.json`);
      continue;
    }
    const src = readFileSync(join(root, "modules", id, "adapters", f), "utf8");
    if (!/^package modules\b/m.test(src)) problem(at, `adapters-declared: adapters/${f} must be package modules (it lives in the composition root)`);
    for (const imp of goImports(src)) {
      const mm = imp.match(/^__GO_MODULE__\/internal\/modules\/([a-z0-9-]+)\/([a-z]+)$/);
      if (!mm && imp.startsWith("__GO_MODULE__/internal/modules/")) problem(at, `adapters-declared: adapters/${f} imports ${imp} — adapters see only ports and contracts`);
      if (!mm) continue;
      const [, mod, pkg] = mm;
      const ok = (mod === id && pkg === "ports") || (mod === provider && pkg === "contracts");
      if (!ok) problem(at, `adapters-declared: adapters/${f} imports ${mod}/${pkg} — an adapter joins ${id}/ports to ${provider}/contracts, nothing else`);
    }
  }
}

function checkMigrations(root, id, m, problem) {
  const at = `modules/${id}`;
  const files = walk(join(root, "modules", id, "migrations"));
  if (!files.length) return;
  const owned = new Set(m.owns_tables ?? []);
  const numbers = new Map();
  for (const f of files) {
    const mm = f.match(MIGRATION);
    if (!mm) {
      problem(at, `migrations: ${f} must be named NNN_snake_case.up.sql / .down.sql`);
      continue;
    }
    numbers.set(mm[1], [...(numbers.get(mm[1]) ?? []), mm[2]]);
    const sql = readFileSync(join(root, "modules", id, "migrations", f), "utf8").replace(/--.*$/gm, "");
    const touched = [
      ...[...sql.matchAll(/\b(?:CREATE|ALTER|DROP|RENAME)\s+TABLE\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?`?(\w+)`?/gi)].map((x) => x[1]),
      ...[...sql.matchAll(/\b(?:INSERT\s+INTO|(?<!\bON\s+)UPDATE|DELETE\s+FROM|TRUNCATE(?:\s+TABLE)?)\s+`?(\w+)`?/gi)].map((x) => x[1]),
      ...[...sql.matchAll(/\bREFERENCES\s+`?(\w+)`?/gi)].map((x) => x[1]),
    ];
    for (const t of new Set(touched)) {
      if (!owned.has(t)) problem(at, `own-tables-only: migrations/${f} touches "${t}", which is not in owns_tables — no cross-module tables or foreign keys`);
    }
  }
  const sorted = [...numbers.keys()].sort();
  sorted.forEach((n, i) => {
    if (Number(n) !== i + 1) problem(at, `migrations: numbering must run 001, 002, … without gaps (found ${n} at position ${i + 1})`);
    const kinds = numbers.get(n);
    if (!kinds.includes("up") || !kinds.includes("down")) problem(at, `migrations: ${n} needs both .up.sql and .down.sql`);
  });
  if (!FOLDERS[m.kind]?.may.includes("migrations")) problem(at, `migrations: a ${m.kind} module owns no tables`);
}

// --- history: what a change may not do to what is already released -----------------------

const git = (root, args) => spawnSync("git", args, { cwd: root, encoding: "utf8" });
const semver = (v) => (v ?? "0.0.0").split(".").map(Number);
const newer = (a, b) => {
  const [x, y] = [semver(a), semver(b)];
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] > y[i];
  return false;
};

/**
 * Compares the working tree with `ref` (a commit): a changed module or preset must raise its
 * version, a released migration never changes, and a released module or preset is never removed
 * — projects that installed them upgrade from exactly that history.
 */
export function checkHistory(root, ref, problem) {
  if (git(root, ["rev-parse", "--verify", "--quiet", `${ref}^{commit}`]).status !== 0) {
    console.warn(`history: ${ref} is not a commit here — skipping the version and migration checks`);
    return;
  }
  const diff = git(root, ["diff", "--name-status", "--no-renames", ref, "--", "modules", "themes"]).stdout;
  const untracked = git(root, ["ls-files", "--others", "--exclude-standard", "--", "modules", "themes"]).stdout;
  const changes = [
    ...diff.split("\n").filter(Boolean).map((l) => l.split("\t")),
    ...untracked.split("\n").filter(Boolean).map((p) => ["A", p]),
  ];
  const touched = new Map(); // "modules/users" → [[status, rest]]
  for (const [status, path] of changes) {
    const [top, id, ...rest] = path.split("/");
    if (!id || id === ".DS_Store" || rest.at(-1) === ".DS_Store") continue;
    const key = `${top}/${id}`;
    touched.set(key, [...(touched.get(key) ?? []), [status, rest.join("/")]]);
  }
  for (const [key, files] of touched) {
    const manifest = key.startsWith("themes/") ? "preset.json" : "module.json";
    const before = git(root, ["show", `${ref}:${key}/${manifest}`]);
    if (before.status !== 0) continue; // new since ref
    const old = JSON.parse(before.stdout);
    if (!existsSync(join(root, key, manifest))) {
      problem(key, `released-is-forever: ${key} existed at ${ref} — projects installed it; keep it (mark it deprecated in its README) instead of removing it`);
      continue;
    }
    const now = JSON.parse(readFileSync(join(root, key, manifest), "utf8"));
    for (const [status, file] of files) {
      if (file.startsWith("migrations/") && status !== "A") {
        problem(key, `forward-only-migrations: ${file} was released at ${ref} — never edit or remove it; add a new migration`);
      }
    }
    const installable = files.some(([, file]) => file !== "README.md" && file !== manifest)
      || Object.keys({ ...old, ...now }).some((k) => !PROSE_FIELDS.has(k) && JSON.stringify(old[k]) !== JSON.stringify(now[k]));
    if (installable && !newer(now.version, old.version)) {
      problem(key, `version-bump: changed since ${ref} but version is still ${now.version} — raise it (patch: fix, minor: feature, major: breaking) and note it under ## Changes`);
    }
  }
}

// The architecture every module obeys, as checks. The rules come from the monorepo-elcodelabs
// skill (backend-modular-monolith.md, backend-go.md, frontend.md) — the skill defines the
// architecture, this file only refuses modules that break it. catalog.mjs runs these on every
// check, so a module that breaks a rule never reaches the catalog, a commit, or CI.
//
// Each message starts with the rule's name (see AGENTS.md › Rules) so a contributor — person or
// AI — can look up why it exists.

import { spawnSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, normalize, relative } from "node:path";

const TOP_ENTRIES = new Set(["module.json", "README.md", "api", "web", "shared", "lib", "migrations", "adapters"]);
const INSTALL_TARGET = {
  api: (id) => `apps/api/internal/modules/${id}`,
  web: (id) => `apps/web/src/modules/${id}`,
  shared: () => "apps/web/src/shared",
  lib: (id) => `apps/api/internal/shared/${id}`,
};
const FOLDERS = {
  fullstack: { must: ["api", "web"], may: ["migrations", "adapters"] },
  backend: { must: ["api"], may: ["migrations", "adapters"] },
  web: { must: ["web"], may: ["shared"] },
  ui: { must: ["shared"], may: ["web"] },
  // A Go kit: shared Go code (no tables, no wiring) installed into internal/shared/<id>.
  gokit: { must: ["lib"], may: [] },
};
// Names a module publishes and subscribes under (kernel 6 d.Events).
const EVENT_NAME = /^[a-z][a-z0-9-]*(\.[a-z][a-z0-9_-]*){2,}$/;
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
// kernel/kerneltest is the kernel's test kit (kernel 6) — a module's tests may use it.
const GO_PROJECT_ALLOWED = /^__GO_MODULE__\/internal\/(kernel(\/kerneltest)?|config|shared\/[a-z0-9_/]+)$/;
const GO_FORBIDDEN_DEPS = /^(gorm\.io\/|github\.com\/jmoiron\/sqlx|entgo\.io\/|xorm\.io\/|github\.com\/go-pg\/|github\.com\/uptrace\/bun)/;
const PALETTE = "red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|slate|gray|zinc|neutral|stone";
const RAW_COLOR = new RegExp(
  `\\b(?:bg|text|border|ring|outline|fill|stroke|from|via|to|shadow|divide|decoration|accent|caret|placeholder)-(?:(?:${PALETTE})-\\d{2,3}|white|black)\\b|\\[(?:#|rgb|hsl|oklch)[^\\]]*\\]|["'\`]#[0-9a-fA-F]{3,8}["'\`]|\\b(?:rgba?|hsla?)\\(`,
);
const RAW_HTTP = /\bfetch\(|\baxios\.(?:create|get|post|put|patch|delete|request|head)\(|\bXMLHttpRequest\b/;
const MIGRATION = /^(\d{3})_[a-z0-9_]+\.(up|down)\.sql$/;
const WEB_SOURCE = /\.(tsx?|jsx?|mjs|css)$/;
// Go modules the scaffolded kernel already requires: a module may import them without declaring.
const KERNEL_GO_DEPS = ["github.com/go-sql-driver/mysql", "github.com/joho/godotenv"];
// A Go kit installs beside the kernel's own internal/shared packages, and is imported beside
// internal/kernel and internal/config — it cannot take one of their names.
const sharedPackages = (root) => {
  const dir = join(root, "kernel/host/apps/api/internal/shared");
  return new Set(["kernel", "config", ...(existsSync(dir) ? readdirSync(dir) : [])]);
};
// Go source without its comments, and — bare — with every string's contents blanked too, so
// text in a comment or a message never looks like code.
const goSource = (src) => {
  let code = "";
  let bare = "";
  for (let i = 0; i < src.length; i++) {
    const c = src[i];
    if (c === "/" && src[i + 1] === "/") {
      while (i < src.length && src[i] !== "\n") i++;
      code += "\n";
      bare += "\n";
    } else if (c === "/" && src[i + 1] === "*") {
      const end = src.indexOf("*/", i + 2);
      i = end < 0 ? src.length : end + 1;
      code += " ";
      bare += " ";
    } else if (c === '"' || c === "'" || c === "`") {
      let j = i + 1;
      while (j < src.length && src[j] !== c && !(c !== "`" && src[j] === "\n")) j += src[j] === "\\" && c !== "`" ? 2 : 1;
      code += src.slice(i, j + 1);
      bare += c + " ".repeat(Math.max(0, j - i - 1)) + c;
      i = j;
    } else {
      code += c;
      bare += c;
    }
  }
  return { code, bare };
};
// SQL with its subqueries blanked: what is left is the statement's own level (a parenthesized
// condition stays).
const topLevel = (sql) => {
  let s = sql;
  for (;;) {
    const next = s.replace(/\(([^()]*)\)/g, (_, inner) => (/\bSELECT\b/i.test(inner) ? " ".repeat(inner.length + 2) : `\u0001${inner}\u0002`));
    if (next === s) break;
    s = next;
  }
  return s.replace(/\u0001/g, "(").replace(/\u0002/g, ")");
};
// Every SELECT of a UNION compares company_id in its own WHERE (not in an ORDER BY after it).
const whereNamesCompany = (sql) =>
  topLevel(sql).split(/\bUNION\b(?:\s+ALL\b)?/i).every((part) => {
    const at = part.search(/\bWHERE\b/i);
    const where = at < 0 ? "" : part.slice(at).split(/\b(?:GROUP|ORDER)\s+BY\b|\bLIMIT\b|\bHAVING\b|\bFOR\s+(?:UPDATE|SHARE)\b/i)[0];
    return /\bcompany_id\b\s*(?:=|<=>|\bIN\b)/i.test(where);
  });
// The arguments of every call to .<method>( in Go code, split at the call's own commas.
function callArgs(code, method) {
  const out = [];
  const re = new RegExp(`\\.${method}\\(`, "g");
  for (let m = re.exec(code); m; m = re.exec(code)) {
    const args = [];
    let depth = 0;
    let cur = "";
    for (let i = m.index + m[0].length; i < code.length; i++) {
      const c = code[i];
      if (c === '"' || c === "`" || c === "'") {
        const end = code.indexOf(c, i + 1);
        cur += code.slice(i, end + 1);
        i = end;
      } else if ("([{".includes(c)) {
        depth++;
        cur += c;
      } else if (")]}".includes(c)) {
        if (depth === 0) {
          args.push(cur.trim());
          break;
        }
        depth--;
        cur += c;
      } else if (c === "," && depth === 0) {
        args.push(cur.trim());
        cur = "";
      } else cur += c;
    }
    out.push(args);
  }
  return out;
}
// The primary and unique keys a table ends with, through every migration in order.
function finalKeys(statements, table) {
  const keys = new Map();
  const t = escapeRe(table);
  for (const st of statements) {
    const create = st.match(new RegExp(`^\\s*CREATE TABLE(?:\\s+IF NOT EXISTS)?\\s+\`?${t}\`?\\s*\\(([\\s\\S]*)\\)`, "i"));
    const alter = st.match(new RegExp(`^\\s*ALTER TABLE\\s+\`?${t}\`?\\s+([\\s\\S]*)$`, "i"));
    const body = create?.[1] ?? alter?.[1];
    if (!body) continue;
    if (alter) {
      if (/\bDROP\s+PRIMARY\s+KEY\b/i.test(body)) keys.delete("PRIMARY");
      for (const [, n] of body.matchAll(/\bDROP\s+(?:KEY|INDEX)\s+`?(\w+)`?/gi)) keys.delete(n);
    }
    const cols = String.raw`\(((?:[^()]|\(\d+\))*)\)`; // a prefix length, name(20), inside
    for (const [, c] of body.matchAll(new RegExp(String.raw`\bPRIMARY\s+KEY\s*` + cols, "gi"))) keys.set("PRIMARY", c);
    for (const [, n, c] of body.matchAll(new RegExp(String.raw`\bUNIQUE\s+(?:KEY|INDEX)?\s*` + "`?(\\w*)`?\\s*" + cols, "gi"))) keys.set(n || c, c);
    // A column declared UNIQUE or PRIMARY KEY on its own line.
    for (const [, c] of body.matchAll(/(?:^|,|\bADD\s+(?:COLUMN\s+)?)\s*`?(\w+)`?\s+[A-Z]+[^,]*?\b(UNIQUE|PRIMARY\s+KEY)\b(?!\s*(?:KEY|INDEX)?\s*`?\w*`?\s*\()/gi)) {
      keys.set(c, c);
    }
  }
  for (const st of statements) {
    const index = st.match(new RegExp(`^\\s*CREATE\\s+UNIQUE\\s+INDEX\\s+\`?(\\w+)\`?\\s+ON\\s+\`?${t}\`?\\s*\\(([^;]*)\\)\\s*$`, "i"));
    if (index) keys.set(index[1], index[2]);
  }
  return keys;
}
const escapeRe = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
// Table names a statement reads or writes. "ON UPDATE", "KEY UPDATE", and "FOR UPDATE" are
// clauses, not tables.
const SQL_TABLE_REFS = /\b(?:FROM|JOIN|INTO|(?<!(?:ON|KEY|FOR)\s+)UPDATE|TABLE(?:\s+IF\s+(?:NOT\s+)?EXISTS)?|REFERENCES)\s+`?([A-Za-z_][A-Za-z0-9_]*)`?/gi;
const SQL_WORDS = new Set(["select", "duplicate", "delete", "update", "insert", "set", "where", "values", "lateral", "dual"]);
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
  // Who owns each table — exactly one module.
  const tableOwner = new Map();
  for (const { id, m } of modules) {
    for (const t of m.owns_tables ?? []) {
      if (tableOwner.has(t)) problem(`modules/${id}`, `own-tables-only: table "${t}" is also owned by ${tableOwner.get(t)}`);
      else tableOwner.set(t, id);
    }
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
      if (!want) problem(at, `install-targets: "${folder}" cannot be installed — only api, web, shared, lib`);
      else if (target !== want(id)) problem(at, `install-targets: ${folder} must install to ${want(id)}, not ${target}`);
    }
    for (const f of ["api", "web", "shared", "lib"]) {
      if (existsSync(join(dir, f)) && !m.install?.[f]) problem(at, `install-targets: ${f}/ exists but install has no "${f}"`);
    }

    // Modules are code: every file is UTF-8 text, so hashes, 3-way merges, and reviews work.
    for (const f of walk(dir)) {
      const buf = readFileSync(join(dir, f));
      if (buf.includes(0)) problem(at, `text-only: ${f} is binary — modules hold text only (images and fonts belong in a preset or the project)`);
    }
    if (m.register?.go && m.register.go.import !== `internal/modules/${id}`) {
      problem(at, `install-targets: register.go.import must be internal/modules/${id}`);
    }

    // --- backend -------------------------------------------------------------------------
    if (existsSync(join(dir, "api"))) checkGo(root, id, m, byId, problem);
    if (existsSync(join(dir, "lib"))) checkGoKit(root, id, m, byId, problem);
    checkKernelSeams(root, id, m, byId, problem);
    if (existsSync(join(dir, "adapters"))) checkAdapters(root, id, m, byId, problem);
    checkMigrations(root, id, m, problem);

    // --- frontend ------------------------------------------------------------------------
    for (const folder of ["web", "shared"]) {
      for (const f of walk(join(dir, folder))) {
        if (!WEB_SOURCE.test(f)) continue;
        const src = readFileSync(join(dir, folder, f), "utf8");
        const file = `${folder}/${f}`;
        const line = (re) => src.split("\n").findIndex((l) => re.test(l)) + 1;
        if (RAW_COLOR.test(src)) problem(at, `semantic-colors: ${file}:${line(RAW_COLOR)} uses a raw color — use theme tokens (bg-primary, text-muted-foreground, …)`);
        if (!/\.css$/.test(f) && RAW_HTTP.test(src)) problem(at, `one-http-client: ${file}:${line(RAW_HTTP)} — requests go through @/shared/services/http-client`);
        for (const spec of tsImports(src)) {
          // A relative import may not climb out of the folder it is installed with.
          if (spec.startsWith(".") && normalize(join(dirname(f), spec)).startsWith("..")) {
            problem(at, `no-module-imports: ${file} imports ${spec}, outside ${folder}/ — use @/shared or @/modules/${id}`);
            continue;
          }
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
    for (const k of [...(byId.get(id)?.ui ?? []), ...(byId.get(id)?.go_kits ?? [])]) visit(k, [...stack, id]);
  };
  for (const { id } of modules) visit(id, []);
}

function kindWhy(kind, folder) {
  if (kind === "ui") return "ui kits are frontend-only";
  if (kind === "web") return "web modules have no backend";
  if (kind === "gokit") return "a Go kit is shared Go code only — tables and wiring belong in a module";
  if (kind === "backend") return "backend modules have no frontend";
  if (folder === "shared") return "only ui kits and web modules install into shared/ — put module UI in web/";
  return "";
}

/** The Go kit an import names, when it names one (internal/shared/<kit>[/…]). */
function goKitOf(imp, byId) {
  const kit = imp.match(/^__GO_MODULE__\/internal\/shared\/([a-z0-9-]+)(?:\/|$)/)?.[1];
  return kit && byId.get(kit)?.kind === "gokit" ? kit : null;
}

function checkGo(root, id, m, byId, problem) {
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
      const own = imp.match(new RegExp(`^__GO_MODULE__/internal/modules/${escapeRe(id)}(?:/([a-z]+)(?:/.*)?)?$`));
      if (own) {
        if (layer && LAYER_FORBIDS[layer]?.includes(own[1])) problem(at, `go-layers: api/${f} (${layer}) imports ${own[1]} — ${layer} may not depend on ${own[1]}`);
        continue;
      }
      const kit = goKitOf(imp, byId);
      if (kit && !(m.go_kits ?? []).includes(kit)) problem(at, `declared-go-kits: api/${f} imports the Go kit ${kit} — add "${kit}" to "go_kits" in module.json`);
      if (/^__GO_MODULE__\/internal\/modules\//.test(imp)) problem(at, `no-module-imports: api/${f} imports ${imp.replace("__GO_MODULE__/", "")} — declare a port instead`);
      else if (imp.startsWith("__GO_MODULE__/") && !GO_PROJECT_ALLOWED.test(imp)) problem(at, `kernel-only: api/${f} imports ${imp.replace("__GO_MODULE__/", "")} — a module builds on kernel, config, and shared only`);
      else if (GO_FORBIDDEN_DEPS.test(imp)) problem(at, `sqlc-not-orm: api/${f} imports ${imp} — data access is database/sql + sqlc`);
      else if (imp === "database/sql" && layer !== "infrastructure") problem(at, `go-layers: api/${f} imports database/sql — only infrastructure/ touches the database`);
      else if (imp.split("/")[0].includes(".") && !imp.startsWith("__GO_MODULE__/")) {
        const declared = [...KERNEL_GO_DEPS, ...Object.keys(m.go_dependencies ?? {})].some((d) => imp === d || imp.startsWith(`${d}/`));
        if (!declared) problem(at, `declared-go-deps: api/${f} imports ${imp} — add its module to "go_dependencies" in module.json with the version it was tested with`);
      }
    }
    if (/\bos\.(Getenv|LookupEnv|Environ)\(/.test(src)) problem(at, `config-through-kernel: api/${f} reads the environment directly — use config.Get / config.Require`);
  }
  // Queries touch only the module's own tables — sqlc sees every table in the project, so this
  // is the line that keeps a JOIN onto another module's table out.
  const owned = new Set(m.owns_tables ?? []);
  for (const f of walk(join(api, "infrastructure", "queries"))) {
    if (!f.endsWith(".sql")) continue;
    const sql = readFileSync(join(api, "infrastructure", "queries", f), "utf8").replace(/--.*$/gm, "");
    for (const [, t] of sql.matchAll(SQL_TABLE_REFS)) {
      if (SQL_WORDS.has(t.toLowerCase()) || owned.has(t)) continue;
      problem(at, `own-tables-only: infrastructure/queries/${f} uses "${t}", which is not in owns_tables — ask its owner through a port`);
    }
  }
}

/** A Go kit: pure Go code — the standard library, its declared dependencies, other Go kits. */
function checkGoKit(root, id, m, byId, problem) {
  const at = `modules/${id}`;
  const pkg = id.replace(/-/g, "");
  if (sharedPackages(root).has(pkg)) problem(at, `gokit-pure: "${id}" is a package the kernel's internal/shared already has — name the kit differently`);
  for (const f of walk(join(root, "modules", id, "lib"))) {
    if (!f.endsWith(".go")) continue;
    const src = readFileSync(join(root, "modules", id, "lib", f), "utf8");
    if (!f.includes("/") && !new RegExp(`^package ${pkg}(_test)?\\b`, "m").test(src)) problem(at, `gokit-pure: lib/${f} must be package ${pkg}`);
    for (const imp of goImports(src)) {
      if (imp.startsWith("__GO_MODULE__/")) {
        const kit = goKitOf(imp, byId);
        if (!kit) problem(at, `gokit-pure: lib/${f} imports ${imp.replace("__GO_MODULE__/", "")} — a Go kit builds on the standard library and other Go kits only`);
        else if (!(m.go_kits ?? []).includes(kit)) problem(at, `declared-go-kits: lib/${f} imports the Go kit ${kit} — add "${kit}" to "go_kits"`);
      } else if (imp.split("/")[0].includes(".")) {
        const declared = [...KERNEL_GO_DEPS, ...Object.keys(m.go_dependencies ?? {})].some((d) => imp === d || imp.startsWith(`${d}/`));
        if (!declared) problem(at, `declared-go-deps: lib/${f} imports ${imp} — add it to "go_dependencies"`);
      }
    }
    if (/\bos\.(Getenv|LookupEnv|Environ)\(/.test(src)) problem(at, `gokit-pure: lib/${f} reads the environment — a kit takes its settings from the module that calls it`);
  }
}

/**
 * Kernel 6 seams: the kernel's tables are its own; a module on kernel 6 works inside d.Tx;
 * scoped data is filtered by company explicitly; events are declared where they are published
 * or heard.
 */
function checkKernelSeams(root, id, m, byId, problem) {
  const at = `modules/${id}`;
  const api = join(root, "modules", id, "api");
  for (const t of m.owns_tables ?? []) {
    if (t.startsWith("kernel_")) problem(at, `kernel-tables: "${t}" — tables named kernel_* belong to the kernel`);
  }
  const goFiles = walk(api).filter((f) => f.endsWith(".go")).map((f) => [f, goSource(readFileSync(join(api, f), "utf8"))]);
  if ((m.requires_kernel ?? 0) >= 6) {
    for (const [f, { code, bare }] of goFiles) {
      if (/\.BeginTx\(|\.Begin\(\)/.test(bare)) problem(at, `tx-aware: api/${f} opens its own transaction — use d.Tx.Do, so a caller's transaction is joined`);
      if (f.endsWith("_test.go")) continue; // tests may build fakes on the pool
      if (/\b\w+\.DB\b/.test(bare)) problem(at, `tx-aware: api/${f} uses the kernel's DB — reach the database through d.Tx (d.Tx.Conn(ctx)), so a caller's transaction is joined`);
      const sqlPkg = ["sql", ...[...code.matchAll(/\b(\w+)\s+"database\/sql"/g)].map((x) => x[1])];
      if (new RegExp(`\\b(?:${sqlPkg.join("|")})\\.Open(?:DB)?\\(`).test(bare)) problem(at, `tx-aware: api/${f} opens its own pool — the app has one, reached through d.Tx`);
      const sqlcNames = ["sqlc", ...[...code.matchAll(/\b(\w+)\s+"__GO_MODULE__\/[^"]*\/sqlc"/g)].map((x) => x[1])];
      // Function by function: a variable holding d.Tx.Conn(ctx) is the transaction too.
      for (const fn of bare.split(/^func\b/m)) {
        // A conn taken before a d.Tx.Do is the pool, not the transaction Do starts.
        const firstDo = fn.search(/\.Do\(/);
        const conns = new Set();
        for (const x of fn.matchAll(/\b(\w+)\s*:?=\s*[\w.]*\.Conn\(\w+\)/g)) {
          if (firstDo >= 0 && x.index < firstDo && new RegExp(`\\b${x[1]}\\b`).test(fn.slice(firstDo))) {
            problem(at, `tx-aware: api/${f} takes ${x[1]} before d.Tx.Do and uses it inside — take it from the ctx Do passes in`);
          }
          conns.add(x[1]);
        }
        const onTx = (expr) => /\.Conn\(\w+\)$/.test(expr.trim()) || conns.has(expr.trim());
        for (const name of sqlcNames) {
          for (const [, arg] of fn.matchAll(new RegExp(`\\b${name}\\.New\\(([^)]*\\)?)\\)`, "g"))) {
            if (!onTx(arg)) problem(at, `tx-aware: api/${f} builds queries on ${arg.trim() || "nothing"} — build them per call with sqlc.New(tx.Conn(ctx)) so they run inside d.Tx`);
          }
        }
        for (const [call, recv] of fn.matchAll(/([\w.]*(?:\([^()]*\))?)\.(?:Exec|Query|QueryRow|Prepare)(?:Context)?\((?!\))/g)) {
          if (!onTx(recv)) problem(at, `tx-aware: api/${f} runs SQL through ${call.slice(0, -1)} — write it as a sqlc query, or run it on d.Tx.Conn(ctx)`);
        }
      }
    }
  }
  if (m.scoped) checkScoped(root, id, m, problem);
  // Events: published names are this module's own and declared; subscriptions are declared and
  // wired only in the composition root.
  const emits = new Set((m.emits ?? []).map((e) => e.name));
  for (const e of m.emits ?? []) {
    if (!EVENT_NAME.test(e.name ?? "") || !e.name.startsWith(`${id}.`)) problem(at, `events-declared: emits "${e.name}" must be ${id}.<thing>.<past-verb>`);
    if (!Number.isInteger(e.version) || e.version < 1) problem(at, `events-declared: emits "${e.name}" needs a version ≥ 1`);
  }
  const versions = new Map((m.emits ?? []).map((e) => [e.name, e.version]));
  for (const [f, { code: src }] of goFiles) {
    if (f.endsWith("_test.go")) continue; // a subscriber's test may publish what it listens to
    // Any Publish(ctx, name, version, payload) — through d.Events or whatever holds it.
    for (const args of callArgs(src, "Publish")) {
      if (args.length !== 4) continue;
      const [, arg, version] = args;
      const name = arg.match(/^"([^"]+)"$/)?.[1];
      if (!name) problem(at, `events-declared: api/${f} publishes ${arg} — name the event with a string literal, so it is checked against "emits"`);
      else if (!emits.has(name)) problem(at, `events-declared: api/${f} publishes "${name}" — declare it in "emits"`);
      else if (String(versions.get(name)) !== version) problem(at, `events-declared: api/${f} publishes "${name}" version ${version} — "emits" declares version ${versions.get(name)}`);
    }
    if (/\b[Ee]vents\.On\(|\.On\(\s*"[a-z][\w-]*\.[\w-]+\.[\w-]+"/.test(src)) problem(at, `events-declared: api/${f} subscribes in code — declare it in "listens"; the composition root wires it`);
  }
  const known = new Set([...byId.values()].flatMap((x) => (x.emits ?? []).map((e) => e.name)));
  for (const l of m.listens ?? []) {
    if (!l.subscriber?.startsWith(`${id}.`)) problem(at, `events-declared: listens subscriber "${l.subscriber}" must be ${id}.<reaction>`);
    if (!known.has(l.event)) problem(at, `events-declared: listens to "${l.event}", which no module in the library emits`);
    if (!l.handler) problem(at, `events-declared: listens to "${l.event}" without a handler (a Go expression on the module, e.g. ${id.replace(/-./g, (s) => s[1].toUpperCase())}Module.OnX)`);
  }
}

/** Every table of a scoped module carries company_id, and every query filters by it. */
function checkScoped(root, id, m, problem) {
  const at = `modules/${id}`;
  const migrations = walk(join(root, "modules", id, "migrations")).filter((f) => f.endsWith(".up.sql")).sort()
    .map((f) => readFileSync(join(root, "modules", id, "migrations", f), "utf8").replace(/--.*$/gm, "")).join("\n");
  const statements = migrations.split(";");
  for (const t of m.owns_tables ?? []) {
    const create = migrations.match(new RegExp(`CREATE TABLE(?:\\s+IF NOT EXISTS)?\\s+\`?${escapeRe(t)}\`?\\s*\\(([^;]*);`, "i"))?.[1] ?? "";
    const added = new RegExp(`ALTER TABLE\\s+\`?${escapeRe(t)}\`?[^;]*ADD\\s+(?:COLUMN\\s+)?\`?company_id\`?`, "i").test(migrations);
    if (!/\bcompany_id\b/.test(create) && !added) problem(at, `scoped-tables: table "${t}" has no company_id — a scoped module's tables belong to a company`);
    // A business key is unique per company, or an upsert reaches another company's row.
    for (const [key, cols] of finalKeys(statements, t)) {
      const names = cols.split(",").map((c) => c.replace(/[`\s]/g, "").replace(/\(\d+\)$/, "").replace(/(ASC|DESC)$/i, ""));
      if (!(names.length === 1 && names[0] === "id") && !names.includes("company_id")) {
        problem(at, `scoped-tables: table "${t}" key ${key === "PRIMARY" ? "PRIMARY" : `"${key}"`} (${names.join(", ")}) has no company_id — make it unique per company`);
      }
    }
  }
  const queries = join(root, "modules", id, "api", "infrastructure", "queries");
  for (const f of walk(queries)) {
    if (!f.endsWith(".sql")) continue;
    const blocks = readFileSync(join(queries, f), "utf8").split(/^-- name:/m).slice(1);
    for (const block of blocks) {
      const name = block.trim().split(/\s+/)[0];
      // A system job working across every company (a purge, a retention sweep) says so, and its
      // name says so wherever it is called.
      if (/^\s*--\s*all companies\b/im.test(block)) {
        if (!name.startsWith("System")) problem(at, `scoped-queries: infrastructure/queries/${f} ${name} spans every company — name it System…, so its callers see that`);
        continue;
      }
      const sql = block.replace(/--.*$/gm, "").replace(/^\s*\S+\s+:\w+/, "");
      const touches = (m.owns_tables ?? []).some((t) => new RegExp(`\\b${escapeRe(t)}\\b`).test(sql));
      if (!touches) continue;
      // In the statement's own WHERE (not a subquery's); an INSERT names it among its columns,
      // and an INSERT … SELECT filters what it copies too.
      const insert = sql.match(/^\s*(?:INSERT|REPLACE)\s+(?:IGNORE\s+)?INTO\s+\S+\s*\(([^)]*)\)/i);
      const copies = insert && /^\s*SELECT\b/i.test(sql.slice(insert.index + insert[0].length));
      const filtered = insert
        ? /\bcompany_id\b/.test(insert[1]) && (!copies || whereNamesCompany(sql.slice(insert.index + insert[0].length)))
        : whereNamesCompany(sql);
      if (!filtered) problem(at, `scoped-queries: infrastructure/queries/${f} ${name} does not ${insert ? "insert company_id" : "filter by company_id in its WHERE"} — work in the request's company explicitly, or mark a system query that spans every company with a line "-- all companies" and a System… name`);
    }
  }
}

function checkAdapters(root, id, m, byId, problem) {
  const at = `modules/${id}`;
  const declared = new Map();
  for (const port of m.ports ?? []) {
    for (const [provider, a] of Object.entries(port.adapters ?? {})) {
      declared.set(a.file, provider);
      if (a.requires !== undefined && (!/^\d+\.\d+\.\d+$/.test(a.requires) || (byId.has(provider) && newer(a.requires, byId.get(provider).version)))) {
        problem(at, `adapter-requires: the ${port.name} adapter for ${provider} requires ${provider} ${a.requires}, which is not a released version of it`);
      }
    }
  }
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
    const raw = readFileSync(join(root, "modules", id, "migrations", f), "utf8");
    raw.split("\n").forEach((ln, i) => {
      const [code, note = ""] = ln.split("--");
      if (/\b(?:FLOAT|DOUBLE|REAL)\b/i.test(code) && !/^\s*float:\s*\S/.test(note)) {
        problem(at, `exact-numbers: migrations/${f}:${i + 1} declares a FLOAT/DOUBLE/REAL column — money and quantities are DECIMAL (money kit); a column that truly needs a float ends its line with "-- float: <why>"`);
      }
    });
    const sql = raw.replace(/--.*$/gm, "");
    const touched = [
      ...[...sql.matchAll(/\b(?:CREATE|ALTER|DROP|RENAME)\s+TABLE\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?`?(\w+)`?/gi)].map((x) => x[1]),
      ...[...sql.matchAll(/\b(?:INSERT\s+INTO|(?<!\b(?:ON|KEY|FOR)\s+)UPDATE|DELETE\s+FROM|TRUNCATE(?:\s+TABLE)?)\s+`?(\w+)`?/gi)].map((x) => x[1]),
      ...[...sql.matchAll(/\bREFERENCES\s+`?(\w+)`?/gi)].map((x) => x[1]),
      ...[...sql.matchAll(/\bINDEX\s+`?\w+`?\s+ON\s+`?(\w+)`?/gi)].map((x) => x[1]),
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

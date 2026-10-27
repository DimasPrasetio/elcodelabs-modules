// Promote: turn a module a project built into a library module. Copies its code (with the
// project's Go module path turned back into __GO_MODULE__), reads what it can from the code to
// pre-fill module.json, and leaves TODOs for what only a person can say (summary, when to
// use). Refuses a module that imports another module — that has to become a port first.

import { existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { checkModules } from "./rules.mjs";

const read = (p) => readFileSync(p, "utf8");

function walk(dir) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { recursive: true, withFileTypes: true })
    .filter((e) => e.isFile())
    .map((e) => relative(dir, join(e.parentPath ?? e.path, e.name)))
    .sort();
}

const uniq = (xs) => [...new Set(xs)].sort();
const matches = (text, re) => [...text.matchAll(re)].map((m) => m[1]);

/**
 * Copies project module id into libRoot/modules/id. options.migrations lists the project's
 * migration files (paths or names under apps/api/migrations) that belong to it.
 */
export function promote(libRoot, project, id, options = {}) {
  const target = join(libRoot, "modules", id);
  if (existsSync(target)) throw new Error(`the library already has a module "${id}"`);
  const goModFile = join(project, "apps/api/go.mod");
  const goModule = existsSync(goModFile) ? read(goModFile).match(/^module (\S+)/m)[1] : undefined;
  const apiDir = join(project, "apps/api/internal/modules", id);
  const webDir = join(project, "apps/web/src/modules", id);
  const hasApi = existsSync(apiDir);
  const hasWeb = existsSync(webDir);
  if (!hasApi && !hasWeb) throw new Error(`no module "${id}" in the project (looked in apps/api/internal/modules and apps/web/src/modules)`);

  // Generated code is rebuilt in each project, never kept in the library.
  const apiFiles = walk(apiDir).filter((f) => !f.startsWith("infrastructure/sqlc/") && !f.endsWith(".gitkeep"));
  const webFiles = walk(webDir).filter((f) => !f.endsWith(".gitkeep"));

  const problems = [];
  for (const f of apiFiles.filter((f) => f.endsWith(".go"))) {
    for (const other of matches(read(join(apiDir, f)), /internal\/modules\/([a-z0-9_-]+)/g)) {
      if (other !== id) problems.push(`api/${f} imports module "${other}" — declare a port in ${id}/ports instead`);
    }
  }
  for (const f of webFiles) {
    for (const other of matches(read(join(webDir, f)), /@\/modules\/([a-z0-9_-]+)/g)) {
      if (other !== id) problems.push(`web/${f} imports module "${other}" — go through @/shared or the registry instead`);
    }
  }
  if (problems.length) throw new Error(`${id} cannot be promoted yet:\n  - ${problems.join("\n  - ")}`);

  const copy = (from, rel, to) => {
    let content = read(join(from, rel));
    // Only inside import paths: a short module path ("pos") also occurs in ordinary words
    // ("repository"), which a plain replace would corrupt.
    if (rel.endsWith(".go") && goModule) content = content.replaceAll(`"${goModule}/`, `"__GO_MODULE__/`);
    mkdirSync(dirname(join(to, rel)), { recursive: true });
    writeFileSync(join(to, rel), content);
  };
  for (const f of apiFiles) copy(apiDir, f, join(target, "api"));
  for (const f of webFiles) copy(webDir, f, join(target, "web"));

  // Migrations: renumbered NNN_<name>, without the project's version or module prefix.
  const migDir = join(project, "apps/api/migrations");
  const migs = (options.migrations ?? []).map((m) => m.split("/").pop().replace(/\.(up|down)\.sql$/, ""));
  const tables = [];
  uniq(migs).forEach((base, i) => {
    const name = `${String(i + 1).padStart(3, "0")}_${base.replace(/^\d+_/, "").replace(new RegExp(`^${id}_`), "")}`;
    for (const dir of ["up", "down"]) {
      const src = join(migDir, `${base}.${dir}.sql`);
      if (!existsSync(src)) {
        if (dir === "up") throw new Error(`migration ${base}.up.sql not found in apps/api/migrations`);
        continue;
      }
      const sql = read(src);
      if (dir === "up") tables.push(...matches(sql, /CREATE TABLE(?: IF NOT EXISTS)?\s+`?(\w+)`?/gi));
      mkdirSync(join(target, "migrations"), { recursive: true });
      writeFileSync(join(target, "migrations", `${name}.${dir}.sql`), sql);
    }
  });

  // What the code itself says.
  const apiText = apiFiles.filter((f) => f.endsWith(".go")).map((f) => read(join(apiDir, f))).join("\n");
  const contracts = apiFiles.filter((f) => f.startsWith("contracts/")).flatMap((f) => matches(read(join(apiDir, f)), /^type (\w+) interface/gm));
  const ports = apiFiles.filter((f) => f.startsWith("ports/")).flatMap((f) => matches(read(join(apiDir, f)), /^type (\w+) interface/gm));
  const env = uniq([...matches(apiText, /config\.Require\("(\w+)"\)/g), ...matches(apiText, /config\.Get\("(\w+)"/g)]);
  const required = new Set(matches(apiText, /config\.Require\("(\w+)"\)/g));
  const apiRoutes = uniq(matches(apiText, /mux\.Handle(?:Func)?\("([A-Z]+ \/api\/[^"]+)"/g));
  const webText = webFiles.map((f) => read(join(webDir, f))).join("\n");
  const kernelFile = join(project, "apps/api/internal/kernel/kernel.go");
  const kernel = existsSync(kernelFile) ? Number(read(kernelFile).match(/const Version = (\d+)/)?.[1] ?? 1) : 1;

  // Registry lines, as the project wired them.
  let register = {};
  const regFile = join(project, "apps/api/internal/modules/registry.go");
  const todo = [];
  if (hasApi && existsSync(regFile)) {
    const lines = read(regFile).split("\n").map((l) => l.replace(/^\t/, ""));
    const v = `${id.replace(/-./g, (s) => s[1].toUpperCase())}Module`;
    const at = lines.findIndex((l) => l.includes(`${v}, err := `) || l.includes(`${v} := `));
    if (at === -1) todo.push("register.go: write the build/routes lines (not found in registry.go)");
    else {
      const build = [lines[at]];
      if (lines[at + 1]?.trim() === "if err != nil {") build.push("if err != nil {", "\treturn err", "}");
      const routes = lines.filter((l) => l.trim() === `${v}.Routes(mux)`);
      const after = lines.filter((l) => l.includes(`${v}.`) && !routes.includes(l) && l !== lines[at]);
      register.go = { import: `internal/modules/${id}`, build, routes, after };
      if (ports.length) todo.push(`register.go.build: replace each port argument with {{port:<Name>}} (${ports.join(", ")})`);
    }
  }
  const modulesTs = join(project, "apps/web/src/app/modules.ts");
  if (hasWeb && existsSync(modulesTs)) {
    const name = read(modulesTs).match(new RegExp(`import \\{ (\\w+) \\} from "@/modules/${id}/module"`))?.[1];
    if (name) register.web = { import: `@/modules/${id}/module`, export: name };
    else todo.push("register.web: the module is not in app/modules.ts");
  }

  const manifest = {
    id,
    version: "0.1.0",
    kind: hasApi && hasWeb ? "fullstack" : hasApi ? "backend" : "web",
    title: "TODO",
    summary: "TODO: one line — what it does.",
    when_to_use: ["TODO"],
    when_not_to_use: ["TODO"],
    stacks: hasApi ? ["go"] : [],
    requires_kernel: kernel,
    provides: { contracts: uniq(contracts), kernel: [] },
    ports: uniq(ports).map((name) => ({ name, required: true, description: "TODO", adapters: {} })),
    ui: kitsUsed(libRoot, webText),
    owns_tables: uniq(tables),
    env: env.map((name) => ({ name, required: required.has(name), description: "TODO" })),
    routes: { api: apiRoutes, web: [] },
    npm_dependencies: {},
    install: {
      ...(hasApi ? { api: `apps/api/internal/modules/${id}` } : {}),
      ...(hasWeb ? { web: `apps/web/src/modules/${id}` } : {}),
    },
    register,
  };
  writeFileSync(join(target, "module.json"), JSON.stringify(manifest, null, 2) + "\n");
  writeFileSync(join(target, "README.md"), `# ${id}\n\nTODO: what it does, its API and screens, what it offers and needs, what it does not do.\n`);

  // The same architecture rules every library module passes (AGENTS.md). A module that breaks
  // one is not copied in half-way: the library is left as it was.
  const library = readdirSync(join(libRoot, "modules"), { withFileTypes: true })
    .filter((e) => e.isDirectory())
    .map((e) => ({ id: e.name, m: JSON.parse(read(join(libRoot, "modules", e.name, "module.json"))) }));
  const broken = [];
  checkModules(libRoot, library, (where, msg) => where === `modules/${id}` && broken.push(msg));
  if (broken.length) {
    rmSync(target, { recursive: true, force: true });
    throw new Error(`${id} cannot be promoted yet — it breaks the library's architecture rules:\n  - ${broken.join("\n  - ")}`);
  }

  todo.unshift("module.json: title, summary, when_to_use, when_not_to_use, port and env descriptions");
  todo.push("README.md", "remove anything specific to the project it came from (names, copy, business rules)",
    "a testbed scenario that installs and exercises it (until then --check refuses it: testbed-covered)",
    "node scripts/catalog.mjs, then a STATUS.md Log entry (docs/BUILDING_MODULES.md › Release)");
  return { id, path: target, files: apiFiles.length + webFiles.length, migrations: migs.length, manifest, todo };
}

/** The ui kits whose files the code imports (each @/shared/... file belongs to at most one kit). */
function kitsUsed(libRoot, code) {
  const kits = new Set();
  const modulesDir = join(libRoot, "modules");
  for (const e of readdirSync(modulesDir, { withFileTypes: true })) {
    const manifest = join(modulesDir, e.name, "module.json");
    if (!e.isDirectory() || !existsSync(manifest)) continue; // the module being promoted has none yet
    const m = JSON.parse(read(manifest));
    if (m.kind !== "ui") continue;
    for (const f of walk(join(modulesDir, e.name, "shared"))) {
      const spec = `@/shared/${f.replace(/\.(tsx?|css)$/, "").replace(/\/index$/, "")}`;
      if (code.includes(`"${spec}"`) || code.includes(`'${spec}'`)) kits.add(e.name);
    }
  }
  return [...kits].sort();
}

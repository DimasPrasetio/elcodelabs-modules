#!/usr/bin/env node
// module.mjs — install and maintain library modules in a monorepo-elcodelabs project.
//
//   node <library>/scripts/module.mjs <command> [args] [--project <dir>] [--lib <dir>]
//
// Commands
//   list                         modules and presets in the library (✓ = installed here)
//   info <id>                    a module's README and manifest summary
//   add <id>... [--port P=V]     install modules in order; V = provider module id, or
//                                expr:<Go expression> for an adapter the project writes
//   preset <id>                  apply a theme preset
//   status                       installed vs library version, files the project changed
//   upgrade <id>|preset|--all    3-way merge to the library's version (conflicts marked)
//   detach <id>                  the project owns the module from now on: never upgraded
//   promote <id> [--migration <file>]...
//                                copy a module the project built into the library
//
// --project defaults to the current directory; --lib to the library this script lives in.
// Exit codes: 0 ok · 1 error · 2 upgrade left conflicts to resolve.

import { dirname, join, resolve } from "node:path";
import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import {
  applyPreset, assertProject, detach, installModule, libraryIndex, loadManifest, readLock, status, upgrade,
} from "./lib/install.mjs";
import { promote } from "./lib/promote.mjs";

const argv = process.argv.slice(2);
const opts = { ports: {}, migrations: [], positional: [] };
for (let i = 0; i < argv.length; i++) {
  const a = argv[i];
  const value = () => {
    const v = argv[++i];
    if (v === undefined) die(`${a} needs a value`);
    return v;
  };
  if (a === "--project") opts.project = value();
  else if (a === "--lib") opts.lib = value();
  else if (a === "--json") opts.json = true;
  else if (a === "--all") opts.all = true;
  else if (a === "--migration") opts.migrations.push(value());
  else if (a === "--port") {
    const [name, v] = value().split(/=(.*)/s);
    if (!name || !v) die("--port takes <Port>=<provider> or <Port>=expr:<Go expression>");
    opts.ports[name] = v.startsWith("expr:") ? { expr: v.slice(5) } : v;
  } else if (a === "-h" || a === "--help") opts.help = true;
  else if (a.startsWith("--")) die(`unknown option ${a}`);
  else opts.positional.push(a);
}

function die(msg) {
  process.stderr.write(`module.mjs: ${msg}\n`);
  process.exit(1);
}

const LIB = resolve(opts.lib ?? join(dirname(fileURLToPath(import.meta.url)), ".."));
const PROJECT = resolve(opts.project ?? process.cwd());
const [command, ...args] = opts.positional;
const out = (s = "") => process.stdout.write(s + "\n");

if (opts.help || !command) {
  out(readFileSync(fileURLToPath(import.meta.url), "utf8").split("\n").slice(1, 21).map((l) => l.replace(/^\/\/ ?/, "")).join("\n"));
  process.exit(command ? 0 : 1);
}

function nextSteps(reports) {
  const env = reports.flatMap((r) => (r.env ?? []).filter((e) => e.required));
  const expr = reports.flatMap((r) => Object.entries(r.ports ?? {}).filter(([, v]) => v.startsWith("expr ")).map(([p, v]) => `${r.id}.${p}: ${v.slice(5)}`));
  out("\nNext:");
  out("  npm install");
  out("  cd apps/api && go mod tidy && cd ../..");
  out("  npm run sqlc:generate");
  if (reports.some((r) => r.migrations)) out("  npm run migrate:up");
  if (expr.length) out(`  write the adapters these expressions refer to, in apps/api/internal/modules/:\n${expr.map((e) => `    ${e}`).join("\n")}`);
  if (env.length) out(`  set in .env: ${[...new Set(env.map((e) => e.name))].join(", ")}`);
  out("  then build and run: npm run build:web && npm run build:api");
}

function printUpgrade(r) {
  out(`${r.id}: ${r.from} → ${r.to}`);
  const list = (label, xs) => xs?.length && out(`  ${label}:\n${xs.map((x) => `    ${x}`).join("\n")}`);
  list("updated", r.updated);
  list("added", r.added);
  list("removed", r.removed);
  list("kept (changed here)", r.keptChanged);
  list("CONFLICTS — resolve the <<<<<<< markers", r.conflicts);
  list("migrations added", r.migrationsAdded);
  list("npm dependencies", r.npm);
  list("env added to .env.example", r.env);
  list("notes", r.notes);
  if (!r.updated?.length && !r.added?.length && !r.removed?.length && !r.conflicts?.length) out("  nothing to merge");
}

try {
  switch (command) {
    case "list": {
      const { modules, presets } = libraryIndex(LIB);
      const lock = existsSync(join(PROJECT, "modules.lock.json")) ? readLock(PROJECT) : { modules: {}, preset: null };
      if (opts.json) { out(JSON.stringify({ modules, presets }, null, 2)); break; }
      out(`Library: ${LIB}\n`);
      for (const m of modules) {
        const mark = lock.modules[m.id] ? "✓" : " ";
        const ports = (m.ports ?? []).map((p) => `${p.name}${p.required ? "" : "?"}`).join(", ");
        out(`${mark} ${m.id.padEnd(14)} ${m.kind.padEnd(9)} ${m.version.padEnd(7)} ${m.summary}${ports ? `  [needs: ${ports}]` : ""}`);
      }
      out("\nPresets:");
      for (const p of presets) out(`${lock.preset?.id === p.id ? "✓" : " "} ${p.id.padEnd(14)} ${p.version.padEnd(7)} ${p.summary}`);
      break;
    }
    case "info": {
      const id = args[0] ?? die("info <id>");
      const m = loadManifest(LIB, id);
      out(readFileSync(join(LIB, "modules", id, "README.md"), "utf8"));
      out(`kind ${m.kind} · version ${m.version} · kernel ${m.requires_kernel} · builds on ${(m.ui ?? []).join(", ") || "—"}`);
      for (const p of m.ports ?? []) out(`port ${p.name}${p.required ? "" : " (optional)"}: ${p.description} — ready-made: ${Object.keys(p.adapters ?? {}).join(", ") || "none"}`);
      for (const e of m.env ?? []) out(`env ${e.name}${e.required ? " (required)" : ""}: ${e.description}`);
      break;
    }
    case "add": {
      if (!args.length) die("add <id>...");
      assertProject(PROJECT);
      const reports = [];
      for (const id of args) {
        const m = loadManifest(LIB, id);
        const ports = Object.fromEntries(Object.entries(opts.ports).filter(([name]) => (m.ports ?? []).some((p) => p.name === name)));
        const r = installModule(LIB, PROJECT, id, { ports });
        reports.push(r);
        out(`✓ ${id} ${r.version}: ${r.files} files, ${r.migrations} migration(s)${Object.keys(r.ports).length ? `, ports ${JSON.stringify(r.ports)}` : ""}`);
        if (r.dirty) out("  ⚠ installed from uncommitted library changes — commit the library so upgrades have an exact base");
      }
      nextSteps(reports);
      break;
    }
    case "preset": {
      const id = args[0] ?? die("preset <id>");
      const r = applyPreset(LIB, PROJECT, id);
      out(`✓ preset ${id} ${r.version}${r.npm.length ? ` (fonts: ${r.npm.join(", ")})` : ""}`);
      out("Next: npm install; record `Preset: " + id + "` in knowledge/DESIGN.md");
      break;
    }
    case "status": {
      const rows = status(LIB, PROJECT);
      if (opts.json) { out(JSON.stringify(rows, null, 2)); break; }
      if (!rows.length) { out("No library modules installed."); break; }
      for (const r of rows) {
        const newer = r.available && r.available !== r.installed ? `  → ${r.available} available` : "";
        out(`${r.id.padEnd(22)} ${r.installed.padEnd(8)} ${r.status}${newer}`);
        if (r.status !== "detached") {
          for (const f of r.modified) out(`    changed here: ${f}`);
          for (const f of r.missing) out(`    deleted here: ${f}`);
        }
      }
      break;
    }
    case "upgrade": {
      const lock = readLock(PROJECT);
      const ids = opts.all
        ? [...(lock.preset ? ["preset"] : []), ...Object.entries(lock.modules).filter(([, e]) => e.status !== "detached").map(([id]) => id)]
        : args;
      if (!ids.length) die("upgrade <id>|preset|--all");
      let conflicts = 0;
      for (const id of ids) {
        const r = upgrade(LIB, PROJECT, id, { ports: opts.ports });
        printUpgrade(r);
        conflicts += r.conflicts.length;
      }
      if (conflicts) {
        out(`\n${conflicts} file(s) need their conflicts resolved, then build and test.`);
        process.exit(2);
      }
      out("\nNext: npm install, go mod tidy, npm run sqlc:generate, npm run migrate:up, build and test.");
      break;
    }
    case "detach": {
      const id = args[0] ?? die("detach <id>");
      detach(PROJECT, id);
      out(`✓ ${id} is detached: the project owns it; status and upgrade leave it alone.`);
      break;
    }
    case "promote": {
      const id = args[0] ?? die("promote <id>");
      const r = promote(LIB, PROJECT, id, { migrations: opts.migrations });
      out(`✓ copied ${r.files} files${r.migrations ? ` and ${r.migrations} migration(s)` : ""} to ${r.path}`);
      out(`  read from the code: contracts ${JSON.stringify(r.manifest.provides.contracts)}, ports ${JSON.stringify(r.manifest.ports.map((p) => p.name))}, tables ${JSON.stringify(r.manifest.owns_tables)}, env ${JSON.stringify(r.manifest.env.map((e) => e.name))}`);
      out("To finish:");
      for (const t of r.todo) out(`  - ${t}`);
      break;
    }
    default:
      die(`unknown command "${command}" — see --help`);
  }
} catch (err) {
  die(err.message);
}

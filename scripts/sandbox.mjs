#!/usr/bin/env node
// sandbox — a project to develop and try modules in, made from the pinned kernel host
// (kernel/host) with nothing outside this repository: the modules asked for are installed from
// the working tree (uncommitted work included), with the modules their required ports need,
// their ui kits, and the theme preset, every port wired to a provider in the sandbox.
//
// Usage: npm run sandbox -- <module>... [--port P=V] [--preset <id> | --no-preset]
//                           [--dir <path>] [--fresh] [--no-setup]
//   --port P=V   choose the provider of port P (when several are ready-made), or expr:<Go>
//   --dir        where to make it (default .sandbox/<modules>; .sandbox/ is ignored by git)
//   --fresh      replace an existing sandbox at that path
//   --no-setup   skip npm install, go mod tidy, and sqlc generate
//
// Then: create a database, set DB_DSN (and USERS_BOOTSTRAP_EMAIL/PASSWORD when users is in) in
// <dir>/.env, npm run migrate:up, npm run dev:api and npm run dev:web. To sign in and see pages,
// include app-shell users auth (and rbac) — see docs/BUILDING_MODULES.md › Test it.
// After changing a module, make the sandbox again with --fresh: upgrade cannot base a merge on a
// module that was never committed.

import { spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, readFileSync, rmSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { applyPreset, installModule, libraryIndex, loadManifest, missingKits, readLock } from "./lib/install.mjs";
import { materializeHost, readKernelLock } from "./lib/kernel.mjs";

const LIB = join(dirname(fileURLToPath(import.meta.url)), "..");
const argv = process.argv.slice(2);
const opts = { ports: {}, ids: [] };
const die = (msg) => {
  process.stderr.write(`sandbox: ${msg}\n`);
  process.exit(1);
};
for (let i = 0; i < argv.length; i++) {
  const a = argv[i];
  const value = () => argv[++i] ?? die(`${a} needs a value`);
  if (a === "--port") {
    const [name, v] = value().split(/=(.*)/s);
    if (!name || !v) die("--port takes <Port>=<provider> or <Port>=expr:<Go expression>");
    opts.ports[name] = v.startsWith("expr:") ? { expr: v.slice(5) } : v;
  } else if (a === "--preset") opts.preset = value();
  else if (a === "--no-preset") opts.preset = null;
  else if (a === "--dir") opts.dir = value();
  else if (a === "--fresh") opts.fresh = true;
  else if (a === "--no-setup") opts.noSetup = true;
  else if (a === "-h" || a === "--help") opts.help = true;
  else if (a.startsWith("--")) die(`unknown option ${a}`);
  else opts.ids.push(a);
}
if (opts.help || !opts.ids.length) {
  // The header comment, up to the first line that is not one.
  const header = readFileSync(fileURLToPath(import.meta.url), "utf8").split("\n").slice(1);
  process.stdout.write(header.slice(0, header.findIndex((l) => !l.startsWith("//"))).map((l) => l.replace(/^\/\/ ?/, "")).join("\n") + "\n");
  process.exit(opts.help ? 0 : 1);
}

const { modules, presets } = libraryIndex(LIB);
const byId = new Map(modules.map((m) => [m.id, m]));
for (const id of opts.ids) if (!byId.has(id)) die(`no module "${id}" in modules/`);

// The set: what was asked for, plus the providers their required ports need (transitively).
// A port with several ready-made providers needs --port, unless one is already in the set.
const chosen = new Map(); // "<module>.<Port>" → provider id or { expr }
const inSet = new Set();
const want = (id) => {
  if (inSet.has(id)) return;
  inSet.add(id);
  for (const p of byId.get(id).ports ?? []) {
    const pick = opts.ports[p.name];
    if (pick) {
      chosen.set(`${id}.${p.name}`, pick);
      if (typeof pick === "string") {
        if (!byId.has(pick)) die(`--port ${p.name}=${pick}: no module "${pick}"`);
        want(pick);
      }
      continue;
    }
    if (!p.required) continue;
    const ready = Object.keys(p.adapters ?? {});
    if (!ready.length) die(`${id} needs port ${p.name} and no module provides it ready-made — pass --port ${p.name}=expr:<Go expression> and write the adapter in the sandbox`);
    const provider = ready.find((r) => inSet.has(r) || opts.ids.includes(r)) ?? (ready.length === 1 ? ready[0] : die(`${id} needs port ${p.name}: choose a provider with --port ${p.name}=<${ready.join("|")}>`));
    chosen.set(`${id}.${p.name}`, provider);
    want(provider);
  }
};
for (const id of opts.ids) want(id);
// Optional ports are wired when their ready-made provider is in the sandbox anyway.
for (const id of inSet) {
  for (const p of byId.get(id).ports ?? []) {
    if (chosen.has(`${id}.${p.name}`)) continue;
    const provider = Object.keys(p.adapters ?? {}).find((r) => inSet.has(r));
    if (provider) chosen.set(`${id}.${p.name}`, provider);
  }
}

// Providers before the modules wired to them.
const order = [];
const visit = (id, path = []) => {
  if (order.includes(id)) return;
  if (path.includes(id)) die(`ports form a cycle: ${[...path, id].join(" → ")}`);
  for (const p of byId.get(id).ports ?? []) {
    const v = chosen.get(`${id}.${p.name}`);
    if (typeof v === "string") visit(v, [...path, id]);
  }
  order.push(id);
};
for (const id of [...inSet].sort()) visit(id);

const lock = readKernelLock(LIB) ?? die("no pinned kernel (kernel/kernel.lock.json) — see kernel/README.md");
const dir = resolve(opts.dir ?? join(LIB, ".sandbox", opts.ids.join("+")));
if (existsSync(dir)) {
  if (!opts.fresh) die(`${relative(process.cwd(), dir) || dir} exists — work there, or pass --fresh to start over`);
  rmSync(dir, { recursive: true, force: true });
}
mkdirSync(dirname(dir), { recursive: true });
materializeHost(LIB, dir);
copyFileSync(join(dir, ".env.example"), join(dir, ".env"));
process.stdout.write(`✓ project from the pinned kernel ${lock.kernel} host → ${dir}\n`);

const preset = opts.preset === undefined ? (presets.length === 1 ? presets[0].id : undefined) : opts.preset;
if (preset) {
  applyPreset(LIB, dir, preset);
  process.stdout.write(`✓ preset ${preset}\n`);
}
for (const id of order) {
  for (const kit of missingKits(LIB, dir, id)) {
    installModule(LIB, dir, kit, {});
    process.stdout.write(`✓ ${kit} (ui kit)\n`);
  }
  if (readLock(dir).modules[id]) continue;
  const ports = Object.fromEntries((loadManifest(LIB, id).ports ?? []).filter((p) => chosen.has(`${id}.${p.name}`)).map((p) => [p.name, chosen.get(`${id}.${p.name}`)]));
  installModule(LIB, dir, id, { ports });
  const wired = Object.entries(ports).map(([p, v]) => `${p} ← ${typeof v === "string" ? v : v.expr}`).join(", ");
  process.stdout.write(`✓ ${id}${wired ? ` (${wired})` : ""}${opts.ids.includes(id) ? "" : " — needed by the modules asked for"}\n`);
}
const unwired = order.flatMap((id) => (byId.get(id).ports ?? []).filter((p) => !p.required && !chosen.has(`${id}.${p.name}`)).map((p) => `${id}.${p.name} (add ${Object.keys(p.adapters ?? {}).join(" or ") || "an adapter of your own"})`));
if (unwired.length) process.stdout.write(`  optional ports left off: ${unwired.join(", ")}\n`);

if (!opts.noSetup) {
  for (const [cmd, args, cwd] of [["npm", ["install", "--no-audit", "--no-fund"], dir], ["go", ["mod", "tidy"], join(dir, "apps/api")], ["npm", ["run", "sqlc:generate"], dir]]) {
    const r = spawnSync(cmd, args, { cwd, encoding: "utf8" });
    if (r.status !== 0) die(`${cmd} ${args.join(" ")} failed in ${cwd}:\n${(r.stdout + r.stderr).trim().split("\n").slice(-20).join("\n")}`);
    process.stdout.write(`✓ ${cmd} ${args.join(" ")}\n`);
  }
}

const env = [...new Set(order.flatMap((id) => (byId.get(id).env ?? []).filter((e) => e.required).map((e) => e.name)))];
process.stdout.write(`\nNext, in ${dir}:\n`);
if (opts.noSetup) process.stdout.write("  npm install && (cd apps/api && go mod tidy) && npm run sqlc:generate\n");
process.stdout.write(`  set DB_* (a database of its own) in .env${env.length ? `, and ${env.join(", ")}` : ""}\n`);
process.stdout.write("  npm run migrate:up\n  npm run dev:api     # and in another terminal: npm run dev:web\n");

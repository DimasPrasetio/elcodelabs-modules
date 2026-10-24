#!/usr/bin/env node
// Testbed — proves the library's modules install into a freshly scaffolded project and work.
//
// Five scenarios, each in its own fresh project scaffolded with monorepo-elcodelabs (Go):
//   full        preset bingkai-navy + ui-core + app-shell + users + auth + rbac (ports ← users)
//   no-rbac     users + auth without rbac: the kernel's default "signed in = allowed"
//   auth-alone  ui-core + auth, UserStore ← an adapter the project wrote itself (no users module)
//   users-alone ui-core + users, no auth provider (guarded routes must refuse)
//   lifecycle   the module.mjs CLI: add, status, upgrade (clean merge + conflict), detach, promote
// Each: install → npm install → go mod tidy → sqlc generate → gofmt → go vet → go test →
// build:web → build:api → run the binary and call the API.
//
// The API checks of full and no-rbac need MySQL: set TESTBED_DB_DSN to a server DSN without a
// database (e.g. "root:@tcp(localhost:3306)/"). A throwaway database is created (mysql client),
// migrated with npm run migrate:up (golang-migrate, installed into .testbed/bin), and dropped.
// Without it those checks are skipped — and reported.
//
// Usage: node scripts/testbed.mjs [--skill <path to monorepo-elcodelabs>] [--only <scenario>] [--keep]
//   --skill defaults to $SKILL_DIR, then ~/.claude/skills/monorepo-elcodelabs.

import { spawn, spawnSync } from "node:child_process";
import { copyFileSync, cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { applyPreset, installModule as installOne, missingKits } from "./lib/install.mjs";

const LIB = join(dirname(fileURLToPath(import.meta.url)), "..");
const argv = process.argv.slice(2);
const flag = (name) => {
  const i = argv.indexOf(name);
  return i === -1 ? undefined : argv[i + 1];
};
const SKILL = flag("--skill") ?? process.env.SKILL_DIR ?? join(homedir(), ".claude/skills/monorepo-elcodelabs");
const ONLY = flag("--only");
const KEEP = argv.includes("--keep");
const DB_DSN = process.env.TESTBED_DB_DSN;

let failures = 0;
const skipped = [];
const log = (m) => process.stdout.write(m + "\n");
const pass = (m) => log(`  ✓ ${m}`);
const fail = (m) => { log(`  ✗ ${m}`); failures++; };
const skip = (m) => { log(`  ⚠ SKIPPED: ${m}`); skipped.push(m); };

function run(cmd, args, cwd, label = `${cmd} ${args.join(" ")}`, env) {
  const r = spawnSync(cmd, args, { cwd, encoding: "utf8", env: env ? { ...process.env, ...env } : process.env });
  if (r.status !== 0) {
    fail(`${label} exited ${r.status}\n${(r.stdout + r.stderr).trim().split("\n").slice(-40).join("\n")}`);
    return false;
  }
  pass(label);
  return true;
}

function scaffold(name) {
  mkdirSync(join(LIB, ".testbed"), { recursive: true });
  const dir = mkdtempSync(join(LIB, ".testbed", `${name}-`));
  const r = spawnSync(process.execPath, [join(SKILL, "scripts/scaffold.mjs"), "--name", "Testbed App", "--backend", "go", "--dir", dir, "--force"], { encoding: "utf8" });
  if (r.status !== 0) throw new Error(`scaffold failed: ${r.stderr}`);
  copyFileSync(join(dir, ".env.example"), join(dir, ".env"));
  return dir;
}

function build(dir) {
  const api = join(dir, "apps/api");
  return run("npm", ["install", "--no-audit", "--no-fund"], dir, "npm install")
    && run("go", ["mod", "tidy"], api, "go mod tidy")
    && run("npm", ["run", "sqlc:generate"], dir, "sqlc generate")
    && (() => {
      const r = spawnSync("gofmt", ["-l", "."], { cwd: api, encoding: "utf8" });
      if (r.stdout.trim()) { fail(`not gofmt-clean:\n${r.stdout.trim()}`); return false; }
      pass("gofmt clean");
      return true;
    })()
    && run("go", ["vet", "./..."], api, "go vet ./...")
    && run("go", ["test", "./..."], api, "go test ./... (module unit tests, ports faked)")
    && run("npm", ["run", "build:web"], dir, "build:web (tsc + vite)")
    && run("npm", ["run", "build:api"], dir, "build:api");
}

/** Installs id after the ui kits it needs — what module.mjs add does. */
function installModule(lib, dir, id, opts) {
  for (const kit of missingKits(lib, dir, id)) installOne(lib, dir, kit);
  return installOne(lib, dir, id, opts);
}

// --- running the API ---------------------------------------------------------------------

async function startApi(dir, port, env) {
  const proc = spawn(join(dir, "dist/api"), [], {
    cwd: join(dir, "apps/api"),
    env: { ...process.env, APP_PORT: String(port), JWT_SECRET: "testbed-secret-testbed-secret-testbed", ...env },
    detached: true,
  });
  let output = "";
  proc.stdout.on("data", (d) => { output += d; });
  proc.stderr.on("data", (d) => { output += d; });
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    if (proc.exitCode !== null) throw new Error(`api exited ${proc.exitCode}:\n${output}`);
    try {
      if ((await fetch(`http://localhost:${port}/api/v1/health`)).ok) return { proc, output: () => output };
    } catch {}
    await new Promise((r) => setTimeout(r, 300));
  }
  throw new Error(`api did not come up:\n${output}`);
}

function stopApi(api) {
  if (!api) return;
  try { process.kill(-api.proc.pid, "SIGTERM"); } catch {}
}

function client(port) {
  let cookie = "";
  return {
    get cookie() { return cookie; },
    forget() { cookie = ""; },
    async call(method, path, body, headers = {}) {
      const res = await fetch(`http://localhost:${port}${path}`, {
        method,
        headers: { ...(body ? { "Content-Type": "application/json" } : {}), ...(cookie ? { Cookie: cookie } : {}), ...headers },
        body: body ? JSON.stringify(body) : undefined,
      });
      const set = res.headers.get("set-cookie");
      if (set) {
        const m = set.match(/session=([^;]*)/);
        if (m) cookie = m[1] ? `session=${m[1]}` : "";
      }
      const text = await res.text();
      let json;
      try { json = JSON.parse(text); } catch { json = text; }
      return { status: res.status, body: json, setCookie: set };
    },
  };
}

function expect(label, cond, detail) {
  if (cond) pass(label);
  else fail(`${label}${detail ? ` — ${JSON.stringify(detail)}` : ""}`);
}

// --- database (full scenario) ------------------------------------------------------------

function parseDsn(dsn) {
  const m = dsn.match(/^([^:@]*)(?::([^@]*))?@tcp\(([^:)]+)(?::(\d+))?\)\/(.*)$/);
  if (!m) throw new Error(`TESTBED_DB_DSN is not a Go MySQL DSN: ${dsn}`);
  return { user: m[1], pass: m[2] ?? "", host: m[3], port: m[4] ?? "3306" };
}

function mysql(db, sqlOrFile, database) {
  const args = ["-h", db.host, "-P", db.port, "-u", db.user, ...(db.pass ? [`-p${db.pass}`] : [])];
  if (database) args.push(database);
  const input = sqlOrFile.endsWith(".sql") ? readFileSync(sqlOrFile, "utf8") : sqlOrFile;
  const r = spawnSync("mysql", args, { input, encoding: "utf8" });
  if (r.status !== 0) throw new Error(`mysql: ${r.stderr}`);
}

// golang-migrate, the project's own migration tool (npm run migrate:up → scripts/migrate.mjs),
// so migrations are tested the way production runs them. Installed into .testbed/bin once.
let migrateBin;
function migratePath() {
  if (migrateBin !== undefined) return migrateBin;
  const bin = join(LIB, ".testbed", "bin");
  if (!existsSync(join(bin, "migrate"))) {
    mkdirSync(bin, { recursive: true });
    const r = spawnSync("go", ["install", "-tags", "mysql", "github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1"], {
      encoding: "utf8", env: { ...process.env, GOBIN: bin },
    });
    if (r.status !== 0) throw new Error(`installing golang-migrate failed: ${r.stderr}`);
  }
  migrateBin = bin;
  return bin;
}

/** Creates a throwaway database, migrates it with npm run migrate:up, returns its DSN. */
function freshDatabase(dir, db, name) {
  mysql(db, `CREATE DATABASE ${name} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`);
  const dsn = `${db.user}${db.pass ? `:${db.pass}` : ""}@tcp(${db.host}:${db.port})/${name}`;
  if (!run("npm", ["run", "migrate:up"], dir, "npm run migrate:up (golang-migrate, multi-statement files)", {
    DB_DSN: dsn, PATH: `${migratePath()}:${process.env.PATH}`,
  })) throw new Error("migrations failed");
  return dsn;
}

// --- scenarios ---------------------------------------------------------------------------

async function full() {
  log("\n=== full: bingkai-navy + ui kits + app-shell + users + auth + rbac + ui-gallery ===");
  const dir = scaffold("full");
  try {
    applyPreset(LIB, dir, "bingkai-navy");
    for (const [id, opts] of [
      ["app-shell"], ["users"],
      ["auth", { ports: { UserStore: "users" } }],
      ["rbac", { ports: { UserDirectory: "users" } }],
      ["ui-gallery"],
    ]) {
      installModule(LIB, dir, id, opts);
    }
    pass("preset + 4 ui kits + app-shell, users, auth, rbac, ui-gallery installed through the markers");
    const reg = readFileSync(join(dir, "apps/api/internal/modules/registry.go"), "utf8");
    expect("registry wires auth's UserStore port to users through an adapter",
      reg.includes("auth.New(d, authUserStore{users: usersModule.Client()})") && existsSync(join(dir, "apps/api/internal/modules/adapter_auth_userstore.go")));
    for (const id of ["users", "auth", "rbac"]) {
      const src = readdirSync(join(dir, `apps/api/internal/modules/${id}`), { recursive: true })
        .filter((f) => f.endsWith(".go"))
        .map((f) => readFileSync(join(dir, `apps/api/internal/modules/${id}`, f), "utf8"))
        .join("\n");
      const others = ["users", "auth", "rbac"].filter((o) => o !== id);
      expect(`${id} imports no other module`, others.every((o) => !src.includes(`/internal/modules/${o}`)));
    }
    if (!build(dir)) return;

    if (!DB_DSN) {
      skip("full: API checks against MySQL — set TESTBED_DB_DSN");
      return;
    }
    const db = parseDsn(DB_DSN);
    const name = `elco_testbed_${Date.now()}`;
    let api;
    try {
      const dsn = freshDatabase(dir, db, name);
      const port = 18091;
      api = await startApi(dir, port, {
        DB_DSN: dsn,
        USERS_BOOTSTRAP_EMAIL: "admin@example.com",
        USERS_BOOTSTRAP_PASSWORD: "admin-pass-123",
        PUBLIC_DIR: join(dir, "apps/web/dist"),
      });
      const admin = client(port);
      expect("readyz 200 with the database", (await admin.call("GET", "/readyz")).status === 200);
      expect("users list without a session → 401", (await admin.call("GET", "/api/v1/users")).status === 401);
      const wrong = await admin.call("POST", "/api/v1/auth/login", { email: "admin@example.com", password: "nope-nope" });
      expect("wrong password → 401", wrong.status === 401, wrong.body);
      const ok = await admin.call("POST", "/api/v1/auth/login", { email: "ADMIN@example.com", password: "admin-pass-123" });
      expect("bootstrap admin signs in → 200 + HttpOnly SameSite=Lax cookie",
        ok.status === 200 && /HttpOnly/i.test(ok.setCookie ?? "") && /SameSite=Lax/i.test(ok.setCookie ?? ""), ok.body);
      const me = await admin.call("GET", "/api/v1/auth/me");
      expect("me (cookie) → admin", me.status === 200 && me.body.data?.email === "admin@example.com", me.body);
      const bearer = await fetch(`http://localhost:${port}/api/v1/auth/me`, { headers: { Authorization: `Bearer ${ok.body.data.token}` } });
      expect("me (Bearer) → 200", bearer.status === 200);
      const created = await admin.call("POST", "/api/v1/users", { name: "Ana", email: "ana@example.com", password: "ana-pass-123" });
      expect("create user → 201", created.status === 201, created.body);
      const dup = await admin.call("POST", "/api/v1/users", { name: "Ana 2", email: "Ana@Example.com", password: "ana-pass-123" });
      expect("duplicate email → 422 on field email", dup.status === 422 && dup.body.errors?.[0]?.field === "email", dup.body);
      const list = await admin.call("GET", "/api/v1/users?q=ana&page=1&limit=10");
      expect("search → 1 result with meta", list.status === 200 && list.body.meta?.total === 1, list.body);
      const adminId = me.body.data.id;
      expect("deactivate self → 409", (await admin.call("PUT", `/api/v1/users/${adminId}`, { name: "Admin", email: "admin@example.com", is_active: false })).status === 409);
      expect("delete self → 409", (await admin.call("DELETE", `/api/v1/users/${adminId}`)).status === 409);

      const ana = client(port);
      expect("new user signs in", (await ana.call("POST", "/api/v1/auth/login", { email: "ana@example.com", password: "ana-pass-123" })).status === 200);
      const anaId = created.body.data.id;
      expect("admin deactivates her", (await admin.call("PUT", `/api/v1/users/${anaId}`, { name: "Ana", email: "ana@example.com", is_active: false })).status === 200);
      expect("her open session ends on the next request → 401", (await ana.call("GET", "/api/v1/auth/me")).status === 401);
      expect("admin deletes her → 200", (await admin.call("DELETE", `/api/v1/users/${anaId}`)).status === 200);

      // rbac: the bootstrap admin holds Administrator (*); a user without roles holds nothing.
      const perms = await admin.call("GET", "/api/v1/rbac/permissions");
      const names = (perms.body.data ?? []).map((p) => p.name);
      expect("permission catalog = what the modules declared",
        ["rbac.manage", "users.manage", "users.read"].every((n) => names.includes(n)), names);
      const adminMe = await admin.call("GET", "/api/v1/rbac/me");
      expect("admin's effective permissions include every declared one", adminMe.body.data?.permissions?.length === names.length, adminMe.body);
      const budi = await admin.call("POST", "/api/v1/users", { name: "Budi", email: "budi@example.com", password: "budi-pass-123" });
      const budiId = budi.body.data.id;
      const b = client(port);
      await b.call("POST", "/api/v1/auth/login", { email: "budi@example.com", password: "budi-pass-123" });
      expect("user without roles: users list → 403", (await b.call("GET", "/api/v1/users")).status === 403);
      expect("user without roles: rbac/me → no permissions", (await b.call("GET", "/api/v1/rbac/me")).body.data?.permissions?.length === 0);
      const viewer = await admin.call("POST", "/api/v1/rbac/roles", { name: "Viewer", description: "Read only", permissions: ["users.read"] });
      expect("create role Viewer (users.read) → 201", viewer.status === 201, viewer.body);
      const bad = await admin.call("POST", "/api/v1/rbac/roles", { name: "Bad", permissions: ["nope.never"] });
      expect("unknown permission → 422", bad.status === 422, bad.body);
      expect("give Budi the Viewer role", (await admin.call("POST", `/api/v1/rbac/roles/${viewer.body.data.id}/members`, { user_id: budiId })).status === 201);
      expect("Budi can now read users (immediately)", (await b.call("GET", "/api/v1/users")).status === 200);
      expect("but not change them → 403", (await b.call("POST", "/api/v1/users", { name: "X", email: "x@example.com", password: "x-pass-1234" })).status === 403);
      expect("nor manage roles → 403", (await b.call("GET", "/api/v1/rbac/roles")).status === 403);
      const roles = await admin.call("GET", "/api/v1/rbac/roles");
      const adminRole = roles.body.data.find((r) => r.is_system);
      expect("Administrator role exists with one member", adminRole?.member_count === 1, roles.body);
      expect("Administrator cannot be changed → 409", (await admin.call("PUT", `/api/v1/rbac/roles/${adminRole.id}`, { name: "Root", permissions: [] })).status === 409);
      expect("the last Administrator keeps the role → 409", (await admin.call("DELETE", `/api/v1/rbac/roles/${adminRole.id}/members/${adminId}`)).status === 409);
      expect("delete the Viewer role → Budi loses access", (await admin.call("DELETE", `/api/v1/rbac/roles/${viewer.body.data.id}`)).status === 200
        && (await b.call("GET", "/api/v1/users")).status === 403);
      const out = await admin.call("POST", "/api/v1/auth/logout");
      expect("logout clears the cookie", out.status === 200 && /Max-Age=0/i.test(out.setCookie ?? ""), out.setCookie);
      expect("after logout → 401", (await admin.call("GET", "/api/v1/auth/me")).status === 401);
      const spa = await fetch(`http://localhost:${port}/users`);
      expect("SPA served for /users (one container)", spa.status === 200 && (await spa.text()).includes('id="root"'));
      expect("unknown API path stays JSON 404", (await admin.call("GET", "/api/v1/nope")).status === 404);
    } catch (err) {
      fail(`full API checks: ${err.message}${api ? `\n${api.output()}` : ""}`);
    } finally {
      stopApi(api);
      mysql(db, `DROP DATABASE IF EXISTS ${name};`);
    }
  } finally {
    if (!KEEP) rmSync(dir, { recursive: true, force: true });
    else log(`  (kept ${dir})`);
  }
}

async function noRbac() {
  log("\n=== no-rbac: users + auth without rbac — signed in = allowed ===");
  const dir = scaffold("no-rbac");
  let api;
  let db;
  const name = `elco_testbed_norbac_${Date.now()}`;
  try {
    installModule(LIB, dir, "users");
    installModule(LIB, dir, "auth", { ports: { UserStore: "users" } });
    pass("users + auth installed, no rbac");
    if (!build(dir)) return;
    if (!DB_DSN) {
      skip("no-rbac: API checks against MySQL — set TESTBED_DB_DSN");
      return;
    }
    db = parseDsn(DB_DSN);
    const dsn = freshDatabase(dir, db, name);
    api = await startApi(dir, 18094, { DB_DSN: dsn, USERS_BOOTSTRAP_EMAIL: "admin@example.com", USERS_BOOTSTRAP_PASSWORD: "admin-pass-123" });
    const admin = client(18094);
    await admin.call("POST", "/api/v1/auth/login", { email: "admin@example.com", password: "admin-pass-123" });
    await admin.call("POST", "/api/v1/users", { name: "Cici", email: "cici@example.com", password: "cici-pass-123" });
    const c = client(18094);
    await c.call("POST", "/api/v1/auth/login", { email: "cici@example.com", password: "cici-pass-123" });
    expect("any signed-in user may manage users (users.manage, default policy)",
      (await c.call("POST", "/api/v1/users", { name: "Dodi", email: "dodi@example.com", password: "dodi-pass-123" })).status === 201);
    expect("not signed in is still 401", (await client(18094).call("GET", "/api/v1/users")).status === 401);
  } catch (err) {
    fail(`no-rbac: ${err.message}${api ? `\n${api.output()}` : ""}`);
  } finally {
    stopApi(api);
    if (db) mysql(db, `DROP DATABASE IF EXISTS ${name};`);
    if (!KEEP) rmSync(dir, { recursive: true, force: true });
  }
}

async function authAlone() {
  log("\n=== auth-alone: auth with a project-written UserStore, no users module ===");
  const dir = scaffold("auth-alone");
  let api;
  try {
    const goModule = readFileSync(join(dir, "apps/api/go.mod"), "utf8").match(/^module (\S+)/m)[1];
    installModule(LIB, dir, "auth", { ports: { UserStore: { expr: "projectUserStore{}" } } });
    writeFileSync(join(dir, "apps/api/internal/modules/adapter_auth_project.go"),
      readFileSync(join(LIB, "testbed/fixtures/adapter_auth_project.go"), "utf8").replaceAll("__GO_MODULE__", goModule));
    pass("auth installed with UserStore ← projectUserStore{}");
    if (!build(dir)) return;
    api = await startApi(dir, 18092, {});
    const c = client(18092);
    const ok = await c.call("POST", "/api/v1/auth/login", { email: "fixture@example.com", password: "fixture-pass-123" });
    expect("sign in against the project's own store → 200", ok.status === 200, ok.body);
    const me = await c.call("GET", "/api/v1/auth/me");
    expect("me → the project's user", me.status === 200 && me.body.data?.id === 7, me.body);
    expect("no users module → /api/v1/users is 404", (await c.call("GET", "/api/v1/users")).status === 404);
  } catch (err) {
    fail(`auth-alone: ${err.message}`);
  } finally {
    stopApi(api);
    if (!KEEP) rmSync(dir, { recursive: true, force: true });
  }
}

async function usersAlone() {
  log("\n=== users-alone: users with no auth provider ===");
  const dir = scaffold("users-alone");
  let api;
  try {
    installModule(LIB, dir, "users");
    pass("users installed alone (with its ui kits)");
    if (!build(dir)) return;
    api = await startApi(dir, 18093, {});
    const r = await client(18093).call("GET", "/api/v1/users");
    expect("guarded route with nothing in d.Auth → 401", r.status === 401 && r.body.success === false, r.body);
  } catch (err) {
    fail(`users-alone: ${err.message}`);
  } finally {
    stopApi(api);
    if (!KEEP) rmSync(dir, { recursive: true, force: true });
  }
}

// The module.mjs CLI over a library's life: add → customize → the library moves on →
// upgrade merges both; a clash conflicts; detach freezes; promote brings a module back.
// Runs against a throwaway git copy of the library, so the real repo is never committed to.
async function lifecycle() {
  log("\n=== lifecycle: module.mjs add / status / upgrade / detach / promote ===");
  const lib = mkdtempSync(join(LIB, ".testbed", "lib-"));
  const dir = scaffold("lifecycle");
  const g = (...a) => spawnSync("git", ["-C", lib, ...a], { encoding: "utf8" });
  const cli = (...a) => spawnSync(process.execPath, [join(lib, "scripts/module.mjs"), ...a, "--project", dir], { encoding: "utf8" });
  const edit = (file, from, to) => {
    const src = readFileSync(file, "utf8");
    if (!src.includes(from)) throw new Error(`${file} has no ${JSON.stringify(from)}`);
    writeFileSync(file, src.replace(from, to));
  };
  const bump = (version) => edit(join(lib, "modules/users/module.json"), /"version": "[^"]+"/.exec(readFileSync(join(lib, "modules/users/module.json"), "utf8"))[0], `"version": "${version}"`);
  try {
    for (const p of ["modules", "themes", "scripts", "docs"]) cpSync(join(LIB, p), join(lib, p), { recursive: true });
    g("init", "-q", "-b", "main");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "library v1");

    let r = cli("add", "users");
    expect("add users brings its four ui kits along", r.status === 0 && ["ui-core", "ui-data", "ui-forms", "ui-feedback"].every((k) => r.stdout.includes(`✓ ${k} `)), r.stderr || r.stdout);
    r = cli("add", "auth");
    expect("add auth without wiring its port is refused, project untouched",
      r.status === 1 && r.stderr.includes("--port UserStore=") && !existsSync(join(dir, "apps/api/internal/modules/auth")), r.stderr);
    r = cli("add", "auth", "--port", "UserStore=users");
    expect("add auth --port UserStore=users", r.status === 0 && r.stdout.includes("Next:"), r.stderr || r.stdout);
    r = cli("preset", "bingkai-navy");
    expect("preset bingkai-navy", r.status === 0, r.stderr);
    let st = JSON.parse(cli("status", "--json").stdout);
    expect("status: everything managed, nothing changed", st.length === 7 && st.every((s) => s.status === "managed" && !s.modified.length), st);

    // The project customizes a module; the library ships a change elsewhere in the same file.
    const page = join(dir, "apps/web/src/modules/users/pages/UsersPage.tsx");
    edit(page, "Orang yang bisa masuk ke aplikasi ini.", "Akun staf gudang.");
    edit(join(lib, "modules/users/web/pages/UsersPage.tsx"), 'noun="pengguna"', 'noun="akun"');
    writeFileSync(join(lib, "modules/users/migrations/002_add_users_phone.up.sql"), "ALTER TABLE users ADD COLUMN phone VARCHAR(30) NULL;\n");
    writeFileSync(join(lib, "modules/users/migrations/002_add_users_phone.down.sql"), "ALTER TABLE users DROP COLUMN phone;\n");
    bump("0.9.0");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "users 0.2.0");

    st = JSON.parse(cli("status", "--json").stdout).find((s) => s.id === "users");
    expect("status: users changed here, 0.9.0 available",
      st.modified.includes("apps/web/src/modules/users/pages/UsersPage.tsx") && st.available === "0.9.0", st);
    r = cli("upgrade", "users");
    const merged = readFileSync(page, "utf8");
    expect("upgrade users merges cleanly (exit 0)", r.status === 0, r.stdout + r.stderr);
    expect("both the project's change and the library's are in the file",
      merged.includes("Akun staf gudang.") && merged.includes('noun="akun"'));
    expect("the new migration is added, the applied one untouched",
      readdirSync(join(dir, "apps/api/migrations")).filter((f) => f.includes("users_add_users_phone")).length === 2);
    if (!build(dir)) return;

    // The library changes the very line the project changed.
    edit(join(lib, "modules/users/web/pages/UsersPage.tsx"), "Orang yang bisa masuk ke aplikasi ini.", "Semua pemilik akun.");
    bump("0.10.0");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "users 0.3.0");
    r = cli("upgrade", "users");
    expect("a clash is reported as a conflict (exit 2) with markers in the file",
      r.status === 2 && readFileSync(page, "utf8").includes("<<<<<<<") && r.stdout.includes("CONFLICTS"), r.stdout);

    r = cli("detach", "auth");
    expect("detach auth", r.status === 0);
    r = cli("upgrade", "auth");
    expect("upgrade of a detached module is refused", r.status === 1 && r.stderr.includes("detached"), r.stderr);

    // Promote: a module the project built goes back to the library.
    r = cli("promote", "dashboard", "--lib", lib);
    const pm = existsSync(join(lib, "modules/dashboard/module.json")) && JSON.parse(readFileSync(join(lib, "modules/dashboard/module.json"), "utf8"));
    expect("promote dashboard → library module with kind and registry read from the code",
      r.status === 0 && pm.kind === "web" && pm.register?.web?.export === "dashboardModule", r.stderr || pm);
    const cat = spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs")], { encoding: "utf8" });
    expect("the promoted module passes the catalog check", cat.status === 0, cat.stderr);
    const bad = join(dir, "apps/api/internal/modules/billing");
    mkdirSync(bad, { recursive: true });
    writeFileSync(join(bad, "billing.module.go"), `package billing\n\nimport _ "${readFileSync(join(dir, "apps/api/go.mod"), "utf8").match(/^module (\S+)/m)[1]}/internal/modules/users/contracts"\n`);
    r = cli("promote", "billing", "--lib", lib);
    expect("promote refuses a module that imports another", r.status === 1 && r.stderr.includes('imports module "users"'), r.stderr);
  } catch (err) {
    fail(`lifecycle: ${err.message}`);
  } finally {
    if (!KEEP) {
      rmSync(dir, { recursive: true, force: true });
      rmSync(lib, { recursive: true, force: true });
    }
  }
}

if (!existsSync(join(SKILL, "scripts/scaffold.mjs"))) {
  log(`monorepo-elcodelabs not found at ${SKILL} — pass --skill <path> or set SKILL_DIR.`);
  process.exit(1);
}
for (const tool of ["go", "sqlc", "gofmt"]) {
  if (spawnSync("which", [tool]).status !== 0) {
    log(`'${tool}' is required on PATH.`);
    process.exit(1);
  }
}

const scenarios = { full, "no-rbac": noRbac, "auth-alone": authAlone, "users-alone": usersAlone, lifecycle };
for (const [name, fn] of Object.entries(scenarios)) {
  if (!ONLY || ONLY === name) await fn();
}

log("\n=== Summary ===");
for (const s of skipped) log(`  ⚠ skipped: ${s}`);
if (failures) {
  log(`FAILED: ${failures} check(s).`);
  process.exit(1);
}
log("All testbed checks passed.");

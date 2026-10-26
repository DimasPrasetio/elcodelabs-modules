#!/usr/bin/env node
// Testbed — proves the library's modules install into a fresh project and work.
//
// Five scenarios, each in its own fresh project: a copy of the pinned kernel host (kernel/host,
// scaffolded by a released monorepo-elcodelabs — see kernel/README.md), so nothing outside this
// repository is needed:
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
// Usage: node scripts/testbed.mjs [--only <scenario>] [--keep] [--skill <path>]
//   --skill (or $SKILL_DIR) scaffolds from a monorepo-elcodelabs working copy instead of the
//   pinned host — for the skill's maintainer, to try an unreleased kernel against the library
//   before releasing it.

import { spawn, spawnSync } from "node:child_process";
import net from "node:net";
import { copyFileSync, cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { applyPreset, installModule as installOne, missingKits } from "./lib/install.mjs";
import { HOST_SCAFFOLD, materializeHost, readKernelLock } from "./lib/kernel.mjs";
import { createHash } from "node:crypto";

const LIB = join(dirname(fileURLToPath(import.meta.url)), "..");
const argv = process.argv.slice(2);
const flag = (name) => {
  const i = argv.indexOf(name);
  return i === -1 ? undefined : argv[i + 1];
};
const SKILL = flag("--skill") ?? process.env.SKILL_DIR;
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
  if (SKILL) {
    const r = spawnSync(process.execPath, [join(SKILL, "scripts/scaffold.mjs"), "--name", HOST_SCAFFOLD.name, "--backend", HOST_SCAFFOLD.backend, "--dir", dir, "--force"], { encoding: "utf8" });
    if (r.status !== 0) throw new Error(`scaffold failed: ${r.stderr}`);
  } else {
    materializeHost(LIB, dir);
  }
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

// --- mail (full scenario) ------------------------------------------------------------------

/** The smallest SMTP server that takes every message: the mailer sends to it (SMTP_TLS=none). */
function smtpSink() {
  const messages = [];
  const server = net.createServer((sock) => {
    let buf = "";
    let data = null;
    let rcpt = [];
    const say = (s) => sock.write(s + "\r\n");
    say("220 testbed");
    sock.on("data", (chunk) => {
      buf += chunk.toString("utf8");
      let i;
      while ((i = buf.indexOf("\r\n")) >= 0) {
        const line = buf.slice(0, i);
        buf = buf.slice(i + 2);
        if (data !== null) {
          if (line === ".") {
            messages.push({ to: rcpt.join(","), raw: data.join("\r\n") });
            data = null;
            rcpt = [];
            say("250 queued");
          } else data.push(line.startsWith("..") ? line.slice(1) : line);
          continue;
        }
        const cmd = line.toUpperCase();
        if (cmd.startsWith("EHLO") || cmd.startsWith("HELO")) say("250 testbed");
        else if (cmd.startsWith("RCPT TO:")) {
          rcpt.push(line.slice(8).replace(/[<>]/g, "").trim());
          say("250 ok");
        } else if (cmd === "DATA") {
          data = [];
          say("354 go");
        } else if (cmd === "QUIT") {
          say("221 bye");
          sock.end();
        } else say("250 ok");
      }
    });
  });
  return new Promise((resolve) => server.listen(0, "127.0.0.1", () => resolve({ port: server.address().port, messages, close: () => server.close() })));
}

/** Quoted-printable body text, decoded. */
const qpDecode = (s) => s.replace(/=\r?\n/g, "").replace(/=([0-9A-F]{2})/g, (_, h) => String.fromCharCode(parseInt(h, 16)));

async function waitFor(fn, ms = 30000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    const v = fn();
    if (v) return v;
    await new Promise((r) => setTimeout(r, 500));
  }
  return null;
}

/** A 1×1 PNG. */
const PNG = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==", "base64");

// --- database (full scenario) ------------------------------------------------------------

function parseDsn(dsn) {
  const m = dsn.match(/^([^:@]*)(?::([^@]*))?@tcp\(([^:)]+)(?::(\d+))?\)\/(.*)$/);
  if (!m) throw new Error(`TESTBED_DB_DSN is not a Go MySQL DSN: ${dsn}`);
  return { user: m[1], pass: m[2] ?? "", host: m[3], port: m[4] ?? "3306" };
}

/** One value from a query (first column of the first row). */
function mysqlQuery(db, sql, database) {
  const args = ["-h", db.host, "-P", db.port, "-u", db.user, ...(db.pass ? [`-p${db.pass}`] : []), "-N", "-B", database];
  const r = spawnSync("mysql", args, { input: sql, encoding: "utf8" });
  if (r.status !== 0) throw new Error(`mysql: ${r.stderr}`);
  return r.stdout.trim().split("\n")[0];
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
  log("\n=== full: bingkai-navy + ui kits + every module, all ports wired, against MySQL and an SMTP sink ===");
  const dir = scaffold("full");
  try {
    applyPreset(LIB, dir, "bingkai-navy");
    for (const [id, opts] of [
      ["app-shell"], ["files"], ["mailer"], ["users"],
      ["auth", { ports: { UserStore: "users", PasswordWriter: "users", Mailer: "mailer" } }],
      ["rbac", { ports: { UserDirectory: "users" } }],
      ["settings", { ports: { FileStore: "files" } }],
      ["audit-log", { ports: { UserDirectory: "users" } }],
      ["ui-gallery"],
    ]) {
      installModule(LIB, dir, id, opts);
    }
    pass("preset + 4 ui kits + app-shell, files, mailer, users, auth, rbac, settings, audit-log, ui-gallery installed through the markers");
    const reg = readFileSync(join(dir, "apps/api/internal/modules/registry.go"), "utf8");
    expect("registry wires auth's three ports through adapters",
      reg.includes("auth.New(d, authUserStore{users: usersModule.Client()}, authPasswordWriter{users: usersModule.Client()}, authMailer{mail: mailerModule.Client()})")
      && existsSync(join(dir, "apps/api/internal/modules/adapter_auth_userstore.go")) && existsSync(join(dir, "apps/api/internal/modules/adapter_auth_mailer.go")));
    expect("registry hands d.Audit to audit-log", reg.includes("d.Audit.Provide(auditLogModule.Sink())"));
    const backendModules = ["users", "auth", "rbac", "files", "mailer", "settings", "audit-log"];
    for (const id of backendModules) {
      const src = readdirSync(join(dir, `apps/api/internal/modules/${id}`), { recursive: true })
        .filter((f) => f.endsWith(".go"))
        .map((f) => readFileSync(join(dir, `apps/api/internal/modules/${id}`, f), "utf8"))
        .join("\n");
      const others = backendModules.filter((o) => o !== id);
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
    let api2;
    const sink = await smtpSink();
    try {
      const dsn = freshDatabase(dir, db, name);
      const port = 18091;
      const apiEnv = {
        DB_DSN: dsn,
        USERS_BOOTSTRAP_EMAIL: "admin@example.com",
        USERS_BOOTSTRAP_PASSWORD: "admin-pass-123",
        PUBLIC_DIR: join(dir, "apps/web/dist"),
        SMTP_HOST: "127.0.0.1", SMTP_PORT: String(sink.port), SMTP_TLS: "none", MAIL_FROM: "noreply@testbed.test",
        APP_URL: "http://testbed.test",
      };
      api = await startApi(dir, port, apiEnv);
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
      expect("a browser sign-in gets no token in the body (cookie only)", ok.body.data?.token === undefined, ok.body);
      const api1 = await client(port).call("POST", "/api/v1/auth/login", { email: "admin@example.com", password: "admin-pass-123", return_token: true });
      const token = api1.body.data?.token;
      const asBearer = (path) => fetch(`http://localhost:${port}${path}`, { headers: { Authorization: `Bearer ${token}` } });
      expect("me (Bearer, token asked for with return_token) → 200", Boolean(token) && (await asBearer("/api/v1/auth/me")).status === 200, api1.body);

      // Kernel 4: forged cross-site writes are refused whatever the domain layout.
      const raw = (headers, body = '{"name":"X","email":"x1@example.com","password":"x-pass-1234"}') =>
        fetch(`http://localhost:${port}/api/v1/users`, { method: "POST", headers: { Cookie: admin.cookie, ...headers }, body });
      expect("a text/plain body (a form a browser posts cross-site) → 415", (await raw({ "Content-Type": "text/plain" })).status === 415);
      expect("a write a browser marks same-site (a sibling subdomain) → 403", (await raw({ "Content-Type": "application/json", "Sec-Fetch-Site": "same-site" })).status === 403);
      expect("a write from a foreign Origin → 403", (await raw({ "Content-Type": "application/json", Origin: "https://evil.example" })).status === 403);
      const typeErr = await admin.call("POST", "/api/v1/users", { name: 5 });
      expect("a wrong JSON type → 400 without Go internals", typeErr.status === 400 && !/struct|unmarshal/i.test(typeErr.body.message ?? ""), typeErr.body);
      const created = await admin.call("POST", "/api/v1/users", { name: "Ana", email: "ana@example.com", password: "ana-pass-123" });
      expect("create user → 201", created.status === 201, created.body);
      const dup = await admin.call("POST", "/api/v1/users", { name: "Ana 2", email: "Ana@Example.com", password: "ana-pass-123" });
      expect("duplicate email → 422 on field email", dup.status === 422 && dup.body.errors?.[0]?.field === "email", dup.body);
      const list = await admin.call("GET", "/api/v1/users?q=ana&page=1&limit=10");
      expect("search → 1 result with meta", list.status === 200 && list.body.meta?.total === 1, list.body);
      const adminId = me.body.data.id;
      expect("deactivate self → 409", (await admin.call("PUT", `/api/v1/users/${adminId}`, { name: "Admin", email: "admin@example.com", is_active: false })).status === 409);
      expect("users are never deleted: DELETE /users/{id} is not a route", [404, 405].includes((await admin.call("DELETE", `/api/v1/users/${adminId}`)).status));

      const ana = client(port);
      expect("new user signs in", (await ana.call("POST", "/api/v1/auth/login", { email: "ana@example.com", password: "ana-pass-123" })).status === 200);
      const anaId = created.body.data.id;
      const renamed = await admin.call("PUT", `/api/v1/users/${anaId}`, { name: "Ana B", email: "ana@example.com" });
      expect("a PUT without is_active keeps her active", renamed.status === 200 && renamed.body.data?.is_active === true, renamed.body);
      expect("her session survives a rename", (await ana.call("GET", "/api/v1/auth/me")).status === 200);
      expect("admin resets her password", (await admin.call("PUT", `/api/v1/users/${anaId}`, { name: "Ana B", email: "ana@example.com", password: "ana-pass-456" })).status === 200);
      expect("the password change ends her earlier session → 401", (await ana.call("GET", "/api/v1/auth/me")).status === 401);
      expect("she signs in with the new password", (await ana.call("POST", "/api/v1/auth/login", { email: "ana@example.com", password: "ana-pass-456" })).status === 200);
      expect("admin deactivates her", (await admin.call("PUT", `/api/v1/users/${anaId}`, { name: "Ana B", email: "ana@example.com", is_active: false })).status === 200);
      expect("her open session ends on the next request → 401", (await ana.call("GET", "/api/v1/auth/me")).status === 401);

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

      // An operator manages users but not administrators; a role manager cannot climb.
      const operator = await admin.call("POST", "/api/v1/rbac/roles", { name: "Operator", permissions: ["users.read", "users.manage", "rbac.manage"] });
      await admin.call("POST", `/api/v1/rbac/roles/${operator.body.data.id}/members`, { user_id: budiId });
      expect("an operator cannot reset the administrator's password → 403",
        (await b.call("PUT", `/api/v1/users/${adminId}`, { name: "Admin", email: "admin@example.com", password: "taken-over-1" })).status === 403);
      expect("nor deactivate the administrator → 403",
        (await b.call("PUT", `/api/v1/users/${adminId}`, { name: "Admin", email: "admin@example.com", is_active: false })).status === 403);
      const cici = await b.call("POST", "/api/v1/users", { name: "Cici", email: "cici@example.com", password: "cici-pass-123" });
      expect("but manages people with no more rights than theirs", cici.status === 201
        && (await b.call("PUT", `/api/v1/users/${cici.body.data.id}`, { name: "Cici K", email: "cici@example.com" })).status === 200, cici.body);
      expect("rbac.manage cannot make yourself Administrator → 403",
        (await b.call("POST", `/api/v1/rbac/roles/${adminRole.id}/members`, { user_id: budiId })).status === 403);
      expect("the administrator still signs in with their own password",
        (await client(port).call("POST", "/api/v1/auth/login", { email: "admin@example.com", password: "admin-pass-123" })).status === 200);
// --- kernel 5 and its modules: settings, files, mailer, account, audit ----------------
      const co = await admin.call("GET", "/api/v1/settings/company");
      expect("settings: a fresh company profile is empty, in Asia/Jakarta, logo upload on", co.status === 200 && co.body.data?.timezone === "Asia/Jakarta" && co.body.data?.logo_upload === true, co.body);
      expect("settings: a malformed NPWP → 422 on tax_id", (await admin.call("PUT", "/api/v1/settings/company", { name: "Toko Maju", tax_id: "123" })).body.errors?.[0]?.field === "tax_id");
      const savedCo = await admin.call("PUT", "/api/v1/settings/company", { name: "Toko Maju", legal_name: "PT Toko Maju", tax_id: "01.234.567.8-901.000", email: "info@tokomaju.test", timezone: "Asia/Jakarta" });
      expect("settings: the profile saves", savedCo.status === 200 && savedCo.body.data?.name === "Toko Maju", savedCo.body);
      const uploadLogo = (filename, bytes, type) => {
        const form = new FormData();
        form.append("file", new Blob([bytes], { type }), filename);
        return fetch(`http://localhost:${port}/api/v1/settings/company/logo`, { method: "PUT", headers: { Cookie: admin.cookie }, body: form });
      };
      const up = await uploadLogo("logo.png", PNG, "image/png");
      expect("settings: a PNG logo uploads, streamed into files", up.status === 200 && (await up.json()).data?.has_logo === true);
      const svg = await uploadLogo("x.svg", Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>'), "image/svg+xml");
      expect("settings: an SVG logo is refused → 422", svg.status === 422);
      const disguised = await uploadLogo("x.png", Buffer.from("<html><script>alert(1)</script></html>"), "image/png");
      expect("settings: HTML named .png is refused → 422", disguised.status === 422);
      const logo = await fetch(`http://localhost:${port}/api/v1/settings/company/logo`);
      expect("settings: the logo is public, typed by its content, nosniff",
        logo.status === 200 && logo.headers.get("content-type") === "image/png" && logo.headers.get("x-content-type-options") === "nosniff");

      const fl = await admin.call("GET", "/api/v1/files");
      const logoFile = fl.body.data?.find((f) => f.owner === "settings");
      expect("files: the logo is listed with its module, type, and SHA-256", fl.status === 200 && logoFile?.content_type === "image/png" && logoFile?.sha256?.length === 64 && fl.body.data.length === 1, fl.body);
      const dl = await fetch(`http://localhost:${port}/api/v1/files/${logoFile?.id}/download`, { headers: { Cookie: admin.cookie } });
      expect("files: a download is an attachment in a sandbox, never rendered",
        dl.status === 200 && /attachment/.test(dl.headers.get("content-disposition") ?? "") && /sandbox/.test(dl.headers.get("content-security-policy") ?? ""));
      expect("files: someone without files.read → 403", (await b.call("GET", "/api/v1/files")).status === 403);

      const prof = await admin.call("PUT", "/api/v1/account/profile", { name: "Admin Utama" });
      expect("users: the signed-in user renames themselves", prof.status === 200 && (await admin.call("GET", "/api/v1/auth/me")).body.data?.name === "Admin Utama", prof.body);

      const opts = await client(port).call("GET", "/api/v1/auth/options");
      expect("auth: change and reset are on (PasswordWriter and Mailer wired)", opts.body.data?.password_change === true && opts.body.data?.password_reset === true, opts.body);
      const other = client(port);
      await other.call("POST", "/api/v1/auth/login", { email: "admin@example.com", password: "admin-pass-123" });
      expect("auth: a wrong current password → 422", (await admin.call("POST", "/api/v1/auth/password", { current_password: "nope-nope-1", new_password: "admin-pass-456" })).status === 422);
      const changed = await admin.call("POST", "/api/v1/auth/password", { current_password: "admin-pass-123", new_password: "admin-pass-456" });
      expect("auth: change password → this device keeps a fresh session", changed.status === 200 && (await admin.call("GET", "/api/v1/auth/me")).status === 200, changed.body);
      expect("auth: the other device is signed out", (await other.call("GET", "/api/v1/auth/me")).status === 401);

      const sentBefore = sink.messages.length;
      const forgot = await client(port).call("POST", "/api/v1/auth/forgot", { email: "admin@example.com" });
      const unknownForgot = await client(port).call("POST", "/api/v1/auth/forgot", { email: "nobody@example.com" });
      expect("auth: forgot answers the same for a known and an unknown address",
        forgot.status === 200 && unknownForgot.status === 200 && forgot.body.message === unknownForgot.body.message);
      const mail = await waitFor(() => sink.messages.slice(sentBefore).find((m) => m.to === "admin@example.com"), 30000);
      expect("mailer: the reset email arrives over SMTP, sent by the background job", Boolean(mail));
      const mailText = qpDecode(mail?.raw ?? "");
      const resetToken = mailText.match(/reset-password#token=([A-Za-z0-9_-]+)/)?.[1];
      expect("mailer: text + HTML alternatives, the link built from APP_URL",
        /multipart\/alternative/.test(mailText) && mailText.includes("http://testbed.test/reset-password#token=") && Boolean(resetToken), mailText.slice(0, 600));
      expect("mailer: nothing was queued for the unknown address", mysqlQuery(db, "SELECT COUNT(*) FROM mail_outbox WHERE to_address = 'nobody@example.com'", name) === "0");
      const reset = await client(port).call("POST", "/api/v1/auth/reset", { token: decodeURIComponent(resetToken ?? ""), new_password: "admin-pass-789" });
      expect("auth: the emailed token resets the password", reset.status === 200, reset.body);
      expect("auth: the reset ended the admin's session", (await admin.call("GET", "/api/v1/auth/me")).status === 401);
      expect("auth: the token works once", (await client(port).call("POST", "/api/v1/auth/reset", { token: decodeURIComponent(resetToken ?? ""), new_password: "admin-pass-000" })).status === 400);
      expect("auth: the new password signs in", (await admin.call("POST", "/api/v1/auth/login", { email: "admin@example.com", password: "admin-pass-789" })).status === 200);
      const ob = await admin.call("GET", "/api/v1/mailer/messages?status=sent");
      expect("mailer: the outbox lists it as sent", ob.status === 200 && ob.body.data?.some((m) => m.to === "admin@example.com" && m.status === "sent"), ob.body);
      expect("mailer: a sent message's bodies are cleared — the reset link does not stay in the database",
        mysqlQuery(db, "SELECT COUNT(*) FROM mail_outbox WHERE status = 'sent' AND (text_body <> '' OR html_body <> '')", name) === "0");

      const au = await admin.call("GET", "/api/v1/audit?limit=100");
      const actions = new Set((au.body.data ?? []).map((e) => e.action));
      const wanted = ["auth.signed_in", "auth.sign_in_failed", "user.created", "user.deactivated", "role.created", "role.member_added", "settings.company_updated",
        "settings.logo_updated", "auth.password_changed", "auth.password_reset_requested", "auth.password_reset", "user.profile_updated"];
      expect("audit-log: sign-ins, users, roles, settings, and password events are all in the trail", wanted.every((a) => actions.has(a)), wanted.filter((a) => !actions.has(a)));
      const coEntry = (au.body.data ?? []).find((e) => e.action === "settings.company_updated");
      expect("audit-log: an entry names its actor and carries IP, request ID, and the change", coEntry?.actor_name === "Admin Utama" && coEntry?.ip && coEntry?.request_id && coEntry?.data?.name?.after === "Toko Maju", coEntry);
      expect("audit-log: no password ever reaches the trail", !JSON.stringify(au.body).includes("admin-pass"));
      const bySettings = (await admin.call("GET", "/api/v1/audit?module=settings")).body.data ?? [];
      expect("audit-log: filtering by module", bySettings.length >= 2 && bySettings.every((e) => e.module === "settings"), bySettings.length);
      expect("audit-log: someone without audit-log.read → 403", (await b.call("GET", "/api/v1/audit")).status === 403);

      // Two API processes on one database (a deploy's overlap): every email goes out exactly once.
      api2 = await startApi(dir, 18096, apiEnv);
      const extra = [];
      for (let i = 1; i <= 4; i++) {
        const email = `staf${i}@example.com`;
        await admin.call("POST", "/api/v1/users", { name: `Staf ${i}`, email, password: "staf-pass-123" });
        await client(i % 2 ? port : 18096).call("POST", "/api/v1/auth/forgot", { email });
        extra.push(email);
      }
      await waitFor(() => extra.every((e) => sink.messages.some((m) => m.to === e)), 35000);
      await new Promise((r) => setTimeout(r, 12000)); // one more delivery round on each process
      const counts = extra.map((e) => sink.messages.filter((m) => m.to === e).length);
      expect("jobs: with two processes running, each of 4 emails was sent exactly once", counts.every((n) => n === 1), counts);

      const cookieBefore = admin.cookie;
      const out = await admin.call("POST", "/api/v1/auth/logout");
      expect("logout clears the cookie", out.status === 200 && /Max-Age=0/i.test(out.setCookie ?? ""), out.setCookie);
      expect("after logout → 401", (await admin.call("GET", "/api/v1/auth/me")).status === 401);
      const copied = await fetch(`http://localhost:${port}/api/v1/auth/me`, { headers: { Cookie: cookieBefore } });
      expect("a copy of the signed-out cookie no longer works → 401", copied.status === 401);
      await fetch(`http://localhost:${port}/api/v1/auth/logout`, { method: "POST", headers: { Authorization: `Bearer ${token}` } });
      expect("a Bearer token signed out ends too → 401", (await asBearer("/api/v1/auth/me")).status === 401);
      const spa = await fetch(`http://localhost:${port}/users`);
      expect("SPA served for /users (one container)", spa.status === 200 && (await spa.text()).includes('id="root"'));
      expect("unknown API path stays JSON 404", (await admin.call("GET", "/api/v1/nope")).status === 404);
    } catch (err) {
      fail(`full API checks: ${err.message}${api ? `\n${api.output()}` : ""}`);
    } finally {
      stopApi(api);
      stopApi(api2);
      sink.close();
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
    stopApi(api);
    api = undefined;
    let refused = "";
    try {
      api = await startApi(dir, 18095, { DB_DSN: dsn, APP_ENV: "production", JWT_SECRET: "x".repeat(40), PUBLIC_DIR: "" });
    } catch (err) {
      refused = err.message;
    }
    expect("in production, declared permissions with no authorization provider refuse to start",
      refused.includes("no authorization provider"), refused || "started");
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
  let db;
  const name = `elco_testbed_authalone_${Date.now()}`;
  try {
    const goModule = readFileSync(join(dir, "apps/api/go.mod"), "utf8").match(/^module (\S+)/m)[1];
    installModule(LIB, dir, "auth", { ports: { UserStore: { expr: "projectUserStore{}" } } });
    writeFileSync(join(dir, "apps/api/internal/modules/adapter_auth_project.go"),
      readFileSync(join(LIB, "testbed/fixtures/adapter_auth_project.go"), "utf8").replaceAll("__GO_MODULE__", goModule));
    pass("auth installed with UserStore ← projectUserStore{} (written before PasswordChangedAt existed: still compiles)");
    if (!build(dir)) return;
    if (!DB_DSN) {
      skip("auth-alone: API checks against MySQL (auth keeps its sessions) — set TESTBED_DB_DSN");
      return;
    }
    db = parseDsn(DB_DSN);
    api = await startApi(dir, 18092, { DB_DSN: freshDatabase(dir, db, name) });
    const c = client(18092);
    const ok = await c.call("POST", "/api/v1/auth/login", { email: "fixture@example.com", password: "fixture-pass-123" });
    expect("sign in against the project's own store → 200", ok.status === 200, ok.body);
    const me = await c.call("GET", "/api/v1/auth/me");
    expect("me → the project's user", me.status === 200 && me.body.data?.id === 7, me.body);
    expect("no users module → /api/v1/users is 404", (await c.call("GET", "/api/v1/users")).status === 404);
    const opts = await c.call("GET", "/api/v1/auth/options");
    expect("without PasswordWriter and Mailer, change and reset are off", opts.body.data?.password_change === false && opts.body.data?.password_reset === false, opts.body);
    expect("forgot → 501 when nothing can send it", (await c.call("POST", "/api/v1/auth/forgot", { email: "fixture@example.com" })).status === 501);
    expect("the optional ports are nil in the registry",
      readFileSync(join(dir, "apps/api/internal/modules/registry.go"), "utf8").includes("auth.New(d, projectUserStore{}, nil, nil)"));
  } catch (err) {
    fail(`auth-alone: ${err.message}${api ? `\n${api.output()}` : ""}`);
  } finally {
    stopApi(api);
    if (db) mysql(db, `DROP DATABASE IF EXISTS ${name};`);
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
    for (const p of ["modules", "themes", "scripts", "docs", "kernel"]) cpSync(join(LIB, p), join(lib, p), { recursive: true });
    for (const f of [".gitignore", ".gitattributes"]) cpSync(join(LIB, f), join(lib, f));
    writeFileSync(join(lib, "modules/ui-core/shared/.DS_Store"), "finder junk"); // ignored by git
    g("init", "-q", "-b", "main");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "library v1");

    let r = cli("add", "users");
    expect("add users brings its four ui kits along", r.status === 0 && ["ui-core", "ui-data", "ui-forms", "ui-feedback"].every((k) => r.stdout.includes(`✓ ${k} `)), r.stderr || r.stdout);
    expect("files git ignores in the library (.DS_Store) never reach the project", !existsSync(join(dir, "apps/web/src/shared/.DS_Store")));
    r = cli("add", "auth");
    expect("add auth without wiring its port is refused, project untouched",
      r.status === 1 && r.stderr.includes("--port UserStore=") && !existsSync(join(dir, "apps/api/internal/modules/auth")), r.stderr);
    r = cli("add", "auth", "--port", "UserStore=users");
    expect("add auth --port UserStore=users", r.status === 0 && r.stdout.includes("Next:"), r.stderr || r.stdout);
    expect("its Go dependencies are pinned in go.mod", /github\.com\/golang-jwt\/jwt\/v5 v5\./.test(readFileSync(join(dir, "apps/api/go.mod"), "utf8")));
    expect("module settings reach the production env example",
      readFileSync(join(dir, "infra/elcodelabs/.env.production.example"), "utf8").includes("USERS_BOOTSTRAP_EMAIL"));
    r = cli("preset", "bingkai-navy");
    expect("preset bingkai-navy", r.status === 0, r.stderr);
    let st = JSON.parse(cli("status", "--json").stdout);
    expect("status: everything managed, nothing changed", st.length === 7 && st.every((s) => s.status === "managed" && !s.modified.length), st);

    // A port wired later to a module added after the consumer: the consumer's registry lines
    // move after the provider's, or the composition root would not compile.
    r = cli("add", "mailer");
    expect("add mailer after auth", r.status === 0, r.stderr || r.stdout);
    // An adapter that needs a newer provider is refused while the project has an older one.
    const lockFile = join(dir, "modules.lock.json");
    const lockText = readFileSync(lockFile, "utf8");
    const oldLock = JSON.parse(lockText);
    oldLock.modules.users.version = "0.4.0";
    writeFileSync(lockFile, JSON.stringify(oldLock, null, 2) + "\n");
    r = cli("upgrade", "auth", "--port", "PasswordWriter=users");
    expect("wiring an adapter to a provider older than it needs is refused",
      r.status !== 0 && r.stderr.includes("needs users 0.5.0 or newer") && readFileSync(lockFile, "utf8") === JSON.stringify(oldLock, null, 2) + "\n", r.stderr || r.stdout);
    writeFileSync(lockFile, lockText);
    r = cli("upgrade", "auth", "--port", "Mailer=mailer", "--port", "PasswordWriter=users");
    const rewired = readFileSync(join(dir, "apps/api/internal/modules/registry.go"), "utf8");
    expect("wiring auth's Mailer port later moves auth after mailer in the registry",
      r.status === 0 && rewired.indexOf("auth.New(") > rewired.indexOf("mailer.New(") && rewired.includes("authMailer{mail: mailerModule.Client()}") && !rewired.includes(", nil"), rewired);

    // The project customizes a module; the library ships a change elsewhere in the same file.
    const page = join(dir, "apps/web/src/modules/users/pages/UsersPage.tsx");
    edit(page, "Orang yang bisa masuk ke aplikasi ini.", "Akun staf gudang.");
    edit(join(lib, "modules/users/web/pages/UsersPage.tsx"), 'noun="pengguna"', 'noun="akun"');
    writeFileSync(join(lib, "modules/users/migrations/003_add_users_phone.up.sql"), "ALTER TABLE users ADD COLUMN phone VARCHAR(30) NULL;\n");
    writeFileSync(join(lib, "modules/users/migrations/003_add_users_phone.down.sql"), "ALTER TABLE users DROP COLUMN phone;\n");
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

    // A file moves between two installed kits; upgrading both in lock order (the order that used
    // to delete it) keeps it, now owned by its new kit.
    const moved = "shared/components/ui/field-hint.tsx";
    cpSync(join(lib, "modules/ui-forms", moved), join(lib, "modules/ui-core", moved));
    rmSync(join(lib, "modules/ui-forms", moved));
    for (const kit of ["ui-core", "ui-forms"]) {
      const mf = join(lib, `modules/${kit}/module.json`);
      const m = JSON.parse(readFileSync(mf, "utf8"));
      m.version = m.version.replace(/\d+$/, (n) => String(Number(n) + 1));
      writeFileSync(mf, JSON.stringify(m, null, 2) + "\n");
    }
    // In the same release: users renames its web export, and changes a file the project deleted.
    edit(join(lib, "modules/users/web/module.tsx"), "export const usersModule", "export const peopleModule");
    edit(join(lib, "modules/users/module.json"), '"export": "usersModule"', '"export": "peopleModule"');
    const formSheet = "apps/web/src/modules/users/components/UserFormSheet.tsx";
    rmSync(join(dir, formSheet));
    writeFileSync(join(lib, "modules/users/web/components/UserFormSheet.tsx"), readFileSync(join(lib, "modules/users/web/components/UserFormSheet.tsx"), "utf8") + "\n// changed in the library\n");
    bump("0.11.0");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "move field-hint, rename export");
    r = cli("upgrade", "--all");
    const lockNow = JSON.parse(readFileSync(join(dir, "modules.lock.json"), "utf8"));
    expect("a file that moved between modules survives the upgrade, owned by its new module",
      existsSync(join(dir, "apps/web/src", moved)) && lockNow.modules["ui-core"].files[`apps/web/src/${moved}`] && !lockNow.modules["ui-forms"].files[`apps/web/src/${moved}`], r.stdout + r.stderr);
    const modulesTs = readFileSync(join(dir, "apps/web/src/app/modules.ts"), "utf8");
    expect("a changed web registration is rewritten in app/modules.ts",
      modulesTs.includes('import { peopleModule } from "@/modules/users/module"') && modulesTs.includes("  peopleModule,") && !modulesTs.includes("usersModule"), modulesTs);
    expect("a file the project deleted stays deleted", !existsSync(join(dir, formSheet)) && r.stdout.includes("deleted here"), r.stdout);

    // The commit a module was installed from is gone: its release tag stands in.
    const locked = JSON.parse(readFileSync(join(dir, "modules.lock.json"), "utf8"));
    g("tag", "-a", `users/v${locked.modules.users.version}`, "-m", "release", locked.modules.users.commit);
    locked.modules.users.commit = "0123456789abcdef0123456789abcdef01234567";
    writeFileSync(join(dir, "modules.lock.json"), JSON.stringify(locked, null, 2) + "\n");
    edit(join(lib, "modules/users/README.md"), "# users", "# users\n");
    edit(join(lib, "modules/users/web/components/UserFormSheet.tsx"), "// changed in the library", "// changed in the library again");
    bump("0.12.0");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "users 0.12.0");
    r = cli("upgrade", "users");
    expect("an upgrade whose base commit is gone uses the release tag", r.status !== 1 && r.stdout.includes("0.12.0"), r.stdout + r.stderr);

    // Uninstalling: refused while needed, clean when not.
    r = cli("remove", "ui-data");
    expect("remove refuses a kit another installed module builds on", r.status === 1 && r.stderr.includes("users"), r.stderr);
    r = cli("add", "ui-gallery");
    const galleryFile = join(dir, "apps/web/src/modules/ui-gallery/module.tsx");
    const hadGallery = existsSync(galleryFile);
    r = cli("remove", "ui-gallery");
    const after = readFileSync(join(dir, "apps/web/src/app/modules.ts"), "utf8");
    expect("remove ui-gallery takes its files, registration, and lock entry",
      hadGallery && r.status === 0 && !existsSync(galleryFile) && !after.includes("uiGalleryModule")
      && !JSON.parse(readFileSync(join(dir, "modules.lock.json"), "utf8")).modules["ui-gallery"], r.stdout + r.stderr);

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
    const loud = join(dir, "apps/web/src/modules/banner");
    mkdirSync(loud, { recursive: true });
    writeFileSync(join(loud, "module.tsx"), `export const bannerModule = { id: "banner", nav: [], slots: {} };\nexport const Banner = () => <div className="bg-red-500" />;\n`);
    r = cli("promote", "banner", "--lib", lib);
    expect("promote refuses a module that breaks an architecture rule, leaving the library untouched",
      r.status === 1 && r.stderr.includes("semantic-colors") && !existsSync(join(lib, "modules/banner")), r.stderr);

    // The pinned kernel: read-only from the library's side.
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "promoted");
    const check = () => spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs"), "--check", "--since", "HEAD"], { cwd: lib, encoding: "utf8" });
    const kernelGo = join(lib, "kernel/host/apps/api/internal/kernel/kernel.go");
    const kernelSrc = readFileSync(kernelGo, "utf8");
    writeFileSync(kernelGo, kernelSrc + "\n// a module developer's edit\n");
    r = check();
    expect("an edit to the pinned kernel is refused", r.status === 1 && r.stderr.includes("pinned-kernel") && r.stderr.includes("host/apps/api/internal/kernel/kernel.go"), r.stderr);
    const hostCopy = mkdtempSync(join(LIB, ".testbed", "host-"));
    let refused = false;
    try { materializeHost(lib, hostCopy); } catch { refused = true; }
    rmSync(hostCopy, { recursive: true, force: true });
    expect("a tampered kernel is never copied into a project", refused);
    writeFileSync(kernelGo, kernelSrc);
    edit(join(lib, "modules/users/module.json"), /"requires_kernel": \d+/.exec(readFileSync(join(lib, "modules/users/module.json"), "utf8"))[0], `"requires_kernel": ${readKernelLock(lib).kernel + 1}`);
    r = check();
    expect("a module asking for a kernel newer than the pinned one is refused", r.status === 1 && r.stderr.includes("kernel-pinned"), r.stderr);
    g("checkout", "--", "modules/users/module.json");
    // A kernel sync and a module change in one commit: the lock is consistent, the mix is not.
    const kernelLockFile = join(lib, "kernel/kernel.lock.json");
    const kernelLock = JSON.parse(readFileSync(kernelLockFile, "utf8"));
    const readme = readFileSync(join(lib, "kernel/README.md"), "utf8") + "\nsynced again\n";
    writeFileSync(join(lib, "kernel/README.md"), readme);
    kernelLock.files["README.md"] = createHash("sha256").update(readme).digest("hex");
    writeFileSync(kernelLockFile, JSON.stringify(kernelLock, null, 2) + "\n");
    edit(join(lib, "modules/users/README.md"), "#", "# ");
    r = check();
    expect("a kernel update mixed with a module change is refused (kernel-alone)", r.status === 1 && r.stderr.includes("kernel-alone") && !r.stderr.includes("pinned-kernel"), r.stderr);
    g("checkout", "--", "modules/users/README.md");
    r = check();
    expect("the same kernel update on its own passes", r.status === 0, r.stderr);
  } catch (err) {
    fail(`lifecycle: ${err.message}`);
  } finally {
    if (!KEEP) {
      rmSync(dir, { recursive: true, force: true });
      rmSync(lib, { recursive: true, force: true });
    }
  }
}

if (SKILL) {
  if (!existsSync(join(SKILL, "scripts/scaffold.mjs"))) {
    log(`monorepo-elcodelabs not found at ${SKILL}.`);
    process.exit(1);
  }
  log(`Projects scaffolded from ${SKILL} (not the pinned kernel).`);
} else {
  const lock = readKernelLock(LIB);
  if (!lock) {
    log("No pinned kernel (kernel/kernel.lock.json) — see kernel/README.md.");
    process.exit(1);
  }
  log(`Projects copied from the pinned kernel ${lock.kernel} host (${lock.source.tag ?? lock.source.commit.slice(0, 7)}).`);
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

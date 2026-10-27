#!/usr/bin/env node
// Testbed — proves the library's modules install into a fresh project and work.
//
// The scenarios, each in its own fresh project: a copy of the pinned kernel host (kernel/host,
// scaffolded by a released monorepo-elcodelabs — see kernel/README.md), so nothing outside this
// repository is needed:
//   full        preset + the four ui kits + every base module, all ports wired, against MySQL and
//               an SMTP sink: sign-in and its limits, users and own profile, sessions ended by
//               sign-out / deactivation / password change, cross-site and non-JSON writes
//               refused, roles and permissions (no escalation, administrators protected), forgot
//               password by email, the outbox, a logo streamed into files (SVG and disguised HTML
//               refused), the audit trail, two API processes delivering each email exactly once
//   no-rbac     users + auth without rbac: the kernel's default "signed in = allowed"
//   auth-alone  auth on a UserStore the project wrote itself — no users module, no mailer: the
//               password features report themselves off and the optional ports are nil
//   users-alone users without any auth provider: its guarded routes refuse (401)
//   lifecycle   the module.mjs CLI on a throwaway library copy: add (and a refused add), status,
//               upgrade merging both sides, a conflicting upgrade, detach, a port wired later,
//               remove, release-tag fallback, promote (refused for a rule breach), the library's
//               own documents (status-current, a stale STATUS.md snapshot, rules-documented,
//               testbed-covered, work branches warned and never tagged), and the pinned kernel
//               guards (an edit, a too-new module, a mixed kernel commit refused)
//   kernel6     fixture modules against MySQL: one d.Tx across modules joined by a port, an event
//               delivered once across two containers, companies kept apart, a missed calendar
//               tick caught up once; the rules refusing scope, transaction, and event breaches
//   organization every module + organization added later, its late ports wired with module.mjs:
//               several companies and branches against MySQL — isolation, refusals, branch
//               heads, deactivation, recovery, public routes and jobs without panic
//   documents   money, docstate, numbering, period through a fixture invoice module: exact
//               amounts, gapless numbers from two containers at once, a rollback giving its
//               number back, closing and reopening the books, a lock moved mid-posting,
//               revisions, company isolation; the exact-numbers rule
// Each: install → npm install → go mod tidy → sqlc generate → gofmt → go vet → go test →
// build:web → build:api → run the binary and call the API.
//
// The API checks need MySQL: set TESTBED_DB_DSN to a server DSN without a
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
let passes = 0;
const skipped = [];
const log = (m) => process.stdout.write(m + "\n");
const pass = (m) => { passes++; log(`  ✓ ${m}`); };
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
    && run("go", ["test", "./..."], api, `go test ./... (module unit tests, ports faked${DB_DSN ? "; kernel MySQL tests" : ""})`, DB_DSN ? { KERNEL_TEST_DSN: DB_DSN } : undefined)
    && run("npm", ["run", "build:web"], dir, "build:web (tsc + vite)")
    && run("npm", ["run", "build:api"], dir, "build:api");
}

/** A working copy of the library — everything its checks read — for a scenario to change. */
function copyLibrary(lib) {
  for (const p of ["modules", "themes", "scripts", "docs", "kernel", "testbed"]) cpSync(join(LIB, p), join(lib, p), { recursive: true });
  for (const f of [".gitignore", ".gitattributes", "AGENTS.md", "STATUS.md", "CATALOG.md", "catalog.json"]) cpSync(join(LIB, f), join(lib, f));
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
    const v = await fn(); // fn may be async
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
      const security = (await admin.call("GET", "/api/v1/audit?kind=security&limit=100")).body.data ?? [];
      expect("audit-log: sign-ins and role changes are security records, filterable by kind (kernel 6)",
        security.some((e) => e.action === "auth.signed_in") && security.some((e) => e.action === "role.member_added") && security.every((e) => e.kind === "security"), security.length);

      // Kernel 6: mail goes through the kernel's task queue; without an organization module only
      // company 1 exists, and asking for another is refused.
      expect("mailer: delivered by kernel tasks", Number(mysqlQuery(db, "SELECT COUNT(*) FROM kernel_tasks WHERE kind = 'mailer.deliver' AND status = 'done'", name)) >= 1);
      expect("scope: X-Scope for a company that does not exist here → 403", (await admin.call("GET", "/api/v1/users", undefined, { "X-Scope": "2" })).status === 403);
      expect("settings, files, roles, and the audit trail are company 1's",
        mysqlQuery(db, "SELECT COUNT(*) FROM settings WHERE company_id <> 1", name) === "0" && mysqlQuery(db, "SELECT COUNT(*) FROM audit_log WHERE company_id <> 1", name) === "0");

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
// Kernel 6 end to end, with fixture modules (testbed/fixtures/kernel6) in a throwaway copy of
// the library: a Go kit, ports inside one d.Tx across modules, a durable event, company scope,
// a calendar job run once across two containers, and the rules that keep modules honest.
async function kernel6() {
  log("\n=== kernel6: one transaction across modules, events, scope, a Go kit, calendar jobs ===");
  const lib = mkdtempSync(join(LIB, ".testbed", "lib6-"));
  const dir = scaffold("kernel6");
  const g = (...a) => spawnSync("git", ["-C", lib, ...a], { encoding: "utf8" });
  let api;
  let api2;
  try {
    copyLibrary(lib);
    for (const id of ["fixkit", "org6", "ledger", "orders"]) cpSync(join(LIB, "testbed/fixtures/kernel6", id), join(lib, "modules", id), { recursive: true });
    g("init", "-q", "-b", "main");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "library + kernel 6 fixtures");
    const check = () => spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs"), "--check"], { cwd: lib, encoding: "utf8" });
    spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs")], { cwd: lib, encoding: "utf8" });
    let r = check();
    expect("the fixtures (scoped modules, a Go kit, emits/listens) pass the architecture rules", r.status === 0, r.stderr);

    // The rules refuse what would break kernel 6's guarantees.
    const ordersQueries = join(lib, "modules/orders/api/infrastructure/queries/orders.sql");
    const ordersService = join(lib, "modules/orders/api/application/service.go");
    const ordersRepo = join(lib, "modules/orders/api/infrastructure/repository.go");
    const ordersMigration = join(lib, "modules/orders/migrations", readdirSync(join(lib, "modules/orders/migrations")).find((f) => f.endsWith(".up.sql")));
    const original = new Map([ordersQueries, ordersService, ordersRepo, ordersMigration].map((f) => [f, readFileSync(f, "utf8")]));
    const queriesSrc = original.get(ordersQueries);
    const serviceSrc = original.get(ordersService);
    const repoSrc = original.get(ordersRepo);
    for (const [label, file, src, rule] of [
      ["a query on a scoped table without company_id", ordersQueries, queriesSrc + "\n-- name: AllOrders :many\nSELECT id FROM orders;\n", "scoped-queries"],
      ["company_id named, but not in the WHERE", ordersQueries, queriesSrc + "\n-- name: OrderByID :one\nSELECT id, company_id FROM orders WHERE id = ?;\n", "scoped-queries"],
      ["company_id only in a subquery", ordersQueries, queriesSrc + "\n-- name: Leak :many\nSELECT o.id FROM orders o WHERE o.id IN (SELECT id FROM orders WHERE company_id = ?);\n", "scoped-queries"],
      ["an INSERT … SELECT copying from every company", ordersQueries, queriesSrc + "\n-- name: CopyOrder :exec\nINSERT INTO orders (company_id, sku, qty, created_at) SELECT ?, sku, qty, created_at FROM orders WHERE id = ?;\n", "scoped-queries"],
      ["one SELECT of a UNION without company_id", ordersQueries, queriesSrc + "\n-- name: Both :many\nSELECT id FROM orders WHERE company_id = ? UNION ALL SELECT id FROM orders WHERE sku = ?;\n", "scoped-queries"],
      ["company_id only in the ORDER BY", ordersQueries, queriesSrc + "\n-- name: Sorted :many\nSELECT id FROM orders WHERE id > ? ORDER BY company_id;\n", "scoped-queries"],
      ["a column unique across companies", ordersMigration, original.get(ordersMigration) + "\nALTER TABLE orders ADD COLUMN code VARCHAR(20) NOT NULL UNIQUE;\n", "scoped-tables"],
      ["a unique index across companies", ordersMigration, original.get(ordersMigration) + "\nCREATE UNIQUE INDEX orders_code ON orders (sku);\n", "scoped-tables"],
      ["a business key unique across companies", ordersMigration, original.get(ordersMigration) + "\nALTER TABLE orders ADD UNIQUE KEY orders_sku (sku);\n", "scoped-tables"],
      ["a query across every company without a System… name", ordersQueries, queriesSrc + "\n-- name: PurgeOrders :exec\n-- all companies: housekeeping\nDELETE FROM orders WHERE id = ?;\n", "scoped-queries"],
      ["a module opening its own transaction", ordersService, serviceSrc + "\nfunc leak(db interface{ BeginTx() }) { db.BeginTx() }\n", "tx-aware"],
      ["SQL on the pool, beside d.Tx", ordersService, serviceSrc + '\nfunc raw(d *kernel.Deps) { _, _ = d.DB.ExecContext(context.Background(), "DELETE FROM orders") }\n', "tx-aware"],
      ["the kernel's DB under another name", ordersService, serviceSrc + '\nfunc raw(deps *kernel.Deps) { _ = deps.DB }\n', "tx-aware"],
      ["SQL through Exec without a context", ordersRepo, repoSrc + '\nfunc wipe(db interface{ Exec(string, ...any) (any, error) }) { _, _ = db.Exec("DELETE FROM orders") }\n', "tx-aware"],
      ["queries built on the pool through an aliased sqlc", ordersRepo, repoSrc.replace('"__GO_MODULE__/internal/modules/orders/infrastructure/sqlc"', 'q "__GO_MODULE__/internal/modules/orders/infrastructure/sqlc"') + "\nvar pool = q.New(nil)\n", "tx-aware"],
      ["publishing an undeclared event", ordersService, serviceSrc.replace('"orders.order.placed"', '"orders.order.shipped"'), "events-declared"],
      ["an event named by a variable, which no rule can check", ordersService, serviceSrc.replace('"orders.order.placed"', "eventName"), "events-declared"],
      ["publishing through another name for d.Events", ordersService, serviceSrc + '\nfunc shout(ctx context.Context, d *kernel.Deps) error { bus := d.Events; return bus.Publish(ctx, "orders.order.shipped", 1, nil) }\n', "events-declared"],
      ["publishing with a scope set inline", ordersService, serviceSrc + '\nfunc tell(ctx context.Context, d *kernel.Deps) error { return d.Events.Publish(kernel.WithScope(ctx, kernel.Scope{CompanyID: 2}), "orders.order.shipped", 1, nil) }\n', "events-declared"],
      ["a conn taken before d.Tx.Do and used inside it", ordersRepo, repoSrc + '\nfunc (r *Repository) both(ctx context.Context) error {\n\tconn := r.tx.Conn(ctx)\n\treturn r.tx.Do(ctx, func(ctx context.Context) error {\n\t\t_, err := sqlc.New(conn).CountOrders(ctx, 1)\n\t\treturn err\n\t})\n}\n', "tx-aware"],
      ["publishing through an injected publisher", ordersService, serviceSrc + '\nfunc tell(ctx context.Context, bus interface{ Publish(context.Context, string, int, any) error }) error { return bus.Publish(ctx, "orders.order.shipped", 1, nil) }\n', "events-declared"],
      ["SQL on the pool behind a \"/*\" inside a string", ordersRepo, repoSrc + '\nvar accept = "image/*"\n\nfunc wipe(ctx context.Context, db interface{ ExecContext(context.Context, string, ...any) (any, error) }) { _, _ = db.ExecContext(ctx, "DELETE FROM orders") }\n\n/* end */\n', "tx-aware"],
      ["an event version other than the declared one", ordersService, serviceSrc.replace('"orders.order.placed", 1,', '"orders.order.placed", 2,'), "events-declared"],
    ]) {
      writeFileSync(file, src);
      r = check();
      expect(`refused: ${label} (${rule})`, r.status === 1 && r.stderr.includes(rule), r.stderr);
      writeFileSync(file, original.get(file));
    }
    // …and accept what is fine: SQL on a variable holding d.Tx.Conn(ctx); "d.DB" in a comment.
    writeFileSync(ordersRepo, repoSrc + '\n// never d.DB\nfunc (r *Repository) touch(ctx context.Context) error {\n\tconn := r.tx.Conn(ctx)\n\t_ = sqlc.New(conn)\n\t_, err := conn.ExecContext(ctx, "SELECT 1 -- not d.DB")\n\treturn err\n}\n');
    writeFileSync(ordersQueries, queriesSrc + "\n-- name: OrderOf :one\nSELECT id FROM orders WHERE (company_id = ? AND id = ?);\n");
    writeFileSync(ordersMigration, original.get(ordersMigration) + "\nALTER TABLE orders ADD UNIQUE KEY orders_sku (company_id, sku(20));\n");
    r = check();
    expect("accepted: SQL and sqlc.New on a variable holding d.Tx.Conn(ctx), d.DB in a comment or string, a parenthesized company filter, a per-company key with a prefix length", r.status === 0, r.stderr);
    writeFileSync(ordersRepo, repoSrc);
    writeFileSync(ordersQueries, queriesSrc);
    writeFileSync(ordersMigration, original.get(ordersMigration));
    expect("restored fixtures pass again", check().status === 0);

    installModule(lib, dir, "org6");
    installModule(lib, dir, "ledger");
    installModule(lib, dir, "orders", { ports: { Stock: "ledger" } });
    const reg = readFileSync(join(dir, "apps/api/internal/modules/registry.go"), "utf8");
    expect("the Go kit came along with the module that declares it", existsSync(join(dir, "apps/api/internal/shared/fixkit/fixkit.go")) && JSON.parse(readFileSync(join(dir, "modules.lock.json"), "utf8")).modules.fixkit);
    expect("the event subscription is wired in the composition root from \"listens\"",
      reg.includes('d.Events.On("orders.order.placed", "ledger.note-order", ledgerModule.OnOrderPlaced)'), reg);
    if (!build(dir)) return;
    if (!DB_DSN) {
      skip("kernel6: API checks against MySQL — set TESTBED_DB_DSN");
      return;
    }
    const db = parseDsn(DB_DSN);
    const name = `elco_testbed_k6_${Date.now()}`;
    try {
      const dsn = freshDatabase(dir, db, name);
      // An hour ago in the app's time zone: both containers find the tick missed and catch it up.
      const wib = new Date(Date.now() + 7 * 3600 * 1000 - 3600 * 1000);
      const at = `${String(wib.getUTCHours()).padStart(2, "0")}:${String(wib.getUTCMinutes()).padStart(2, "0")}`;
      const env = { DB_DSN: dsn, APP_TIMEZONE: "Asia/Jakarta", LEDGER_DAILY_AT: at };
      // The job ran in an earlier deploy (a job never catches up a tick from before it existed).
      mysqlQuery(db, "INSERT INTO kernel_schedules (name, spec, created_at, updated_at) VALUES ('ledger.daily-note', '', UTC_TIMESTAMP(6) - INTERVAL 2 DAY, UTC_TIMESTAMP(6))", name);
      api = await startApi(dir, 18097, env);
      api2 = await startApi(dir, 18098, env);
      const as = (user, scope) => ({ "X-Test-User": String(user), ...(scope ? { "X-Scope": scope } : {}) });
      const c = client(18097);
      const q = (sql) => mysqlQuery(db, sql, name);

      expect("stock added in company 1", (await c.call("POST", "/api/v1/ledger/stock", { sku: "A", qty: 5 }, as(1))).status === 201);
      r = await c.call("POST", "/api/v1/orders", { sku: "A", qty: 3 }, as(1));
      expect("an order takes its stock through the port, in one transaction", r.status === 201 && q("SELECT qty FROM ledger_stock WHERE company_id = 1 AND sku = 'A'") === "2", r.body);
      r = await c.call("POST", "/api/v1/orders", { sku: "A", qty: 9 }, as(1));
      expect("not enough stock: the order is refused and nothing is left behind (atomic across modules)",
        r.status === 409 && q("SELECT COUNT(*) FROM orders") === "1" && q("SELECT qty FROM ledger_stock WHERE company_id = 1 AND sku = 'A'") === "2", r.body);
      expect("the refused order published nothing", q("SELECT COUNT(*) FROM kernel_tasks WHERE kind = 'event:ledger.note-order'") === "1");
      const note = await waitFor(() => q("SELECT body FROM ledger_notes WHERE body LIKE 'placed:%'") || undefined, 15000);
      expect("the event reached its subscriber in another module, through a task", note === "placed:1:ORD-000001", note);
      expect("…exactly once, across two running containers", q("SELECT COUNT(*) FROM ledger_notes WHERE body LIKE 'placed:%'") === "1" &&
        q("SELECT COUNT(*) FROM kernel_tasks WHERE kind = 'event:ledger.note-order' AND status = 'done'") === "1");

      expect("company 2: its own stock", (await c.call("POST", "/api/v1/ledger/stock", { sku: "A", qty: 7 }, as(1, "2"))).status === 201);
      expect("company 2 sees none of company 1's orders", (await c.call("GET", "/api/v1/orders/count", undefined, as(1, "2"))).body?.data?.count === 0);
      expect("company 1 sees its order", (await c.call("GET", "/api/v1/orders/count", undefined, as(1))).body?.data?.count === 1);
      expect("company 2's stock is apart from company 1's", q("SELECT qty FROM ledger_stock WHERE company_id = 2 AND sku = 'A'") === "7");
      expect("a company the user is not placed in → 403", (await c.call("GET", "/api/v1/orders/count", undefined, as(2, "2"))).status === 403);
      expect("a branch the user is placed in is accepted", (await c.call("GET", "/api/v1/orders/count", undefined, as(2, "1:5"))).status === 200);

      const daily = await waitFor(() => (q("SELECT COUNT(*) FROM ledger_notes WHERE body = 'daily'") !== "0" ? "ok" : undefined), 15000);
      await new Promise((res) => setTimeout(res, 1500));
      expect("a missed daily tick is caught up — once, though two containers started",
        daily === "ok" && q("SELECT COUNT(*) FROM ledger_notes WHERE body = 'daily'") === "1" && q("SELECT COUNT(*) FROM kernel_schedule_runs WHERE name = 'ledger.daily-note' AND ok = TRUE") === "1");
    } finally {
      stopApi(api);
      stopApi(api2);
      try { mysql(db, `DROP DATABASE IF EXISTS ${name};`); } catch {}
    }
  } catch (err) {
    fail(`kernel6: ${err.message}`);
  } finally {
    stopApi(api);
    stopApi(api2);
    if (!KEEP) {
      rmSync(dir, { recursive: true, force: true });
      rmSync(lib, { recursive: true, force: true });
    }
  }
}

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
    copyLibrary(lib);
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

    // A promoted module waits for its testbed scenario: --check (the hook) refuses it until then.
    let gate = spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs"), "--check"], { cwd: lib, encoding: "utf8" });
    expect("…but cannot be committed before a testbed scenario installs it (testbed-covered)", gate.status === 1 && gate.stderr.includes("testbed-covered") && gate.stderr.includes("dashboard"), gate.stderr);
    rmSync(join(lib, "modules/dashboard"), { recursive: true, force: true });
    spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs")], { cwd: lib, encoding: "utf8" });

    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "promoted, then withdrawn");
    const check = () => spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs"), "--check", "--since", "HEAD"], { cwd: lib, encoding: "utf8" });

    // The library's own documents move with it.
    expect("the library copy passes its checks", check().status === 0, check().stderr);
    const statusFile = join(lib, "STATUS.md");
    const statusSrc = readFileSync(statusFile, "utf8");
    const usersReadme = join(lib, "modules/users/README.md");
    bump("9.0.0");
    edit(usersReadme, "## Changes\n", "## Changes\n\n- **9.0.0** — testbed.\n");
    spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs")], { cwd: lib, encoding: "utf8" });
    r = check();
    expect("a release without a STATUS.md entry is refused (status-current)", r.status === 1 && r.stderr.includes("status-current") && r.stderr.includes("users is now 9.0.0"), r.stderr);
    edit(statusFile, "## Log\n", "## Log\n\n- **today — something else.** testbed.\n");
    r = check();
    expect("…and still is when STATUS.md changed but does not name users 9.0.0", r.status === 1 && r.stderr.includes("users 9.0.0 released without a new line"), r.stderr);
    edit(statusFile, "something else", "users 9.0.0");
    r = check();
    expect("…and passes once STATUS.md names it with its version", r.status === 0, r.stderr);
    writeFileSync(statusFile, readFileSync(statusFile, "utf8").replace("| 9.0.0 |", "| 9.9.9 |"));
    r = check();
    expect("a hand-edited snapshot in STATUS.md is refused (stale)", r.status === 1 && r.stderr.includes("Stale: STATUS.md"), r.stderr);
    g("checkout", "--", "modules/users", "STATUS.md", "CATALOG.md", "catalog.json");
    const agentsFile = join(lib, "AGENTS.md");
    const agentsSrc = readFileSync(agentsFile, "utf8");
    writeFileSync(agentsFile, agentsSrc.split("\n").filter((l) => !l.startsWith("| `exact-numbers` |")).join("\n"));
    r = check();
    expect("a rule missing from AGENTS.md's table is refused (rules-documented)", r.status === 1 && r.stderr.includes("rules-documented") && r.stderr.includes("exact-numbers"), r.stderr);
    writeFileSync(agentsFile, agentsSrc);
    expect("restored, the copy passes again", check().status === 0 && readFileSync(statusFile, "utf8") === statusSrc);

    // A work branch: one change against main, coverage and status only warn, nothing tagged.
    g("switch", "-q", "-c", "work");
    cpSync(join(lib, "modules/money"), join(lib, "modules/wip-kit"), { recursive: true });
    const wipManifest = join(lib, "modules/wip-kit/module.json");
    writeFileSync(wipManifest, readFileSync(wipManifest, "utf8").replace('"id": "money"', '"id": "wip-kit"').replace("internal/shared/money", "internal/shared/wip-kit").replace(/"version": "[^"]+"/, '"version": "0.1.0"'));
    for (const f of readdirSync(join(lib, "modules/wip-kit/lib"))) {
      const file = join(lib, "modules/wip-kit/lib", f);
      writeFileSync(file, readFileSync(file, "utf8").replace(/^package money\b/m, "package wipkit"));
    }
    spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs")], { cwd: lib, encoding: "utf8" });
    const branchCheck = (...a) => spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs"), "--check", "--since", "main", ...a], { cwd: lib, encoding: "utf8" });
    r = branchCheck("--branch-work");
    expect("on a work branch a module without its scenario and STATUS.md line is committed with warnings", r.status === 0 && r.stderr.includes("testbed-covered") && r.stderr.includes("status-current"), r.stderr);
    r = branchCheck();
    expect("…which its pull request's check (without --branch-work) refuses", r.status === 1 && r.stderr.includes("testbed-covered"), r.stderr);
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "wip");
    r = spawnSync(process.execPath, [join(lib, "scripts/tags.mjs")], { cwd: lib, encoding: "utf8" });
    expect("…and nothing is tagged off main", r.status === 0 && !g("tag", "-l", "wip-kit/*").stdout.trim(), r.stdout);
    g("switch", "-q", "main");

    // The pinned kernel: read-only from the library's side.
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
    edit(join(lib, "STATUS.md"), "## Log\n", "## Log\n\n- **today — kernel synced again.**\n");
    r = check();
    expect("a kernel update mixed with a module change is refused (kernel-alone)", r.status === 1 && r.stderr.includes("kernel-alone") && !r.stderr.includes("pinned-kernel"), r.stderr);
    g("checkout", "--", "modules/users/README.md");
    r = check();
    expect("the same kernel update on its own, with its STATUS.md log line, passes", r.status === 0, r.stderr);
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

// organization: a running app with every module gains several companies — organization added
// after the others, the late ports of users and rbac wired with module.mjs — and then works per
// company and branch against MySQL, with no route or job left reading a scope it has not got.
async function organization() {
  log("\n=== organization: several companies and branches, added to a running app ===");
  const lib = mkdtempSync(join(LIB, ".testbed", "liborg-"));
  const dir = scaffold("organization");
  const g = (...a) => spawnSync("git", ["-C", lib, ...a], { encoding: "utf8" });
  const cli = (...a) => spawnSync(process.execPath, [join(lib, "scripts/module.mjs"), ...a, "--project", dir], { encoding: "utf8" });
  let api;
  try {
    copyLibrary(lib);
    g("init", "-q", "-b", "main");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "library");
    applyPreset(lib, dir, "bingkai-navy");
    let installed;
    for (const [id, opts] of [
      ["app-shell"], ["files"], ["mailer"], ["users"],
      ["auth", { ports: { UserStore: "users", PasswordWriter: "users", Mailer: "mailer" } }],
      ["rbac", { ports: { UserDirectory: "users" } }],
      ["settings", { ports: { FileStore: "files" } }],
      ["organization", { ports: { UserDirectory: "users" } }],
    ]) {
      installed = installModule(lib, dir, id, opts);
    }
    expect("installing organization names the optional ports waiting for it",
      ["users's optional port Placements", "rbac's optional port Placements"].every((n) => installed.notes.some((x) => x.includes(n))), installed.notes);
    const registry = join(dir, "apps/api/internal/modules/registry.go");
    const before = readFileSync(registry, "utf8");
    let r = cli("upgrade", "users", "--port", "Placements=organization");
    expect("module.mjs wires users' late Placements port to organization (installed after it)", r.status === 0, r.stdout + r.stderr);
    r = cli("upgrade", "rbac", "--port", "Placements=organization");
    expect("…and rbac's", r.status === 0, r.stdout + r.stderr);
    const reg = readFileSync(registry, "utf8");
    const end = reg.indexOf("// elco:modules:end");
    expect("the late lines follow every module's block — users' own block stays where it was",
      reg.indexOf("usersModule.UsePlacements(usersPlacements{organization: organizationModule.Client()})") > reg.indexOf("organization.New(")
      && reg.indexOf("rbacModule.UsePlacements(rbacPlacements{organization: organizationModule.Client()})") > reg.indexOf("organization.New(")
      && reg.indexOf("rbacModule.UsePlacements(rbacPlacements{organization: organizationModule.Client()})") < end
      && reg.indexOf("usersModule, err := users.New(d)") === before.indexOf("usersModule, err := users.New(d)"), reg);
    expect("rbac hears organization.company.created through the composition root",
      reg.includes('d.Events.On("organization.company.created", "rbac.appoint-founder", rbacModule.OnCompanyCreated)'));
    r = cli("remove", "organization");
    expect("removing organization is refused while users and rbac are wired to it", r.status === 1 && r.stderr.includes("users"), r.stderr);
    // A module installed after the late lines: they move back after its block.
    installModule(lib, dir, "audit-log", { ports: { UserDirectory: "users" } });
    // A release whose build line changes, upgraded while rewiring (its block moves to the end):
    // its own late line must follow it again.
    const rbacManifest = join(lib, "modules/rbac/module.json");
    const rm = JSON.parse(readFileSync(rbacManifest, "utf8"));
    rm.version = "0.6.1";
    rm.ports.push({ name: "Extra", required: false, description: "testbed: a port a later release adds", adapters: {} });
    rm.register.go.build[0] = "rbacModule, err := rbac.New(d, {{port:UserDirectory}}, {{port:Extra}})";
    writeFileSync(rbacManifest, JSON.stringify(rm, null, 2) + "\n");
    const rbacGo = join(lib, "modules/rbac/api/rbac.module.go");
    writeFileSync(rbacGo, readFileSync(rbacGo, "utf8").replace("func New(d *kernel.Deps, users ports.UserDirectory) (*Module, error) {", "func New(d *kernel.Deps, users ports.UserDirectory, _ ...any) (*Module, error) {"));
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qam", "rbac 0.6.1");
    r = cli("upgrade", "rbac", "--port", "Extra=expr:struct{}{}");
    const moved = readFileSync(registry, "utf8");
    expect("an upgrade moving a block to the end keeps every late line after every block",
      r.status === 0 && moved.indexOf("rbac.New(d,") > moved.indexOf("auditlog.New(")
      && moved.indexOf("rbacModule.UsePlacements(") > moved.indexOf("rbac.New(d,") && moved.indexOf("usersModule.UsePlacements(") > moved.indexOf("rbac.New(d,"),
      r.stdout + r.stderr + moved.slice(moved.indexOf("elco:modules:begin")));
    if (!build(dir)) return;
    if (!DB_DSN) {
      skip("organization: API checks against MySQL — set TESTBED_DB_DSN");
      return;
    }
    const db = parseDsn(DB_DSN);
    const name = `elco_testbed_org_${Date.now()}`;
    try {
      const dsn = freshDatabase(dir, db, name);
      const port = 18099;
      api = await startApi(dir, port, { DB_DSN: dsn, USERS_BOOTSTRAP_EMAIL: "admin@example.com", USERS_BOOTSTRAP_PASSWORD: "admin-pass-123", APP_TIMEZONE: "Asia/Jakarta" });
      const q = (sql) => mysqlQuery(db, sql, name);
      const signIn = async (email, password) => {
        const c = client(port);
        const res = await c.call("POST", "/api/v1/auth/login", { email, password });
        expect(`${email} signs in`, res.status === 200, res.body);
        return c;
      };
      const admin = await signIn("admin@example.com", "admin-pass-123");
      const at = (scope) => (scope ? { "X-Scope": scope } : {});

      let me = await admin.call("GET", "/api/v1/organization/me");
      expect("installing organization changes nothing: the admin works in company 1, named after the app",
        me.status === 200 && me.body.data.current.company_id === 1 && me.body.data.units[0]?.company_name === "Testbed App", me.body);
      expect("every existing user was placed across company 1", q("SELECT COUNT(*) FROM org_placements WHERE company_id = 1 AND branch_id = 0") === "1");
      const logo = await admin.call("GET", "/api/v1/settings/company/logo");
      expect("the public logo route chooses its company (no scope to read) → 404, not 500", logo.status === 404, logo.body);

      // A second company: its creator is placed there and becomes its Administrator.
      let res = await admin.call("POST", "/api/v1/organization/companies", { code: "pt-b", name: "PT B", timezone: "Asia/Makassar" });
      expect("company B created", res.status === 201 && res.body.data.code === "PT-B", res.body);
      const b = res.body.data.id;
      const founder = await waitFor(async () => {
        const p = await admin.call("GET", "/api/v1/rbac/me", undefined, at(String(b)));
        return p.status === 200 && p.body.data.permissions.includes("users.manage") ? "ok" : undefined;
      }, 15000);
      expect("its creator becomes its Administrator (organization.company.created → rbac)", founder === "ok",
        founder === "ok" ? undefined : mysqlQuery(db, "SELECT CONCAT_WS(' | ', kind, status, attempts, last_error) FROM kernel_tasks ORDER BY id DESC LIMIT 1", name));
      res = await admin.call("POST", "/api/v1/organization/companies", { code: "PT-C", name: "PT C", timezone: "Asia/Jakarta" }, at(String(b)));
      expect("companies are managed only from the home company → 403 from inside B", res.status === 403, res.body);

      // Isolation: settings and users per company.
      await admin.call("PUT", "/api/v1/settings/company", { name: "PT B Legal", legal_name: "", address: "", phone: "", email: "", tax_id: "", timezone: "Asia/Makassar" }, at(String(b)));
      const s1 = await admin.call("GET", "/api/v1/settings/company");
      const sB = await admin.call("GET", "/api/v1/settings/company", undefined, at(String(b)));
      expect("each company keeps its own profile", s1.body.data?.name !== "PT B Legal" && sB.body.data?.name === "PT B Legal" && sB.body.data?.company_id === b, [s1.body, sB.body]);
      res = await admin.call("POST", "/api/v1/users", { name: "Budi B", email: "budi@b.test", password: "budi-pass-123" }, at(String(b)));
      expect("a user created in company B", res.status === 201, res.body);
      const budi = res.body.data.id;
      expect("…is placed in B in the same transaction", q(`SELECT COUNT(*) FROM org_placements WHERE user_id = ${budi} AND company_id = ${b} AND branch_id = 0 AND is_default = TRUE`) === "1");
      const list1 = await admin.call("GET", "/api/v1/users");
      const listB = await admin.call("GET", "/api/v1/users", undefined, at(String(b)));
      expect("company 1's users page does not list B's people; B's lists them",
        !list1.body.data.some((u) => u.id === budi) && listB.body.data.some((u) => u.id === budi), [list1.body.meta, listB.body.meta]);
      expect("B's user is not found from company 1", (await admin.call("GET", `/api/v1/users/${budi}`)).status === 404);

      // Denials, and the user's own default.
      const bc = await signIn("budi@b.test", "budi-pass-123");
      me = await bc.call("GET", "/api/v1/organization/me");
      expect("a user works where they are placed: B, in its time zone", me.status === 200 && me.body.data.current.company_id === b && me.body.data.current.timezone === "Asia/Makassar", me.body);
      expect("…and is refused company 1 (403)", (await bc.call("GET", "/api/v1/organization/me", undefined, at("1"))).status === 403);
      expect("…and a malformed scope (403)", (await bc.call("GET", "/api/v1/organization/me", undefined, at("1;DROP"))).status === 403);

      // Company B's own Administrator cannot claim company 1's accounts.
      res = await admin.call("POST", "/api/v1/users", { name: "Citra A", email: "citra@a.test", password: "citra-pass-123" });
      const citra = res.body.data?.id;
      const adminRole = (await admin.call("GET", "/api/v1/rbac/roles", undefined, at(String(b)))).body.data.find((r) => r.is_system).id;
      res = await admin.call("POST", `/api/v1/rbac/roles/${adminRole}/members`, { user_id: budi }, at(String(b)));
      expect("Budi made Administrator of B only", res.status === 201, res.body);
      await new Promise((resolve) => setTimeout(resolve, 300));
      res = await bc.call("POST", "/api/v1/organization/placements", { user_id: citra, branch_id: 0 });
      expect("B's Administrator cannot place company 1's account in B (to take it over) → 403", res.status === 403, res.body);
      res = await bc.call("POST", "/api/v1/organization/placements", { user_id: 999999, branch_id: 0 });
      expect("…nor learn whether an account exists: an unknown one answers the same 403", res.status === 403, res.body);
      res = await bc.call("PUT", `/api/v1/users/${citra}`, { name: "Citra A", email: "mine@b.test", password: "taken-over-123" });
      expect("…nor change it", res.status === 404, res.body);

      // A branch in another zone: its business date follows it.
      res = await admin.call("POST", "/api/v1/organization/branches", { code: "jyp", name: "Jayapura", timezone: "Asia/Jayapura", nitku: "0123456789012345000001" });
      expect("a branch with its own zone and NITKU", res.status === 201 && res.body.data.code === "JYP", res.body);
      const jyp = res.body.data.id;
      me = await admin.call("GET", "/api/v1/organization/me", undefined, at(`1:${jyp}`));
      expect("someone placed across company 1 enters its branch, in the branch's zone", me.status === 200 && me.body.data.current.branch_id === jyp && me.body.data.current.timezone === "Asia/Jayapura", me.body);
      expect("a branch of company 1 asked under company B → 403", (await admin.call("GET", "/api/v1/organization/me", undefined, at(`${b}:${jyp}`))).status === 403);
      res = await admin.call("POST", "/api/v1/organization/placements", { user_id: budi, branch_id: jyp });
      expect("B's user placed in company 1's Jayapura branch too", res.status === 201, res.body);
      expect("…enters that branch", (await bc.call("GET", "/api/v1/organization/me", undefined, at(`1:${jyp}`))).status === 200);
      expect("…but not company 1 as a whole", (await bc.call("GET", "/api/v1/organization/me", undefined, at("1"))).status === 403);
      res = await admin.call("PUT", "/api/v1/organization/me/default", { company_id: 1, branch_id: jyp });
      expect("a default must be one of the user's own placements → 404", res.status === 404, res.body);

      // A branch head manages the people who work in their branch — and no one else.
      res = await admin.call("POST", "/api/v1/organization/branches", { code: "BDG", name: "Bandung", timezone: "", nitku: "", address: "" });
      const bdg = res.body.data.id;
      res = await admin.call("POST", "/api/v1/rbac/roles", { name: "Kepala cabang", description: "", permissions: ["organization.read", "organization.manage", "users.read", "users.manage"] });
      const head = res.body.data.id;
      const kepala = (await admin.call("POST", "/api/v1/users", { name: "Kepala BDG", email: "kepala@a.test", password: "kepala-pass-123" }, at(`1:${bdg}`))).body.data.id;
      const staf = (await admin.call("POST", "/api/v1/users", { name: "Staf BDG", email: "staf@a.test", password: "staf-pass-123" }, at(`1:${bdg}`))).body.data.id;
      res = await admin.call("POST", `/api/v1/rbac/roles/${head}/members`, { user_id: kepala, branch_id: bdg });
      expect("users created in Bandung work there; one heads it", res.status === 201, res.body);
      const kc = await signIn("kepala@a.test", "kepala-pass-123");
      res = await kc.call("PUT", `/api/v1/users/${staf}`, { name: "Staf Bandung", email: "staf@a.test" });
      expect("the branch head changes the account of someone who works only in the branch", res.status === 200, res.body);
      res = await kc.call("PUT", `/api/v1/users/${citra}`, { name: "Citra A", email: "mine@a.test" });
      expect("…not one who works across the company", res.status === 403, res.body);
      res = await kc.call("POST", "/api/v1/organization/placements", { user_id: citra, branch_id: bdg });
      expect("…nor place them in the branch", res.status === 403, res.body);
      res = await kc.call("DELETE", `/api/v1/organization/placements/${staf}?branch=${bdg}`);
      expect("…but takes the branch's own staff out of it", res.status === 200, res.body);

      // Deactivating B: its people cannot work there; Budi still has Jayapura.
      res = await admin.call("POST", `/api/v1/organization/companies/${b}/deactivate`);
      expect("company B deactivated", res.status === 200, res.body);
      me = await bc.call("GET", "/api/v1/organization/me");
      expect("a deactivated company is no longer anyone's default — Budi falls back to Jayapura", me.status === 200 && me.body.data.current.branch_id === jyp, me.body);
      expect("…and B cannot be entered", (await bc.call("GET", "/api/v1/organization/me", undefined, at(String(b)))).status === 403);
      res = await admin.call("DELETE", `/api/v1/organization/placements/${budi}?branch=${jyp}`);
      expect("his last placement taken away", res.status === 200, res.body);
      const nowhere = await bc.call("GET", "/api/v1/auth/me");
      expect("placed nowhere: every request refused, saying why", nowhere.status === 403 && /belum ditempatkan/.test(nowhere.body.message ?? ""), nowhere.body);
      const unplaced = await admin.call("GET", "/api/v1/organization/unplaced");
      expect("…and he is listed for the home company's administrators to place", unplaced.status === 200 && unplaced.body.data.some((u) => u.id === budi), unplaced.body);
      res = await admin.call("POST", `/api/v1/organization/companies/${b}/placements`, { email: "citra@a.test" });
      expect("from the home company, someone is placed across another company by exact email (recovery)", res.status === 201, res.body);
      res = await admin.call("POST", `/api/v1/organization/companies/${b}/placements`, { email: "citra@a.test" }, at(`1:${jyp}`));
      expect("…only from the home company as a whole", res.status === 403, res.body);

      // Public routes and background work in a several-companies app: nothing reads a scope it
      // has not got (that would panic).
      expect("public: a failed sign-in → 401", (await client(port).call("POST", "/api/v1/auth/login", { email: "admin@example.com", password: "wrong-wrong" })).status === 401);
      const forgot = await client(port).call("POST", "/api/v1/auth/forgot", { email: "admin@example.com" });
      expect("public: forgot password → not a 5xx", forgot.status < 500, forgot.body);
      await new Promise((resolve) => setTimeout(resolve, 13000)); // every interval job runs within ~11 s of start
      expect("no panic in any job, task, or route", !/panic/i.test(api.output()), api.output().split("\n").filter((l) => /panic/i.test(l)).slice(0, 5));
    } finally {
      stopApi(api);
      api = undefined;
      try { mysql(db, `DROP DATABASE IF EXISTS ${name};`); } catch {}
    }
    // Unwired, organization can be removed — and the project still compiles.
    r = cli("upgrade", "users", "--port", "Placements=none");
    expect("--port Placements=none unwires users' late port", r.status === 0, r.stdout + r.stderr);
    r = cli("upgrade", "rbac", "--port", "Placements=none");
    expect("…and rbac's", r.status === 0, r.stdout + r.stderr);
    r = cli("remove", "organization");
    const left = readFileSync(registry, "utf8");
    expect("then organization can be removed (rbac's listener stays: the event simply never comes)",
      r.status === 0 && !left.includes("organizationModule") && !left.includes("/internal/modules/organization\""), r.stdout + r.stderr);
    run("go", ["vet", "./..."], join(dir, "apps/api"), "the project compiles without organization (go vet ./...)");
  } finally {
    stopApi(api);
    if (!KEEP) {
      rmSync(dir, { recursive: true, force: true });
      rmSync(lib, { recursive: true, force: true });
    }
  }
}

// The document foundations: money, docstate, numbering, period — through the fixture invoice
// module, against MySQL, with two containers posting at once.
async function documents() {
  log("\n=== documents: money, docstate, numbering, period — the posting recipe end to end ===");
  const lib = mkdtempSync(join(LIB, ".testbed", "libdoc-"));
  const dir = scaffold("documents");
  const g = (...a) => spawnSync("git", ["-C", lib, ...a], { encoding: "utf8" });
  const cli = (...a) => spawnSync(process.execPath, [join(lib, "scripts/module.mjs"), ...a, "--project", dir], { encoding: "utf8" });
  let api;
  let api2;
  try {
    copyLibrary(lib);
    cpSync(join(LIB, "testbed/fixtures/documents/fixinvoice"), join(lib, "modules/fixinvoice"), { recursive: true });
    g("init", "-q", "-b", "main");
    g("add", "-A");
    g("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "library + documents fixture");
    spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs")], { cwd: lib, encoding: "utf8" });
    const check = () => spawnSync(process.execPath, [join(lib, "scripts/catalog.mjs"), "--check"], { cwd: lib, encoding: "utf8" });
    let r = check();
    expect("the fixture (two Go kits, two required ports) passes the architecture rules", r.status === 0, r.stderr);
    const mig = join(lib, "modules/fixinvoice/migrations/001_create_fixinvoice.up.sql");
    const migSrc = readFileSync(mig, "utf8");
    writeFileSync(mig, migSrc + "\nALTER TABLE fix_stock ADD COLUMN weight DOUBLE NOT NULL DEFAULT 0;\n");
    r = check();
    expect("refused: a DOUBLE column (exact-numbers)", r.status === 1 && r.stderr.includes("exact-numbers"), r.stderr);
    writeFileSync(mig, migSrc + "\nALTER TABLE fix_stock ADD COLUMN lat DOUBLE NULL; -- float: a map coordinate\n");
    expect("accepted: a float column that says why", check().status === 0);
    writeFileSync(mig, migSrc);

    for (const [id, opts] of [
      ["app-shell"], ["users"],
      ["auth", { ports: { UserStore: "users", PasswordWriter: "users" } }],
      ["rbac", { ports: { UserDirectory: "users" } }],
      ["organization", { ports: { UserDirectory: "users" } }],
      ["audit-log", { ports: { UserDirectory: "users" } }],
      ["period"],
      ["numbering", { ports: { Units: "organization" } }],
      ["fixinvoice", { ports: { Numbers: "numbering", Periods: "period" } }],
    ]) installModule(lib, dir, id, opts);
    r = cli("upgrade", "users", "--port", "Placements=organization");
    expect("users' late Placements port wired to organization", r.status === 0, r.stdout + r.stderr);
    r = cli("upgrade", "rbac", "--port", "Placements=organization");
    expect("…and rbac's", r.status === 0, r.stdout + r.stderr);
    expect("the Go kits came along with the fixture that declares them",
      existsSync(join(dir, "apps/api/internal/shared/money/money.go")) && existsSync(join(dir, "apps/api/internal/shared/docstate/docstate.go")));
    expect("money's decimal library is pinned in go.mod", readFileSync(join(dir, "apps/api/go.mod"), "utf8").includes("github.com/govalues/decimal v0.1.36"));
    if (!build(dir)) return;
    if (!DB_DSN) {
      skip("documents: API checks against MySQL — set TESTBED_DB_DSN");
      return;
    }
    const db = parseDsn(DB_DSN);
    const name = `elco_testbed_doc_${Date.now()}`;
    try {
      const dsn = freshDatabase(dir, db, name);
      const env = { DB_DSN: dsn, USERS_BOOTSTRAP_EMAIL: "admin@example.com", USERS_BOOTSTRAP_PASSWORD: "admin-pass-123", APP_TIMEZONE: "Asia/Jakarta" };
      const [p1, p2] = [18101, 18102];
      api = await startApi(dir, p1, env);
      api2 = await startApi(dir, p2, env);
      const q = (sql) => mysqlQuery(db, sql, name);
      const signIn = async (port, email, password) => {
        const c = client(port);
        const res = await c.call("POST", "/api/v1/auth/login", { email, password });
        expect(`${email} signs in on :${port}`, res.status === 200, res.body);
        return c;
      };
      const admin = await signIn(p1, "admin@example.com", "admin-pass-123");
      const admin2 = await signIn(p2, "admin@example.com", "admin-pass-123");
      const at = (scope) => (scope ? { "X-Scope": scope } : {});
      const INV = "/api/v1/fixinvoice/invoices";
      const draft = async (c, date, lines, scope, discount = "0") => {
        const res = await c.call("POST", INV, { date, discount, lines }, at(scope));
        if (res.status !== 201) fail(`draft ${date}: ${JSON.stringify(res.body)}`);
        return res.body.data;
      };
      const post = (c, id, scope, query = "") => c.call("POST", `${INV}/${id}/post${query}`, undefined, at(scope));
      const one = [{ sku: "A", qty: "1", price: "1000" }];
      const wib = new Date(Date.now() + 7 * 3600 * 1000).toISOString().slice(0, 10);

      // Money: exact through JSON, Go, and DECIMAL columns; the discount split adds up.
      expect("stock added", (await admin.call("POST", "/api/v1/fixinvoice/stock", { sku: "A", qty: "1000" })).status === 201);
      let inv = await draft(admin, "2025-01-10", [{ sku: "A", qty: "3", price: "0.10" }, { sku: "A", qty: "1", price: "1000000.05" }, { sku: "A", qty: "2.5", price: "3333.33" }], undefined, "0.10");
      expect("line amounts are exact and rounded once, half-up (0.10 × 3 = 0.30; 3333.33 × 2.5 = 8333.325 → 8333.33)",
        inv?.lines?.[0]?.amount === "0.30" && inv.lines[2].amount === "8333.33" && inv.total === "1008333.58", inv);
      expect("the DECIMAL columns hold exactly that", q(`SELECT CONCAT(total, '|', discount) FROM fix_invoices WHERE id = ${inv.id}`) === "1008333.58|0.10");
      expect("the discount allocated over the lines adds up to the discount, to the sen",
        q(`SELECT SUM(discount_share) FROM fix_invoice_lines WHERE invoice_id = ${inv.id}`) === "0.10" &&
        q(`SELECT SUM(amount) - (SELECT discount FROM fix_invoices WHERE id = ${inv.id}) FROM fix_invoice_lines WHERE invoice_id = ${inv.id}`) === "1008333.58");
      let res = await admin.call("POST", INV, { date: "2025-01-10", discount: "0", lines: [{ sku: "A", qty: "1", price: "1.005" }] });
      expect("a price with three decimals is refused, naming the field — never rounded silently", res.status === 400 && /lines\.price/.test(res.body.message), res.body);

      // Numbering: previews take nothing; posting takes the next number.
      res = await admin.call("GET", "/api/v1/numbering/types");
      const type = res.body?.data?.types?.find((t) => t.doc_type === "fixinvoice.invoice");
      expect("the fixture's document type is declared, with a preview", res.status === 200 && type?.series?.[0]?.next?.endsWith("/0001"), res.body);
      res = await post(admin, inv.id);
      expect("posting gives INV/2025/01/0001 — from the document's date, not today", res.status === 200 && res.body.data.number === "INV/2025/01/0001" && res.body.data.status === "posted", res.body);
      const first = inv.id;

      // The lifecycle.
      res = await admin.call("PUT", `${INV}/${first}`, { date: "2025-01-10", discount: "0", lines: one });
      expect("a posted invoice cannot be edited (409, saying what to do)", res.status === 409 && /batalkan lalu buat revisinya/.test(res.body.message), res.body);
      expect("…nor deleted (409)", (await admin.call("DELETE", `${INV}/${first}`)).status === 409);

      // Gapless: a posting that fails after taking its number gives it back.
      const tooMuch = await draft(admin, "2025-01-11", [{ sku: "A", qty: "5000", price: "1" }]);
      res = await post(admin, tooMuch.id);
      expect("a posting refused for stock → 409, nothing taken", res.status === 409 && /Stok/.test(res.body.message), res.body);
      const failing = await draft(admin, "2025-01-11", one);
      res = await post(admin, failing.id, undefined, "?fail_after_number=1");
      expect("a posting that fails after taking INV/2025/01/0002 → 409", res.status === 409, res.body);
      expect("…and the number went back with the rollback (counter and record of numbers given)",
        q("SELECT last_number FROM num_counters WHERE company_id = 1 AND period_key = '2025-01'") === "1" &&
        q("SELECT COUNT(*) FROM num_issued WHERE company_id = 1 AND number = 'INV/2025/01/0002'") === "0");
      const third = await draft(admin, "2025-01-11", one);
      res = await post(admin, third.id);
      expect("the next posting gets that number: INV/2025/01/0002", res.body?.data?.number === "INV/2025/01/0002", res.body);

      res = await admin.call("POST", `${INV}/${first}/cancel`, { reason: "  " });
      expect("cancelling without a reason → 422", res.status === 422, res.body);
      res = await admin.call("POST", `${INV}/${first}/cancel`, { reason: "salah harga" });
      expect("cancelled with a reason", res.status === 200 && res.body.data.status === "cancelled" && res.body.data.cancel_reason === "salah harga", res.body);
      res = await admin.call("POST", `${INV}/${first}/amend`);
      const amended = res.body?.data;
      expect("amending starts a draft INV/2025/01/0001-1 pointing at the cancelled one",
        res.status === 200 && amended.status === "draft" && amended.number === "INV/2025/01/0001-1" && amended.amended_from === first && amended.revision === 1 && amended.lines.length === 3, res.body);
      expect("a cancelled invoice is amended once (409)", (await admin.call("POST", `${INV}/${first}/amend`)).status === 409);
      res = await post(admin, amended.id);
      expect("the amendment posts under its own number and takes none from the series",
        res.body?.data?.number === "INV/2025/01/0001-1" && q("SELECT last_number FROM num_counters WHERE company_id = 1 AND period_key = '2025-01'") === "2", res.body);

      // 50 postings at once, from two containers: 1…50, no duplicate, no gap.
      const feb = [];
      for (let i = 0; i < 50; i++) feb.push(await draft(admin, `2025-02-${String(1 + (i % 28)).padStart(2, "0")}`, one));
      const t0 = Date.now();
      const results = await Promise.all(feb.map((d, i) => post(i % 2 ? admin2 : admin, d.id)));
      const numbers = results.map((x) => x.body?.data?.number).filter(Boolean).sort();
      const want = Array.from({ length: 50 }, (_, i) => `INV/2025/02/${String(i + 1).padStart(4, "0")}`);
      expect(`50 concurrent postings from two containers → INV/2025/02/0001…0050, no duplicate, no gap (${Date.now() - t0} ms)`,
        results.every((x) => x.status === 200) && JSON.stringify(numbers) === JSON.stringify(want), results.filter((x) => x.status !== 200).map((x) => x.body).slice(0, 3));

      // Reset per month, per branch, and formats that would repeat a number.
      const mar = await draft(admin, "2025-03-01", one);
      expect("a new month starts again: INV/2025/03/0001", (await post(admin, mar.id)).body?.data?.number === "INV/2025/03/0001");
      res = await admin.call("PUT", "/api/v1/numbering/types/fixinvoice.invoice", { pattern: "INV/{SEQ:4}", reset: "yearly", per_branch: false });
      expect("a yearly reset without the year in the pattern is refused (422)", res.status === 422, res.body);
      res = await admin.call("PUT", "/api/v1/numbering/types/fixinvoice.invoice", { pattern: "INV/{YYYY}/{SEQ:4}", reset: "monthly", per_branch: true });
      expect("a per-branch format without {BRANCH} is refused (422)", res.status === 422, res.body);
      res = await admin.call("PUT", "/api/v1/numbering/types/fixinvoice.invoice", { pattern: "INV/{YYYY}/{MM}/{SEQ:4}", reset: "yearly", per_branch: false });
      expect("once numbers were given, a new reset (which would print INV/2025/01/0001 again) is refused (409)", res.status === 409, res.body);
      res = await admin.call("PUT", "/api/v1/numbering/types/fixinvoice.invoice", { pattern: "FJ-{YYYY}{MM}-{SEQ:4}", reset: "monthly", per_branch: false });
      expect("…but the pattern may change", res.status === 200, res.body);
      const marFj = await draft(admin, "2025-03-03", one);
      expect("the series continues under the new pattern: FJ-202503-0002", (await post(admin, marFj.id)).body?.data?.number === "FJ-202503-0002");
      expect("the format back to the module's default", (await admin.call("DELETE", "/api/v1/numbering/types/fixinvoice.invoice")).status === 200);

      // Closing the books.
      res = await admin.call("POST", "/api/v1/rbac/roles", { name: "Akuntan", description: "", permissions: ["period.read", "period.manage", "fixinvoice.manage"] });
      const accountantRole = res.body?.data?.id;
      res = await admin.call("POST", "/api/v1/users", { name: "Akuntan", email: "akuntan@example.com", password: "akuntan-pass-123" });
      expect("an accountant who may close but not reopen", res.status === 201 && (await admin.call("POST", `/api/v1/rbac/roles/${accountantRole}/members`, { user_id: res.body.data.id })).status === 201, res.body);
      await new Promise((resolve) => setTimeout(resolve, 300));
      const acc = await signIn(p1, "akuntan@example.com", "akuntan-pass-123");
      const lock = (c, area, through, reason = "", scope) => c.call("PUT", `/api/v1/period/locks/${encodeURIComponent(area)}`, { locked_through: through, reason }, at(scope));
      res = await lock(acc, "sales", "2025-01-31");
      expect("the accountant closes January for sales", res.status === 200, res.body);
      res = await post(acc, tooMuch.id);
      expect("posting a January invoice now → 409, naming the lock date", res.status === 409 && /31 Januari 2025/.test(res.body.message), res.body);
      res = await acc.call("POST", `${INV}/${third.id}/cancel`, { reason: "salah pelanggan" });
      expect("…and cancelling one → 409 (closed books do not change)", res.status === 409, res.body);
      const lateFeb = await draft(admin, "2025-02-20", one);
      res = await lock(acc, "*", "2025-02-28");
      expect("every area closed through February", res.status === 200, res.body);
      expect("a February invoice is refused by the company-wide lock", (await post(admin, lateFeb.id)).status === 409);
      res = await lock(acc, "sales", "2025-02-15");
      expect("an area lock earlier than the company-wide one is refused (422)", res.status === 422, res.body);
      res = await lock(acc, "*", "2025-01-31", "faktur Februari tertinggal");
      expect("reopening without period.unlock → 403", res.status === 403, res.body);
      res = await lock(admin, "*", "2025-01-31");
      expect("reopening without a reason → 422", res.status === 422, res.body);
      res = await lock(admin, "*", "2025-01-31", "faktur Februari tertinggal");
      expect("reopening with period.unlock and a reason", res.status === 200, res.body);
      expect("…recorded as a security event with its reason",
        q("SELECT COUNT(*) FROM audit_log WHERE module = 'period' AND action = 'lock.reopened' AND kind = 'security' AND JSON_UNQUOTE(JSON_EXTRACT(data, '$.reason')) = 'faktur Februari tertinggal'") === "1");
      res = await post(admin, lateFeb.id);
      expect("February posts again — continuing its series: INV/2025/02/0051", res.body?.data?.number === "INV/2025/02/0051", res.body);
      res = await lock(admin, "sales", wib);
      expect("a lock through today (or later) is refused (422)", res.status === 422, res.body);

      // A lock moved while a posting runs: the lock waits for the posting.
      const apr = await draft(admin, "2025-04-10", one);
      const tPost = Date.now();
      const posting = post(admin, apr.id, undefined, "?hold_ms=2000").then((x) => ({ ...x, done: Date.now() }));
      await new Promise((resolve) => setTimeout(resolve, 400));
      const moving = lock(acc, "sales", "2025-04-30").then((x) => ({ ...x, done: Date.now() }));
      const [pr, mv] = await Promise.all([posting, moving]);
      expect("the posting in flight finished, and the lock moved only after it committed",
        pr.status === 200 && mv.status === 200 && mv.done >= pr.done - 50 && pr.done - tPost >= 1900, { post: pr.status, lock: mv.status, gap: mv.done - pr.done });
      const apr2 = await draft(admin, "2025-04-11", one);
      expect("…and once moved, April is closed (409)", (await post(admin, apr2.id)).status === 409);

      // Another company: its own series, its own locks.
      res = await admin.call("POST", "/api/v1/organization/companies", { code: "PT-B", name: "PT B", timezone: "Asia/Jakarta" });
      const b = res.body?.data?.id;
      const founder = await waitFor(async () => {
        const p = await admin.call("GET", "/api/v1/rbac/me", undefined, at(String(b)));
        return p.status === 200 && p.body.data.permissions.includes("fixinvoice.manage") ? "ok" : undefined;
      }, 15000);
      expect("company B, with its creator as Administrator", founder === "ok");
      expect("stock in B", (await admin.call("POST", "/api/v1/fixinvoice/stock", { sku: "A", qty: "5" }, at(String(b)))).status === 201);
      res = await admin.call("POST", "/api/v1/organization/branches", { code: "SBY", name: "Surabaya", timezone: "", nitku: "", address: "" }, at(String(b)));
      const sby = res.body?.data?.id;
      expect("B's branch Surabaya", res.status === 201, res.body);
      res = await admin.call("PUT", "/api/v1/numbering/types/fixinvoice.invoice", { pattern: "INV/{BRANCH}/{YYYY}/{MM}/{SEQ:4}", reset: "monthly", per_branch: true }, at(String(b)));
      expect("B has given no number yet: it may count per branch (company 1's numbers are not B's)", res.status === 200, res.body);
      const inB = await draft(admin, "2025-01-15", one, String(b));
      res = await post(admin, inB.id, String(b));
      expect("B's January is open and counts from 1, under B's code: INV/PT-B/2025/01/0001", res.status === 200 && res.body.data.number === "INV/PT-B/2025/01/0001", res.body);
      const inSby = await draft(admin, "2025-01-15", one, `${b}:${sby}`);
      res = await post(admin, inSby.id, `${b}:${sby}`);
      expect("Surabaya counts on its own: INV/SBY/2025/01/0001", res.body?.data?.number === "INV/SBY/2025/01/0001", res.body);
      res = await admin.call("PUT", "/api/v1/numbering/types/fixinvoice.invoice/next", { branch_id: sby, next: 500, reason: "lanjut dari sistem lama" }, at(String(b)));
      expect("the next number of Surabaya's series raised (this period)", res.status === 200, res.body);
      res = await admin.call("PUT", "/api/v1/numbering/types/fixinvoice.invoice/next", { branch_id: sby, next: 10, reason: "turunkan nomor" }, at(String(b)));
      expect("…never lowered (422)", res.status === 422, res.body);
      res = await admin.call("PUT", "/api/v1/numbering/types/fixinvoice.invoice/next", { branch_id: sby, next: 9223372036854775000, reason: "salah ketik" }, at(String(b)));
      expect("…nor past the counter's limit (422)", res.status === 422, res.body);
      const todaySby = await draft(admin, wib, one, `${b}:${sby}`);
      res = await post(admin, todaySby.id, `${b}:${sby}`);
      expect("a posting today continues from it", res.body?.data?.number === `INV/SBY/${wib.slice(0, 4)}/${wib.slice(5, 7)}/0500`, res.body);
      expect("raising it is a security record", q("SELECT COUNT(*) FROM audit_log WHERE module = 'numbering' AND action = 'counter.raised' AND kind = 'security'") === "1");
      res = await admin.call("GET", "/api/v1/numbering/types", undefined, at(`${b}:${sby}`));
      expect("in a branch, the page offers no format change (that needs the whole company)", res.status === 200 && res.body.data.manage_formats === false, res.body);
      res = await admin.call("GET", "/api/v1/period/locks", undefined, at(String(b)));
      expect("B has none of company 1's locks", res.status === 200 && res.body.data.areas.every((a) => a.effective === null), res.body);
      expect("company 1's invoice is not found from B (404)", (await admin.call("GET", `${INV}/${first}`, undefined, at(String(b)))).status === 404);
      expect("no duplicate number anywhere", q("SELECT COUNT(*) - COUNT(DISTINCT company_id, number) FROM fix_invoices WHERE number IS NOT NULL") === "0");
      expect("no panic in either container", !/panic/i.test(api.output() + api2.output()));
    } finally {
      stopApi(api);
      stopApi(api2);
      api = api2 = undefined;
      try { mysql(db, `DROP DATABASE IF EXISTS ${name};`); } catch {}
    }
  } catch (err) {
    fail(`documents: ${err.message}`);
  } finally {
    stopApi(api);
    stopApi(api2);
    if (!KEEP) {
      rmSync(dir, { recursive: true, force: true });
      rmSync(lib, { recursive: true, force: true });
    }
  }
}

const scenarios = { full, "no-rbac": noRbac, "auth-alone": authAlone, "users-alone": usersAlone, lifecycle, kernel6, organization, documents };
for (const [name, fn] of Object.entries(scenarios)) {
  if (!ONLY || ONLY === name) await fn();
}

log("\n=== Summary ===");
for (const s of skipped) log(`  ⚠ skipped: ${s}`);
if (failures) {
  log(`FAILED: ${failures} check(s); ${passes} passed.`);
  process.exit(1);
}
// The line STATUS.md › Last verified records — with what was skipped, if anything.
log(`All testbed checks passed: ${passes} checks${ONLY ? ` (only ${ONLY})` : ""}${skipped.length ? `, ${skipped.length} skipped — not a full run` : ""}.`);

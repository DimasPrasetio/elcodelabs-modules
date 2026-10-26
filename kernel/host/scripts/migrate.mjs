#!/usr/bin/env node
// golang-migrate wrapper that works the same in PowerShell, cmd, and sh.
// npm runs scripts through cmd.exe on Windows, which does not expand $VAR — so the DB
// connection is read here from .env and passed as a literal argument instead.
import { readFileSync, existsSync } from "node:fs";
import { spawnSync } from "node:child_process";

if (existsSync(".env")) {
  for (const line of readFileSync(".env", "utf8").split(/\r?\n/)) {
    const m = line.match(/^\s*([A-Z0-9_]+)\s*=\s*(.*)\s*$/);
    if (m && !process.env[m[1]]) process.env[m[1]] = m[2];
  }
}

const dsn = process.env.DB_DSN;
if (!dsn) {
  console.error("DB_DSN not found. Copy .env.example to .env and fill in DB_DSN.");
  process.exit(1);
}

// multiStatements: a migration file may hold several statements (a table and its seed rows).
// No shell: the DSN's "tcp(host:port)" would be parsed as shell syntax.
const url = `mysql://${dsn}${dsn.includes("?") ? "&" : "?"}multiStatements=true`;
const r = spawnSync("migrate", ["-path", "migrations", "-database", url, ...process.argv.slice(2)], {
  cwd: "apps/api",
  stdio: "inherit",
});
if (r.error) console.error(`migrate: ${r.error.message} — install golang-migrate (references/backend-go.md)`);
process.exit(r.status ?? 1);

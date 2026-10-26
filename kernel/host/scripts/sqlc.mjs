#!/usr/bin/env node
// sqlc wrapper. sqlc refuses a config with no sql: block ("no packages") — the normal state
// of a project until a module with queries is installed — so skip instead of failing.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";

if (!/^\s*- engine:/m.test(readFileSync("apps/api/sqlc.yaml", "utf8"))) {
  console.log("sqlc: no module has queries yet — nothing to generate.");
  process.exit(0);
}
const r = spawnSync("sqlc", ["generate"], { cwd: "apps/api", stdio: "inherit", shell: true });
process.exit(r.status ?? 1);

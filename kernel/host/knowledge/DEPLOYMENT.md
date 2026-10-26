# Deployment — Testbed App

Owns this project's concrete deploy facts and procedure. The rules every deploy file must keep
are in `.claude/rules/deployment.md`; the server's own convention is the canonical
`/opt/elcodelabs/SERVER_PLAYBOOK.md` on the VPS (`ssh elcodelabs`) — re-read it before any
server change, it is updated as apps are added.

## Local
The dev commands live in `CLAUDE.md` (agents) and `README.md` (humans) — not repeated here.

`docker compose up --build` (root `docker-compose.yml`) builds the image from source on
your own machine for a quick check — never on the VPS. Set the DB env in `.env` first.

## Target: elcodelabs VPS

| Fact | Value |
|---|---|
| App name (slug) | `testbedapp` |
| Server folder | `~/testbedapp` (`docker-compose.prod.yml`, `deploy.sh`, `.env`, `backups/`) |
| Container | `testbedapp-app` |
| Port | `127.0.0.1:__VPS_PORT__` on the host → `8080` in the container |
| Database / user | `testbedapp_db` / `testbedapp_user@'172.%'` (`mysql_native_password`) |
| Image | `ghcr.io/__GHCR_OWNER__/testbed-app:<git-sha>` (`:latest` is also pushed but never deployed) |
| Domain | `<domain>` |

One image serves the static frontend and the API on port 8080. The container reaches the
host MySQL via `host.docker.internal` (`extra_hosts: host-gateway`). Users are created
with `mysql_native_password` because the app connects over the internal docker bridge
without TLS, which MySQL 8's default `caching_sha2_password` rejects (ERROR 2061).

## Release flow

merge to `main` → `ci` green → `release-image` pushes `ghcr.io/__GHCR_OWNER__/testbed-app:<git-sha>` →
on the VPS: `cd ~/testbedapp && ./deploy.sh <git-sha>`
(preflight → pull → check `.env` has every setting the image lists → dump the database to
`backups/pre-deploy.sql.gz` → migrate → up → wait for `/readyz`). Nothing changes before
the `.env` check passes.

**Rollback** = `./deploy.sh <older-git-sha>`. The schema stays where it is (migrations are
forward-only): deploy.sh sees the database is newer than that image and skips migrate, and the
older version runs on the newer schema — which works only because every release follows
expand/contract (`.claude/rules/database.md`).

## Backups and restore

- **Nightly, rotated** — the server's backup job (installed once per server, not per app;
  the skill's `scripts/server/`): `/var/backups/elcodelabs/testbedapp_db/{daily,weekly,monthly}/`,
  5 daily, 2 weekly, 1 monthly, test-restored on the 1st, failures reported to Telegram.
- **Before each migration** — `~/testbedapp/backups/pre-deploy.sql.gz`, replaced every deploy.

**Restore** (lost or damaged data):
1. `docker compose -f docker-compose.prod.yml stop app`
2. Pick the dump: `sudo ls -lt /var/backups/elcodelabs/testbedapp_db/*/` or `backups/pre-deploy.sql.gz`.
3. Check it in a scratch database first:
   `sudo mysql -e 'CREATE DATABASE testbedapp_db_check'` · `gunzip -c <dump> | sudo mysql testbedapp_db_check` ·
   look at the data · `sudo mysql -e 'DROP DATABASE testbedapp_db_check'`
4. Replace: `sudo mysql -e 'DROP DATABASE testbedapp_db; CREATE DATABASE testbedapp_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci'`,
   then `gunzip -c <dump> | sudo mysql testbedapp_db` (the user's grants on `testbedapp_db.*` survive the drop).
5. `./deploy.sh <git-sha>` with the version to run — it migrates the restored schema forward if needed.

## A migration failed

deploy.sh stops before the new version starts; the next deploy refuses while the database is
marked *dirty*. MySQL commits each DDL statement at once, so a file that failed half-way is
partly applied — that is why one migration file holds one schema change.
1. Read the error in the deploy output. `docker compose -f docker-compose.prod.yml --profile ops run --rm migrate version`
   shows `<version> (dirty)`.
2. Safe path: restore `backups/pre-deploy.sql.gz` (Restore, steps 1–4). Only if the failed
   statement left nothing behind: `… run --rm migrate force <previous-version>`.
3. Fix it in a **new** migration (never edit one that ran anywhere), release, deploy.

## First-time server setup (once)

Every step below changes the shared server — do it only with the owner's go-ahead.

1. **Port** — take the next free port from the App registry in the playbook and confirm it
   with `ssh elcodelabs 'sudo ss -tlnp'`.
2. **Database + dedicated user** on the host MySQL (password: `openssl rand -hex 24`):
   ```sql
   CREATE DATABASE testbedapp_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
   CREATE USER 'testbedapp_user'@'172.%' IDENTIFIED WITH mysql_native_password BY '<random-hex>';
   GRANT ALL PRIVILEGES ON testbedapp_db.* TO 'testbedapp_user'@'172.%';
   FLUSH PRIVILEGES;
   ```
3. **Server folder** — `mkdir -p ~/testbedapp`; copy `infra/elcodelabs/docker-compose.prod.yml`
   and `infra/elcodelabs/deploy.sh` into it, and `infra/elcodelabs/.env.production.example`
   as `.env`; fill in real values; `chmod 600 .env`.
4. **Registry access** — new GHCR packages are private: either `docker login ghcr.io` on the
   VPS with a read-only (`read:packages`) token, or set the package to Public.
5. **First deploy** (database still empty): `SKIP_BACKUP=1 ./deploy.sh <git-sha>`.
6. **nginx + TLS** — copy `infra/nginx/nginx.conf` to `/etc/nginx/sites-available/testbedapp`,
   `sudo ln -s /etc/nginx/sites-available/testbedapp /etc/nginx/sites-enabled/`,
   `sudo nginx -t && sudo systemctl reload nginx`, then `sudo certbot --nginx -d <domain>`
   (point the domain's DNS A record at the VPS first).
7. **Registry entry** — add this app's row to the playbook's App registry.
8. **Backups** — if this server has no `elcodelabs-backup.timer` yet
   (`systemctl list-timers | grep elcodelabs-backup`), install it once
   (references/docker-deployment.md › Backups in the skill). The new database is included from
   its first night.

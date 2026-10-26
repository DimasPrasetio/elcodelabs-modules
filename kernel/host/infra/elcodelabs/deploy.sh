#!/usr/bin/env bash
# Testbed App — deploy on the elcodelabs VPS (procedure: knowledge/DEPLOYMENT.md).
# The VPS NEVER builds: CI (release-image.yml) builds & pushes to GHCR; this script only does
#   preflight -> pull -> DB backup -> migrate -> up -> wait for /readyz.
#
# Usage (on the VPS, from ~/testbedapp):
#   ./deploy.sh <git-sha>         deploy that exact, immutable image
#   ./deploy.sh <older-git-sha>   rollback (migrations are forward-only)
#
# Optional env:
#   SKIP_BACKUP=1     skip the DB dump (FIRST deploy only, while the database is still empty)
#   SKIP_PULL=1       use an image already on the machine (production always pulls)
#   MYSQLDUMP="..."   dump command (default: sudo -n mysqldump — MySQL root via auth_socket)
#   SKIP_MIGRATE=1    do not migrate (Go detects a rollback past a schema change by itself)
#
# Scheduled, rotated backups are the server's job, not this script's (knowledge/DEPLOYMENT.md
# › Backups). This script keeps one dump: the state right before migrating.
set -euo pipefail
umask 077 # dumps and logs are readable by the owner only from the moment they exist
cd "$(dirname "$0")"

APP=testbedapp
PORT=__VPS_PORT__
IMAGE_REPO=ghcr.io/__GHCR_OWNER__/testbed-app
COMPOSE="docker compose -f docker-compose.prod.yml"
MYSQLDUMP="${MYSQLDUMP:-sudo -n mysqldump}"

TAG="${1:-}"
if [ -z "$TAG" ]; then
  echo "usage: ./deploy.sh <git-sha>   (tag shown in GitHub Actions » release-image)" >&2
  exit 2
fi
if [ "$TAG" = "latest" ]; then
  echo "refused: 'latest' cannot be rolled back to — use the full <git-sha>." >&2
  exit 2
fi

envval() { grep -E "^$1=" .env | tail -1 | cut -d= -f2-; }

echo "▶ [0/5] preflight"
if grep -qE '__[A-Z_]+__' "$0" docker-compose.prod.yml; then
  echo "  ✗ deploy facts still unset (placeholder markers left in deploy.sh / docker-compose.prod.yml)."
  echo "    Fill in the port and GHCR owner from knowledge/DEPLOYMENT.md first."
  exit 1
fi
[ -f .env ] || { echo "  ✗ .env missing (copy .env.production.example from the repo, fill real values)"; exit 1; }
perm="$(stat -c %a .env 2>/dev/null || stat -f %Lp .env)"
[ "$perm" = "600" ] || { echo "  ✗ .env must be chmod 600 (is $perm)"; exit 1; }
[ "$(envval APP_ENV)" = "production" ] || { echo "  ✗ APP_ENV in .env must be production"; exit 1; }
DB_NAME="$(envval DB_NAME)"
[ -n "$DB_NAME" ] || { echo "  ✗ DB_NAME is empty in .env"; exit 1; }
[ "$DB_NAME" = "${APP}_db" ] || echo "  ! DB_NAME is $DB_NAME, convention is ${APP}_db"
mkdir -p backups
# DATA_DIR on the server: the app runs as uid 10001 on a read-only filesystem, so this folder
# is the only place it can write. Created and handed over once; the check costs nothing after.
# Not fatal: an app that stores no files runs without it, and one that does refuses to start
# with a clear message, which the health check then reports.
mkdir -p data
if [ "$(stat -c %u data 2>/dev/null || stat -f %u data)" != "10001" ] && ! sudo -n chown 10001:10001 data 2>/dev/null; then
  echo "  ! data/ does not belong to uid 10001 and sudo is not allowed here; an app that stores files needs: sudo chown 10001:10001 ~/${APP}/data"
fi
PREV_TAG="$(envval IMAGE_TAG || true)"
echo "  ✓ .env 600, APP_ENV=production, DB_NAME=${DB_NAME}; previous version: ${PREV_TAG:-(none)}"

export IMAGE_TAG="$TAG"
IMAGE="${IMAGE_REPO}:${TAG}"

echo "▶ [1/5] pull ${IMAGE}"
if [ "${SKIP_PULL:-0}" = "1" ]; then
  docker image inspect "$IMAGE" >/dev/null || { echo "  ✗ SKIP_PULL=1 but the image is not on this machine"; exit 1; }
  echo "  (SKIP_PULL=1 — using the local image)"
else
  $COMPOSE pull app
fi
# Every setting this version needs is in .env — checked before anything changes. The keys come
# from the image's own copy of .env.production.example; one written as <...> there must have a
# value here.
missing=""
while IFS= read -r line; do
  key="${line%%=*}"
  example="${line#*=}"
  if ! grep -qE "^${key}=" .env; then
    missing="${missing} ${key}"
  elif [ "${example#<}" != "$example" ] && [ -z "$(envval "$key")" ]; then
    missing="${missing} ${key}(empty)"
  fi
done < <(docker run --rm --entrypoint cat "$IMAGE" /app/env.production.example 2>/dev/null | grep -E '^[A-Z][A-Z0-9_]*=' || true)
if [ -n "$missing" ]; then
  echo "  ✗ .env lacks settings this version needs:${missing}"
  echo "    Add them (see infra/elcodelabs/.env.production.example) and rerun. Nothing was changed."
  exit 1
fi

echo "▶ [2/5] back up ${DB_NAME} before migrating"
if [ "${SKIP_BACKUP:-0}" = "1" ]; then
  echo "  (SKIP_BACKUP=1 — skipped; first deploy on an empty database only)"
else
  # One dump, replaced each deploy: the state right before this migration. Written beside the
  # old one first, so a failed dump keeps the previous dump. pipefail: a failed dump fails
  # the deploy (never migrate without a backup).
  out="backups/pre-deploy.sql.gz"
  $MYSQLDUMP --single-transaction --routines --triggers --events --hex-blob \
    --default-character-set=utf8mb4 --set-gtid-purged=OFF --no-tablespaces "$DB_NAME" | gzip -9 > "$out.tmp"
  gzip -t "$out.tmp"
  mv -f "$out.tmp" "$out"
  rm -f backups/pre-deploy-*.sql.gz # dumps from older versions of this script
  echo "  ✓ ${out} ($(du -h "$out" | cut -f1)) — before ${TAG:0:12}"
fi

echo "▶ [3/5] migrate"
if [ "${SKIP_MIGRATE:-0}" = "1" ]; then
  echo "  (SKIP_MIGRATE=1 — skipped)"
else
  # Where the database is, and the newest migration this image ships.
  current="$($COMPOSE --profile ops run --rm migrate version 2>&1 | tail -1 || true)"
  newest="$($COMPOSE --profile ops run --rm --entrypoint sh migrate -c 'ls /app/migrations' 2>/dev/null \
    | sed -n 's/^\([0-9][0-9]*\)_.*\.up\.sql$/\1/p' | sort -n | tail -1 || true)"
  case "$current" in
    *dirty*)
      echo "  ✗ the database is marked dirty: a migration failed half-way (${current})."
      echo "    Nothing was started. Recovery: knowledge/DEPLOYMENT.md › A migration failed."
      exit 1 ;;
  esac
  cur="$(printf '%s' "$current" | grep -oE '^[0-9]+' || true)"
  if [ -n "$cur" ] && [ -n "$newest" ] && [ "$cur" -gt "$newest" ]; then
    # A rollback: this older image does not know the newer migrations. The schema stays —
    # releases follow expand/contract, so the previous version runs on it.
    echo "  ! database is at ${cur}, newer than this image (${newest}): rollback — migrate skipped"
  else
    $COMPOSE --profile ops run --rm migrate
  fi
fi

echo "▶ [4/5] start app"
# Keep the outgoing version's log: it disappears with its container, and is what a rollback
# investigation needs. The last 5 are kept.
if docker inspect "${APP}-app" >/dev/null 2>&1; then
  mkdir -p logs
  docker logs "${APP}-app" > "logs/${PREV_TAG:-previous}.log" 2>&1 || true
  ls -1t logs/*.log 2>/dev/null | tail -n +6 | xargs -r rm -f
fi
$COMPOSE up -d app
# Record the running version so a manual `docker compose up` stays on it.
if grep -qE '^IMAGE_TAG=' .env; then
  sed -i.bak -E "s|^IMAGE_TAG=.*|IMAGE_TAG=${TAG}|" .env && rm -f .env.bak
else
  printf '\n# managed by deploy.sh — image version currently running\nIMAGE_TAG=%s\n' "$TAG" >> .env
fi
chmod 600 .env

echo "▶ [5/5] wait for /readyz (max 120 s)"
for _ in $(seq 1 24); do
  if curl -fsS "http://127.0.0.1:${PORT}/readyz" >/dev/null 2>&1; then
    # Keep this version and the previous one for a quick rollback; older ones only fill the disk.
    docker images "$IMAGE_REPO" --format '{{.Tag}}' | grep -vxF -e "$TAG" -e "${PREV_TAG:-none}" \
      | while read -r old; do docker rmi "${IMAGE_REPO}:${old}" >/dev/null 2>&1 || true; done
    docker image prune -f >/dev/null 2>&1 || true
    echo "✓ done. ${APP} is running ${IMAGE}"
    exit 0
  fi
  sleep 5
done
echo "  ✗ /readyz never became healthy — last log lines:"
docker logs --tail 60 "${APP}-app" || true
if [ -n "$PREV_TAG" ]; then
  echo "  Rollback: ./deploy.sh ${PREV_TAG}   (migrations are forward-only)"
fi
exit 1

# One image = the whole app (static frontend + API). Built ONLY in CI
# (.github/workflows/release-image.yml); the elcodelabs VPS never builds — it pulls
# ghcr.io/__GHCR_OWNER__/testbed-app:<git-sha>.
# - Commit package-lock.json: with it the build uses `npm ci` (reproducible).
# - Runs as non-root uid 10001 with a HEALTHCHECK on /readyz.
# - ENTRYPOINT, not CMD: `docker run <image> <args>` passes args to the app instead of
#   replacing it (a CMD here once broke a one-off `migrate` run on this server).
# - go.sum must be committed (run `go mod tidy` once after scaffolding).
# - golang-migrate ships inside the image so the VPS can migrate without a Go toolchain.

FROM node:22-alpine AS web
WORKDIR /repo
COPY package.json package-lock.json* ./
COPY packages ./packages
COPY apps/web ./apps/web
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
RUN npm run build -w apps/web

FROM golang:1.27-alpine AS api
ENV CGO_ENABLED=0 GOFLAGS=-buildvcs=false
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api ./
RUN go build -trimpath -ldflags "-s -w" -o /out/api ./cmd/server

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata wget \
 && addgroup -S app && adduser -S -G app -u 10001 app
WORKDIR /app
COPY --from=api /out/api ./api
COPY --from=migrate/migrate:v4.18.1 /usr/local/bin/migrate /usr/local/bin/migrate
COPY apps/api/migrations ./migrations
COPY infra/elcodelabs/.env.production.example ./env.production.example
COPY --from=web /repo/apps/web/dist ./public
# DATA_DIR: mounted from ~/<slug>/data on the server; owned by the app user for local runs.
RUN mkdir -p /app/data && chown app:app /app/data
# GOMEMLIMIT: the Go GC aims below the compose memory limit (512M) instead of being OOM-killed.
ENV APP_ENV=production APP_PORT=8080 PUBLIC_DIR=/app/public DATA_DIR=/app/data GOMEMLIMIT=400MiB
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/readyz >/dev/null || exit 1
ENTRYPOINT ["./api"]

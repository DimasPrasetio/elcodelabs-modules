# Rule: Deployment (elcodelabs VPS)

Applies to: `infra/**`, `.github/workflows/**`, `docker-compose.yml`, `.dockerignore`.

- One application = one image = one container. The monorepo ships web + api together; never
  split it across containers and never co-host another app in it.
- The VPS never builds. `.github/workflows/release-image.yml` builds the image and pushes it to
  GHCR; the server only pulls. No `build:` in `infra/elcodelabs/docker-compose.prod.yml`.
- Deploy an immutable `<git-sha>` tag, never `latest`; `IMAGE_TAG` stays required in the
  prod compose.
- MySQL is the host's shared install: one database + one dedicated user for this app, granted
  on its own database only. Never add a database container or service; never publish 3306.
- The container publishes to `127.0.0.1:<port>` only; public traffic enters through the host
  nginx + certbot. The nginx site never claims `default_server`.
- Secrets live only in `~/<app>/.env` on the server (chmod 600). Commit only `*.example`
  env files; never bake secrets into the image.
- The image runs as a non-root user with a HEALTHCHECK on `/readyz`, and uses ENTRYPOINT
  (not CMD). The container stays `read_only`; anything writable is an explicit volume.
- Every deploy runs `infra/elcodelabs/deploy.sh <git-sha>`: DB backup before migrate,
  migrations forward-only.
- Concrete values (port, names, image, domain) are owned by `knowledge/DEPLOYMENT.md`. Any
  change on the VPS itself needs the owner's explicit go-ahead.

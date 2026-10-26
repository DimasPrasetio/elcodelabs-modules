// Package kernel is the stable surface every backend module is built against: what a module
// receives (Deps) and the shared/ packages it may import.
package kernel

import (
	"database/sql"
	"log/slog"

	"testbed-app/internal/config"
)

// Version is the kernel contract version, frontend shared surface included
// (apps/web/src/shared/kernel.ts states the same number). A library module states the
// minimum it needs (requires_kernel).
//
//	1  Deps (Config, DB, Log, Auth), shared/, the module registry
//	2  Deps.Authz: permissions declared by modules, an authorization provider slot;
//	   web: useCan / AccessContext, FrontendModule.provider, NavItem.permission, app events
//	3  web: shared/lib/field-classes.ts (FIELD), Bingkai Navy primitives (Button loading prop,
//	   outline/secondary/ghost restyled), the --animate-loader-slide theme token;
//	   Indonesian default messages in the API and the starter pages
//	4  security and safety: request.DecodeJSON accepts only application/json (415) and never
//	   echoes Go errors; cross-site state-changing calls refused (Sec-Fetch-Site / Origin);
//	   request.ClientIP behind TRUSTED_PROXIES; Authorizer.Covers + d.Authz.Covers; Declare
//	   refuses a name two modules share; production refuses declared permissions without a
//	   provider; APP_ENV must be explicit in the container; DB sessions in UTC; 25 s shutdown;
//	   web: http-client sends cookies to a separate API origin, route error and 404 pages
//	5  Deps.Jobs (scheduled background work, one place at a time across containers) and
//	   Deps.Audit (who did what, with an audit module as provider); request.RequestID and
//	   request.ClientIPFrom for a context; Config.DataDir (DATA_DIR, the one writable folder);
//	   web: open slot names ("<owner>:<place>"), the "profile:changed" app event
const Version = 5

// Deps is what a module's New receives. A module keeps what it needs from it and never
// reaches for globals instead.
type Deps struct {
	Config *config.Config
	DB     *sql.DB
	Log    *slog.Logger
	// Auth guards handlers; the auth provider module fills it (see AuthSlot).
	Auth *AuthSlot
	// Authz guards handlers by permission; an authorization module fills it (see AuthzSlot).
	Authz *AuthzSlot
	// Jobs runs scheduled background work (see JobsSlot).
	Jobs *JobsSlot
	// Audit records who changed what; an audit module stores it (see AuditSlot).
	Audit *AuditSlot
}

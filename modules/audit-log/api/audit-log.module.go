// Package auditlog keeps the audit trail: it fills the kernel's d.Audit, so every module's
// d.Audit.Record lands in one append-only table, readable on the "Log aktivitas" page. It needs
// one optional port, a directory to name the people in it, and imports no other module.
package auditlog

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"__GO_MODULE__/internal/config"
	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/audit-log/application"
	"__GO_MODULE__/internal/modules/audit-log/infrastructure"
	"__GO_MODULE__/internal/modules/audit-log/ports"
	"__GO_MODULE__/internal/modules/audit-log/presentation"
)

// Module is the audit-log module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module. AUDIT_RETENTION_DAYS (default 365) is how long entries are kept.
// users is optional (nil): without it actors are shown by ID.
func New(d *kernel.Deps, users ports.UserDirectory) (*Module, error) {
	d.Authz.Declare(kernel.Permission{Name: presentation.PermRead, Module: "audit-log", Description: "Melihat log aktivitas"})
	days, err := strconv.Atoi(config.Get("AUDIT_RETENTION_DAYS", "365"))
	if err != nil || days < 30 {
		return nil, fmt.Errorf("audit-log: AUDIT_RETENTION_DAYS must be a number of days, at least 30 (got %q)", config.Get("AUDIT_RETENTION_DAYS", ""))
	}
	svc := application.NewService(infrastructure.NewRepository(d.DB), users, time.Duration(days)*24*time.Hour)
	d.Jobs.Every("audit-log.prune", 24*time.Hour, svc.Prune)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz)}, nil
}

// Routes mounts /api/v1/audit.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Sink is what the composition root passes to d.Audit.Provide.
func (m *Module) Sink() kernel.AuditSink { return m.svc }

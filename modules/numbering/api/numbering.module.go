// Package numbering gives business documents their numbers: per company (and, if chosen, per
// branch), restarting each year, month, or day, without gaps — a number is taken inside the
// posting's transaction, and a rollback gives it back. It offers contracts.NumberingClient,
// needs one optional port (ports.Units, for company and branch codes), and imports no module.
package numbering

import (
	"context"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/numbering/application"
	"__GO_MODULE__/internal/modules/numbering/contracts"
	"__GO_MODULE__/internal/modules/numbering/domain"
	"__GO_MODULE__/internal/modules/numbering/infrastructure"
	"__GO_MODULE__/internal/modules/numbering/ports"
	"__GO_MODULE__/internal/modules/numbering/presentation"
)

// Module is the numbering module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module. units is optional (nil): {COMPANY} and {BRANCH} then print IDs.
func New(d *kernel.Deps, units ports.Units) (*Module, error) {
	d.Authz.Declare(
		kernel.Permission{Name: application.PermRead, Module: "numbering", Description: "Melihat format nomor dokumen"},
		kernel.Permission{Name: application.PermManage, Module: "numbering", Description: "Mengubah format dan nomor berikutnya dokumen"},
	)
	svc := application.NewService(infrastructure.NewRepository(d.Tx), d.Tx, units, d.Audit, d.Authz, d.Auth, d.Clock)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz)}, nil
}

// Routes mounts /api/v1/numbering.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.NumberingClient { return client{m.svc} }

type client struct{ svc *application.Service }

func (c client) Declare(docType, label string, def contracts.Format) {
	c.svc.Declare(docType, label, domain.Format{Pattern: def.Pattern, Reset: domain.Reset(def.Reset), PerBranch: def.PerBranch})
}

func (c client) Next(ctx context.Context, docType string, date kernel.Date) (string, error) {
	return c.svc.Next(ctx, docType, date)
}

func (c client) Preview(ctx context.Context, docType string, date kernel.Date) (string, error) {
	return c.svc.Preview(ctx, docType, date)
}

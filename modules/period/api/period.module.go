// Package period closes the books: per company, a date through which an area — or every area —
// is closed, so nothing dated in it is posted or cancelled; and the fiscal year. It offers
// contracts.PeriodClient, needs no port, and imports no module.
package period

import (
	"context"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/period/application"
	"__GO_MODULE__/internal/modules/period/contracts"
	"__GO_MODULE__/internal/modules/period/infrastructure"
	"__GO_MODULE__/internal/modules/period/presentation"
)

// Module is the period module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module.
func New(d *kernel.Deps) (*Module, error) {
	d.Authz.Declare(
		kernel.Permission{Name: application.PermRead, Module: "period", Description: "Melihat tanggal tutup buku"},
		kernel.Permission{Name: application.PermManage, Module: "period", Description: "Menutup buku dan mengatur tahun buku"},
		kernel.Permission{Name: application.PermUnlock, Module: "period", Description: "Membuka kembali buku yang sudah ditutup"},
	)
	svc := application.NewService(infrastructure.NewRepository(d.Tx), d.Tx, d.Audit, d.Authz, d.Auth, d.Clock)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz)}, nil
}

// Routes mounts /api/v1/period.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.PeriodClient { return client{m.svc} }

type client struct{ svc *application.Service }

func (c client) Declare(area, label string) { c.svc.Declare(area, label) }

func (c client) EnsureOpen(ctx context.Context, area string, date kernel.Date) error {
	return c.svc.EnsureOpen(ctx, area, date)
}

func (c client) FiscalYear(ctx context.Context, date kernel.Date) (kernel.Date, kernel.Date, error) {
	return c.svc.FiscalYear(ctx, date)
}

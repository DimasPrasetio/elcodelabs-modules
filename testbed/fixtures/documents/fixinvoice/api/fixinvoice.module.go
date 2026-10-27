// Package fixinvoice is a testbed fixture for the document foundations.
package fixinvoice

import (
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/fixinvoice/application"
	"__GO_MODULE__/internal/modules/fixinvoice/infrastructure"
	"__GO_MODULE__/internal/modules/fixinvoice/ports"
	"__GO_MODULE__/internal/modules/fixinvoice/presentation"
)

type Module struct{ handler *presentation.Handler }

// New builds the fixture; customers is optional (nil: invoices have no customer).
func New(d *kernel.Deps, numbers ports.Numbers, periods ports.Periods, customers ports.Customers) *Module {
	d.Authz.Declare(kernel.Permission{Name: presentation.PermManage, Module: "fixinvoice", Description: "Fixture: mengelola faktur"})
	svc := application.NewService(infrastructure.NewRepository(d.Tx), numbers, periods, customers, d)
	return &Module{handler: presentation.NewHandler(svc, d.Authz)}
}

func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Package orders is a testbed fixture for kernel 6.
package orders

import (
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/orders/application"
	"__GO_MODULE__/internal/modules/orders/infrastructure"
	"__GO_MODULE__/internal/modules/orders/ports"
	"__GO_MODULE__/internal/modules/orders/presentation"
)

type Module struct{ handler *presentation.Handler }

func New(d *kernel.Deps, stock ports.Stock) *Module {
	svc := application.NewService(infrastructure.NewRepository(d.Tx), stock, d)
	return &Module{handler: presentation.NewHandler(svc, d.Auth)}
}

func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

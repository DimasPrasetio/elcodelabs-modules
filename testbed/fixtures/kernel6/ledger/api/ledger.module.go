// Package ledger is a testbed fixture for kernel 6.
package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"__GO_MODULE__/internal/config"
	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/ledger/application"
	"__GO_MODULE__/internal/modules/ledger/contracts"
	"__GO_MODULE__/internal/modules/ledger/infrastructure"
	"__GO_MODULE__/internal/modules/ledger/presentation"
)

type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

func New(d *kernel.Deps) *Module {
	svc := application.NewService(infrastructure.NewRepository(d.Tx))
	if at := config.Get("LEDGER_DAILY_AT", ""); at != "" {
		d.Jobs.Daily("ledger.daily-note", at, func(ctx context.Context) error {
			return svc.Note(kernel.WithScope(ctx, kernel.Scope{CompanyID: kernel.DefaultCompany}), "daily")
		})
	}
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Auth)}
}

func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

func (m *Module) Client() contracts.StockClient { return m.svc }

// OnOrderPlaced notes the order — the subscriber of orders.order.placed, wired in registry.go.
func (m *Module) OnOrderPlaced(ctx context.Context, ev kernel.Event) error {
	var p struct {
		ID     int64  `json:"id"`
		Number string `json:"number"`
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	return m.svc.Note(ctx, fmt.Sprintf("placed:%d:%s", p.ID, p.Number))
}

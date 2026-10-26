// Package application is the orders' use cases.
package application

import (
	"context"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/orders/ports"
	"__GO_MODULE__/internal/shared/fixkit"
)

type Repository interface {
	Create(ctx context.Context, sku string, qty int, at time.Time) (int64, error)
	Count(ctx context.Context) (int64, error)
}

type Service struct {
	repo   Repository
	stock  ports.Stock
	tx     *kernel.TxSlot
	events *kernel.EventsSlot
	clock  kernel.Clock
}

func NewService(repo Repository, stock ports.Stock, d *kernel.Deps) *Service {
	return &Service{repo: repo, stock: stock, tx: d.Tx, events: d.Events, clock: d.Clock}
}

// Place records the order, takes its stock, and announces it — all or nothing.
func (s *Service) Place(ctx context.Context, sku string, qty int) (int64, error) {
	var id int64
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		if id, err = s.repo.Create(ctx, sku, qty, s.clock.Now()); err != nil {
			return err
		}
		if err := s.stock.Take(ctx, sku, qty); err != nil {
			return err
		}
		return s.events.Publish(ctx, "orders.order.placed", 1, map[string]any{"id": id, "number": fixkit.OrderNumber(id), "sku": sku, "qty": qty})
	})
	return id, err
}

func (s *Service) Count(ctx context.Context) (int64, error) { return s.repo.Count(ctx) }

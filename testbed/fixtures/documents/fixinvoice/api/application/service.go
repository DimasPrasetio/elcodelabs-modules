// Package application is the fixture's use cases: draft invoices, and posting, cancelling, and
// amending them by the docstate recipe.
package application

import (
	"context"
	"errors"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/fixinvoice/ports"
	"__GO_MODULE__/internal/shared/docstate"
	"__GO_MODULE__/internal/shared/money"
)

// DocType and Area are what the fixture declares to numbering and period.
const (
	DocType = "fixinvoice.invoice"
	Area    = "sales"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInsufficient = errors.New("stock insufficient")
	ErrAmended      = errors.New("already amended")
	ErrInjected     = errors.New("testbed: failed after taking the number")
)

// Line is one line of an invoice.
type Line struct {
	SKU           string
	Qty           money.Qty
	Price         money.Amount
	Amount        money.Amount // Price × Qty, rounded half-up once
	DiscountShare money.Amount // its part of the invoice discount
}

// Invoice is a document with a lifecycle.
type Invoice struct {
	ID       int64
	BranchID int64
	Date     kernel.Date
	Number   string // "" until posted (an amendment carries its own from the start)
	Discount money.Amount
	Total    money.Amount
	docstate.Fields
	Lines []Line
}

type Repository interface {
	Create(ctx context.Context, inv Invoice, at time.Time) (int64, error)
	Get(ctx context.Context, id int64, forUpdate bool) (Invoice, error)
	UpdateDraft(ctx context.Context, inv Invoice) (int64, error)
	MarkPosted(ctx context.Context, id int64, number string, at time.Time, by int64) (int64, error)
	MarkCancelled(ctx context.Context, id int64, reason string, at time.Time, by int64) (int64, error)
	DeleteDraft(ctx context.Context, id int64) (int64, error)
	AddStock(ctx context.Context, sku string, qty money.Qty) error
	TakeStock(ctx context.Context, sku string, qty money.Qty) (bool, error)
	ReturnStock(ctx context.Context, sku string, qty money.Qty) error
}

type Service struct {
	repo    Repository
	numbers ports.Numbers
	periods ports.Periods
	d       *kernel.Deps
}

func NewService(repo Repository, numbers ports.Numbers, periods ports.Periods, d *kernel.Deps) *Service {
	numbers.Declare(DocType, "Faktur fixture", ports.NumberFormat{Pattern: "INV/{YYYY}/{MM}/{SEQ:4}", Reset: "monthly"})
	periods.Declare(Area, "Penjualan")
	return &Service{repo: repo, numbers: numbers, periods: periods, d: d}
}

func (s *Service) actor(ctx context.Context) int64 {
	id, _ := s.d.Auth.UserID(ctx)
	return id
}

// price computes each line's amount, allocates the discount over the amounts, and the total.
func price(inv *Invoice) error {
	weights := make([]money.Amount, len(inv.Lines))
	for i := range inv.Lines {
		a, err := inv.Lines[i].Price.MulQty(inv.Lines[i].Qty, money.HalfUp)
		if err != nil {
			return err
		}
		inv.Lines[i].Amount, weights[i] = a, a
	}
	gross, err := money.Sum(weights...)
	if err != nil {
		return err
	}
	if !inv.Discount.IsZero() {
		shares, err := money.Allocate(inv.Discount, weights)
		if err != nil {
			return err
		}
		for i := range inv.Lines {
			inv.Lines[i].DiscountShare = shares[i]
		}
	}
	inv.Total, err = gross.Sub(inv.Discount)
	return err
}

func (s *Service) Create(ctx context.Context, inv Invoice) (Invoice, error) {
	inv.BranchID = kernel.ScopeFrom(ctx).BranchID
	if err := price(&inv); err != nil {
		return Invoice{}, err
	}
	err := s.d.Tx.Do(ctx, func(ctx context.Context) error {
		id, err := s.repo.Create(ctx, inv, s.d.Clock.Now())
		inv.ID = id
		return err
	})
	if err != nil {
		return Invoice{}, err
	}
	return s.repo.Get(ctx, inv.ID, false)
}

func (s *Service) Get(ctx context.Context, id int64) (Invoice, error) {
	return s.repo.Get(ctx, id, false)
}

func (s *Service) Update(ctx context.Context, id int64, in Invoice) (Invoice, error) {
	err := s.d.Tx.Do(ctx, func(ctx context.Context) error {
		inv, err := s.repo.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if err := docstate.Check(inv.Status, docstate.Edit); err != nil {
			return err
		}
		inv.Date, inv.Discount, inv.Lines = in.Date, in.Discount, in.Lines
		if err := price(&inv); err != nil {
			return err
		}
		n, err := s.repo.UpdateDraft(ctx, inv)
		if err != nil {
			return err
		}
		return docstate.Guard(n)
	})
	if err != nil {
		return Invoice{}, err
	}
	return s.repo.Get(ctx, id, false)
}

// Post is the recipe: lock, check, period open, stock, number, guarded update, event — one
// transaction. hold keeps it open at the end (the testbed's lock race); failAfterNumber fails it
// right after the number is taken (the testbed's rollback check).
func (s *Service) Post(ctx context.Context, id int64, hold time.Duration, failAfterNumber bool) (Invoice, error) {
	err := s.d.Tx.Do(ctx, func(ctx context.Context) error {
		inv, err := s.repo.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if err := docstate.Check(inv.Status, docstate.Post); err != nil {
			return err
		}
		if err := s.periods.EnsureOpen(ctx, Area, inv.Date); err != nil {
			return err
		}
		for _, l := range inv.Lines {
			ok, err := s.repo.TakeStock(ctx, l.SKU, l.Qty)
			if err != nil {
				return err
			}
			if !ok {
				return ErrInsufficient
			}
		}
		// The number last: the series' counter stays locked until the commit, so the less work
		// after it, the shorter other postings queue.
		number := inv.Number // an amendment already has its number
		if number == "" {
			sc := kernel.ScopeFrom(ctx)
			in := kernel.WithScope(ctx, kernel.Scope{CompanyID: sc.CompanyID, BranchID: inv.BranchID, Location: sc.Location})
			if number, err = s.numbers.Next(in, DocType, inv.Date); err != nil {
				return err
			}
		}
		if failAfterNumber {
			return ErrInjected
		}
		n, err := s.repo.MarkPosted(ctx, id, number, s.d.Clock.Now(), s.actor(ctx))
		if err != nil {
			return err
		}
		if err := docstate.Guard(n); err != nil {
			return err
		}
		if err := s.d.Events.Publish(ctx, "fixinvoice.invoice.posted", 1, map[string]any{"id": id, "number": number, "total": inv.Total}); err != nil {
			return err
		}
		if hold > 0 {
			select {
			case <-time.After(hold):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
	if err != nil {
		return Invoice{}, err
	}
	return s.repo.Get(ctx, id, false)
}

func (s *Service) Cancel(ctx context.Context, id int64, reason string) (Invoice, error) {
	reason, err := docstate.CancelReason(reason)
	if err != nil {
		return Invoice{}, err
	}
	err = s.d.Tx.Do(ctx, func(ctx context.Context) error {
		inv, err := s.repo.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if err := docstate.Check(inv.Status, docstate.Cancel); err != nil {
			return err
		}
		if err := s.periods.EnsureOpen(ctx, Area, inv.Date); err != nil {
			return err
		}
		for _, l := range inv.Lines {
			if err := s.repo.ReturnStock(ctx, l.SKU, l.Qty); err != nil {
				return err
			}
		}
		n, err := s.repo.MarkCancelled(ctx, id, reason, s.d.Clock.Now(), s.actor(ctx))
		if err != nil {
			return err
		}
		if err := docstate.Guard(n); err != nil {
			return err
		}
		return s.d.Events.Publish(ctx, "fixinvoice.invoice.cancelled", 1, map[string]any{"id": id, "number": inv.Number, "reason": reason})
	})
	if err != nil {
		return Invoice{}, err
	}
	return s.repo.Get(ctx, id, false)
}

// Amend starts the correction of a cancelled invoice: a new draft with the same lines, pointing
// at it, numbered from it.
func (s *Service) Amend(ctx context.Context, id int64) (Invoice, error) {
	var newID int64
	err := s.d.Tx.Do(ctx, func(ctx context.Context) error {
		inv, err := s.repo.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if err := docstate.Check(inv.Status, docstate.Amend); err != nil {
			return err
		}
		draft := inv
		draft.Number, draft.Revision = docstate.Amendment(inv.Number, inv.Revision)
		draft.AmendedFrom = inv.ID
		newID, err = s.repo.Create(ctx, draft, s.d.Clock.Now())
		return err
	})
	if err != nil {
		return Invoice{}, err
	}
	return s.repo.Get(ctx, newID, false)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.d.Tx.Do(ctx, func(ctx context.Context) error {
		inv, err := s.repo.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if err := docstate.Check(inv.Status, docstate.Delete); err != nil {
			return err
		}
		n, err := s.repo.DeleteDraft(ctx, id)
		if err != nil {
			return err
		}
		return docstate.Guard(n)
	})
}

func (s *Service) AddStock(ctx context.Context, sku string, qty money.Qty) error {
	return s.d.Tx.Do(ctx, func(ctx context.Context) error { return s.repo.AddStock(ctx, sku, qty) })
}

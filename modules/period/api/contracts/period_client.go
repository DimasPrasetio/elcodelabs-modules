// Package contracts is what the period module offers. Another module never imports it: it
// declares its own port, and an adapter in the composition root connects the two.
package contracts

import (
	"context"
	"errors"

	"__GO_MODULE__/internal/kernel"
)

// ErrClosed matches every *ClosedError: the date is in a closed period.
var ErrClosed = errors.New("period closed")

// ClosedError is a refused date, with a message for the person who entered it.
type ClosedError struct {
	Area    string      // the area asked about
	Through kernel.Date // the books are closed through this date (for this area)
	Msg     string
}

func (e *ClosedError) Error() string        { return e.Msg }
func (e *ClosedError) Is(target error) bool { return target == ErrClosed }

// PeriodClient is the period module's public contract.
type PeriodClient interface {
	// Declare registers an area that can be closed on its own ("sales", "payroll") with its
	// label ("Penjualan") — in the consumer's New. EnsureOpen refuses an area never declared.
	Declare(area, label string)
	// EnsureOpen returns a *ClosedError (errors.Is ErrClosed — a 409 for the consumer) when date
	// is on or before the area's lock or the company-wide one, in the company ctx works in.
	// Inside d.Tx it holds the company's lock shared until the transaction ends, so a lock being
	// moved waits for the posting, or the posting for it — never both half-way.
	EnsureOpen(ctx context.Context, area string, date kernel.Date) error
	// FiscalYear is the fiscal year date falls in, first and last day, for the company.
	FiscalYear(ctx context.Context, date kernel.Date) (start, end kernel.Date, err error)
}

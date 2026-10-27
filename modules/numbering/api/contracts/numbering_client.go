// Package contracts is what the numbering module offers. Another module never imports it: it
// declares its own port, and an adapter in the composition root connects the two.
package contracts

import (
	"context"
	"errors"

	"__GO_MODULE__/internal/kernel"
)

// ErrNotInTx is Next called outside d.Tx: the number could be lost, so none is given.
var ErrNotInTx = errors.New("numbering: Next must run inside d.Tx")

// Format is how a document type is numbered by default.
type Format struct {
	// Pattern is text with tokens: {YYYY} {YY} {MM} {DD} (the document's date), {SEQ:n} (the
	// counter, zero-padded to n digits; exactly one), {BRANCH} and {COMPANY} (their codes).
	Pattern string
	// Reset is when the counter starts again: "never", "yearly", "monthly", or "daily" — the
	// pattern must then hold the year (and month, and day).
	Reset string
	// PerBranch gives each branch its own counter; the pattern must then hold {BRANCH}.
	PerBranch bool
}

// NumberingClient is the numbering module's public contract.
type NumberingClient interface {
	// Declare registers a document type ("sales.invoice") with its label ("Faktur penjualan")
	// and default format — in the consumer's New. An invalid name or format panics at start.
	Declare(docType, label string, def Format)
	// Next takes the next number of docType for date, in the scope ctx works in (its company,
	// and its branch for a per-branch series — pass kernel.WithScope with the document's branch
	// when it differs from the request's). It must run inside d.Tx (else ErrNotInTx): the
	// counter stays locked until that transaction ends, and a rollback gives the number back —
	// no gaps.
	Next(ctx context.Context, docType string, date kernel.Date) (string, error)
	// Preview is what Next would return now, without taking it — for a form.
	Preview(ctx context.Context, docType string, date kernel.Date) (string, error)
}

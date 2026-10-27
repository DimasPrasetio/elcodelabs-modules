// Package ports is what the fixture needs from other modules, in its own terms.
package ports

import (
	"context"

	"__GO_MODULE__/internal/kernel"
)

// NumberFormat is a document type's default number format.
type NumberFormat struct {
	Pattern   string
	Reset     string
	PerBranch bool
}

// Numbers gives documents their numbers.
type Numbers interface {
	Declare(docType, label string, def NumberFormat)
	// Next must run inside d.Tx: a rollback gives the number back.
	Next(ctx context.Context, docType string, date kernel.Date) (string, error)
}

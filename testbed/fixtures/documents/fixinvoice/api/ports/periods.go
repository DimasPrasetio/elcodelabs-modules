package ports

import (
	"context"

	"__GO_MODULE__/internal/kernel"
)

// PeriodClosedError is a document date in a closed period; Message says until when.
type PeriodClosedError struct{ Message string }

func (e *PeriodClosedError) Error() string { return e.Message }

// Periods refuses dates in closed periods.
type Periods interface {
	Declare(area, label string)
	// EnsureOpen returns a *PeriodClosedError for a date in a closed period of area.
	EnsureOpen(ctx context.Context, area string, date kernel.Date) error
}

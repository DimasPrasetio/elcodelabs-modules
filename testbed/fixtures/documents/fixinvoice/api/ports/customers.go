package ports

import "context"

// CustomerRefusedError is a customer a new invoice may not use (unknown, inactive, on hold).
type CustomerRefusedError struct{ Message string }

func (e *CustomerRefusedError) Error() string { return e.Message }

// CustomerSnapshot is what a posted invoice keeps of its customer.
type CustomerSnapshot struct {
	Name  string
	TaxID string
}

// Customers checks and copies an invoice's customer. Optional: without it invoices have none.
type Customers interface {
	// Usable returns a *CustomerRefusedError for a customer the invoice may not use; inside the
	// posting's d.Tx it holds the customer until the commit.
	Usable(ctx context.Context, id int64) error
	Snapshot(ctx context.Context, id int64) (CustomerSnapshot, error)
}

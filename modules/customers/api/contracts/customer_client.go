// Package contracts is what the customers module offers. Another module never imports it: it
// declares its own port, and an adapter in the composition root connects the two.
package contracts

import (
	"context"
	"errors"

	"__GO_MODULE__/internal/shared/money"
)

var (
	// ErrNotFound is no customer with that ID in the company the request works in.
	ErrNotFound = errors.New("customers: not found")
	// ErrInactive is a deactivated customer: old documents still show it, new ones may not use it.
	ErrInactive = errors.New("customers: inactive")
	// ErrOnHold is a customer someone put on hold, with a reason (Customer.HoldReason).
	ErrOnHold = errors.New("customers: on hold")
)

// Customer is a customer as another module sees it.
type Customer struct {
	ID          int64
	Code        string
	Name        string
	PartyType   string        // company | individual | government
	PaymentDays *int          // nil: cash
	CreditLimit *money.Amount // nil: no limit
	OnHold      bool
	HoldReason  string
	IsActive    bool
}

// Snapshot is what a document copies when it is posted, so it keeps showing what was true then.
type Snapshot struct {
	CustomerID   int64
	Code         string
	Name         string // the legal name when there is one: what a tax invoice prints
	PartyType    string
	IsForeign    bool
	Country      string
	TaxIDKind    string // npwp | nik | passport | none
	TaxID        string // whole, unmasked: it goes on the tax invoice
	IsPKP        bool
	VATCollector string // none | government | other
	AddressID    int64  // 0 when the customer has no address
	Address      string // one line
	NITKU        string // the address's, or ""
	Email        string
}

// CustomerClient is the customers module's public contract. Every call works in the company of
// ctx's scope: another company's customer is ErrNotFound.
type CustomerClient interface {
	// Get returns a customer, active or not — an old document still renders.
	Get(ctx context.Context, id int64) (Customer, error)
	// Search finds active customers by code, name, or tax ID, for pickers (limit up to 50).
	Search(ctx context.Context, q string, limit int) ([]Customer, error)
	// Usable is what a document checks before it posts, inside its d.Tx: nil, ErrNotFound,
	// ErrInactive, or ErrOnHold. It holds the customer's row shared until the transaction ends,
	// so a deactivation or hold waits for the posting, and the other way round.
	Usable(ctx context.Context, id int64) error
	// Snapshot is what a document copies when posted: the customer and its address addressID —
	// 0 for the default billing address (or the first active one).
	Snapshot(ctx context.Context, id, addressID int64) (Snapshot, error)
}

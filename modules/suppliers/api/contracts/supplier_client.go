// Package contracts is what the suppliers module offers. Another module never imports it: it
// declares its own port, and an adapter in the composition root connects the two.
package contracts

import (
	"context"
	"errors"
)

var (
	// ErrNotFound is no supplier (or bank account) with that ID in the company the request works in.
	ErrNotFound = errors.New("suppliers: not found")
	// ErrInactive is a deactivated supplier: old documents still show it, new ones may not use it.
	ErrInactive = errors.New("suppliers: inactive")
	// ErrOnHold is a supplier someone put on hold, with a reason (Supplier.HoldReason).
	ErrOnHold = errors.New("suppliers: on hold")
)

// Supplier is a supplier as another module sees it.
type Supplier struct {
	ID          int64
	Code        string
	Name        string
	PartyType   string // company | individual | government
	IsForeign   bool
	TaxIDKind   string // npwp | nik | passport | none — a withholding module decides rates from it
	IsPKP       bool   // input VAT is creditable only from a PKP
	PaymentDays *int   // nil: cash on delivery
	OnHold      bool
	HoldReason  string
	IsActive    bool
}

// Snapshot is what a document copies when it is posted, so it keeps showing what was true then.
type Snapshot struct {
	SupplierID int64
	Code       string
	Name       string // the legal name when there is one
	PartyType  string
	IsForeign  bool
	Country    string
	TaxIDKind  string // npwp | nik | passport | none
	TaxID      string // whole, unmasked: it goes on withholding slips
	IsPKP      bool
	AddressID  int64  // 0 when the supplier has no address
	Address    string // one line
	NITKU      string // the address's, or ""
	Email      string
}

// BankAccount is where a supplier is paid — whole, for a payment.
type BankAccount struct {
	ID            int64
	BankName      string
	BankCode      string
	AccountNumber string
	AccountHolder string
}

// SupplierClient is the suppliers module's public contract. Every call works in the company of
// ctx's scope: another company's supplier is ErrNotFound.
type SupplierClient interface {
	// Get returns a supplier, active or not — an old document still renders.
	Get(ctx context.Context, id int64) (Supplier, error)
	// Search finds active suppliers by code, name, or tax ID, for pickers (limit up to 50).
	Search(ctx context.Context, q string, limit int) ([]Supplier, error)
	// Usable is what a document checks before it posts, inside its d.Tx: nil, ErrNotFound,
	// ErrInactive, or ErrOnHold. It holds the supplier's row shared until the transaction ends,
	// so a deactivation or hold waits for the posting, and the other way round.
	Usable(ctx context.Context, id int64) error
	// Snapshot is what a document copies when posted: the supplier and its address addressID —
	// 0 for the default billing address (or the first active one).
	Snapshot(ctx context.Context, id, addressID int64) (Snapshot, error)
	// DefaultBankAccount is the account a payment goes to: the active default, else the first
	// active one; ErrNotFound when there is none. A payment module records reading it.
	DefaultBankAccount(ctx context.Context, id int64) (BankAccount, error)
}

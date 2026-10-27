// Package contracts is what the organization module offers. Another module never imports it:
// it declares its own port, and an adapter in the composition root connects the two.
package contracts

import (
	"context"
	"errors"
)

// ErrNotFound means no company or branch has that ID.
var ErrNotFound = errors.New("not found")

// Company is a legal entity the app serves.
type Company struct {
	ID       int64
	Code     string
	Name     string // short name; the legal name and NPWP are the settings module's
	Timezone string // IANA
	IsActive bool
}

// Branch is a place of business of a company.
type Branch struct {
	ID        int64
	CompanyID int64
	Code      string
	Name      string
	Timezone  string // IANA; "" — the company's
	NITKU     string // 22 digits, or ""
	Address   string
	IsActive  bool
}

// Workplace is one place a user is placed: across a company (BranchID 0) or in one branch.
type Workplace struct {
	CompanyID int64
	BranchID  int64
}

// OrganizationClient is the organization module's public contract.
type OrganizationClient interface {
	// Company returns the company, or ErrNotFound.
	Company(ctx context.Context, id int64) (Company, error)
	// Branch returns the branch, or ErrNotFound.
	Branch(ctx context.Context, id int64) (Branch, error)
	// Branches lists a company's branches, active or not.
	Branches(ctx context.Context, companyID int64) ([]Branch, error)
	// PlacedUserIDs lists the users placed in the company (anywhere in it).
	PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error)
	// IsPlaced reports whether the user is placed in the company (anywhere in it).
	IsPlaced(ctx context.Context, userID, companyID int64) (bool, error)
	// PlacesOf lists every placement of the user, in active units or not — where their account
	// belongs, for deciding who may act on it (deactivating a unit hands its people to no one).
	PlacesOf(ctx context.Context, userID int64) ([]Workplace, error)
	// PlaceNew places a user just created in the scope ctx works in (its company, or its branch)
	// — inside the caller's transaction. For a users module creating accounts.
	PlaceNew(ctx context.Context, userID int64) error
}

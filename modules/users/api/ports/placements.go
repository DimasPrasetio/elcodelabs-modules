// Package ports is what the users module may use from outside. Everything here is optional.
package ports

import "context"

// Placements says who is placed in which company — an organization module, wired once every
// module is built (register.go.late). With it, the users pages show only the people placed in
// the company the request works in, and a new user is placed where they were created.
type Placements interface {
	// PlacedUserIDs lists the users placed in the company (anywhere in it).
	PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error)
	// IsPlaced reports whether the user is placed in the company (anywhere in it).
	IsPlaced(ctx context.Context, userID, companyID int64) (bool, error)
	// PlaceNew places a user just created in the scope ctx works in — inside the caller's
	// transaction.
	PlaceNew(ctx context.Context, userID int64) error
	// PlacesOf lists every placement of the user (in active units or not): where their account
	// belongs.
	PlacesOf(ctx context.Context, userID int64) ([]Workplace, error)
}

// Workplace is a company (BranchID 0: all of it) or one of its branches.
type Workplace struct {
	CompanyID int64
	BranchID  int64
}

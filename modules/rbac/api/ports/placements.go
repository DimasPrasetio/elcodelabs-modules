package ports

import "context"

// Placements says who works where — an organization module, wired once every module is built
// (register.go.late). Optional: without it an account belongs only where it holds roles (or, with
// none, to the home company), and the member search lists everyone.
type Placements interface {
	// PlacesOf lists every placement of the user (in active units or not): where their account
	// belongs.
	PlacesOf(ctx context.Context, userID int64) ([]Workplace, error)
	// PlacedUserIDs lists the users placed in the company (anywhere in it).
	PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error)
}

// Workplace is a company (BranchID 0: all of it) or one of its branches.
type Workplace struct {
	CompanyID int64
	BranchID  int64
}

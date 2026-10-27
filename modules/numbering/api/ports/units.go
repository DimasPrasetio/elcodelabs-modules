// Package ports is what the numbering module needs from others, in its own terms.
package ports

import "context"

// Branch is a branch of the company, for previews and {BRANCH}.
type Branch struct {
	ID     int64
	Code   string
	Name   string
	Active bool
}

// Units gives the codes {COMPANY} and {BRANCH} print. Optional: without it they print the
// company's and branch's IDs.
type Units interface {
	CompanyCode(ctx context.Context, companyID int64) (string, error)
	BranchCode(ctx context.Context, branchID int64) (string, error)
	// Branches lists the company's branches, active or not.
	Branches(ctx context.Context, companyID int64) ([]Branch, error)
}

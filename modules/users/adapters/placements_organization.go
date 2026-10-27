package modules

import (
	"context"

	organizationcontracts "__GO_MODULE__/internal/modules/organization/contracts"
	usersports "__GO_MODULE__/internal/modules/users/ports"
)

// usersPlacements satisfies users' Placements port with the organization module's client.
// Mapping only: no logic lives here.
type usersPlacements struct {
	organization organizationcontracts.OrganizationClient
}

func (a usersPlacements) PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error) {
	return a.organization.PlacedUserIDs(ctx, companyID)
}

func (a usersPlacements) IsPlaced(ctx context.Context, userID, companyID int64) (bool, error) {
	return a.organization.IsPlaced(ctx, userID, companyID)
}

func (a usersPlacements) PlaceNew(ctx context.Context, userID int64) error {
	return a.organization.PlaceNew(ctx, userID)
}

func (a usersPlacements) PlacesOf(ctx context.Context, userID int64) ([]usersports.Workplace, error) {
	ws, err := a.organization.PlacesOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]usersports.Workplace, len(ws))
	for i, w := range ws {
		out[i] = usersports.Workplace{CompanyID: w.CompanyID, BranchID: w.BranchID}
	}
	return out, nil
}

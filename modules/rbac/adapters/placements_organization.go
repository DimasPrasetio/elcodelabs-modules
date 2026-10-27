package modules

import (
	"context"

	organizationcontracts "__GO_MODULE__/internal/modules/organization/contracts"
	rbacports "__GO_MODULE__/internal/modules/rbac/ports"
)

// rbacPlacements satisfies rbac's Placements port with the organization module's client.
// Mapping only: no logic lives here.
type rbacPlacements struct {
	organization organizationcontracts.OrganizationClient
}

func (a rbacPlacements) PlacesOf(ctx context.Context, userID int64) ([]rbacports.Workplace, error) {
	ws, err := a.organization.PlacesOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]rbacports.Workplace, len(ws))
	for i, w := range ws {
		out[i] = rbacports.Workplace{CompanyID: w.CompanyID, BranchID: w.BranchID}
	}
	return out, nil
}

func (a rbacPlacements) PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error) {
	return a.organization.PlacedUserIDs(ctx, companyID)
}

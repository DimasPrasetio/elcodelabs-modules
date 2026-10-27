package modules

import (
	"context"

	numberingports "__GO_MODULE__/internal/modules/numbering/ports"
	organizationcontracts "__GO_MODULE__/internal/modules/organization/contracts"
)

// numberingUnits satisfies numbering's Units port with the organization module's client.
// Mapping only: no logic lives here.
type numberingUnits struct {
	organization organizationcontracts.OrganizationClient
}

func (a numberingUnits) CompanyCode(ctx context.Context, companyID int64) (string, error) {
	c, err := a.organization.Company(ctx, companyID)
	return c.Code, err
}

func (a numberingUnits) BranchCode(ctx context.Context, branchID int64) (string, error) {
	b, err := a.organization.Branch(ctx, branchID)
	return b.Code, err
}

func (a numberingUnits) Branches(ctx context.Context, companyID int64) ([]numberingports.Branch, error) {
	bs, err := a.organization.Branches(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]numberingports.Branch, len(bs))
	for i, b := range bs {
		out[i] = numberingports.Branch{ID: b.ID, Code: b.Code, Name: b.Name, Active: b.IsActive}
	}
	return out, nil
}

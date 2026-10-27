// Package organization keeps the companies the app serves, their branches, and where each user
// is placed — and answers the kernel which company and branch a request works in (d.Scope). It
// offers contracts.OrganizationClient, needs one port (ports.UserDirectory), and imports no
// module.
package organization

import (
	"context"
	"errors"
	"net/http"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/organization/application"
	"__GO_MODULE__/internal/modules/organization/contracts"
	"__GO_MODULE__/internal/modules/organization/domain"
	"__GO_MODULE__/internal/modules/organization/infrastructure"
	"__GO_MODULE__/internal/modules/organization/ports"
	"__GO_MODULE__/internal/modules/organization/presentation"
)

// Module is the organization module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module. On a fresh install it names the home company (company 1) after the
// app and places every existing user across it, so installing it changes nothing for anyone; a
// failed bootstrap (database still down) is logged, and the next start tries again.
func New(d *kernel.Deps, users ports.UserDirectory) (*Module, error) {
	if users == nil {
		return nil, errors.New("organization: the UserDirectory port is required")
	}
	d.Authz.Declare(
		kernel.Permission{Name: application.PermRead, Module: "organization", Description: "Melihat cabang dan penempatan pengguna"},
		kernel.Permission{Name: application.PermManage, Module: "organization", Description: "Mengelola cabang dan menempatkan pengguna"},
		kernel.Permission{Name: application.PermCompanies, Module: "organization", Description: "Membuat, mengubah, dan menonaktifkan perusahaan (dari perusahaan induk)"},
	)
	svc := application.NewService(infrastructure.NewRepository(d.Tx), d.Tx, users, d.Events, d.Audit, d.Authz, d.Auth, d.Clock)
	m := &Module{svc: svc, handler: presentation.NewHandler(svc, d.Auth, d.Authz)}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	placed, err := svc.Bootstrap(ctx, d.Config.Name)
	switch {
	case err != nil:
		d.Log.Warn("organization: bootstrap not done", "err", err)
	case placed > 0:
		d.Log.Info("organization: every user placed across the home company", "users", placed)
	}
	return m, nil
}

// Routes mounts /api/v1/organization.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Resolver is what the composition root passes to d.Scope.Provide.
func (m *Module) Resolver() kernel.ScopeResolver { return resolver{m.svc} }

type resolver struct{ svc *application.Service }

func (r resolver) Resolve(ctx context.Context, userID int64, requested string) (kernel.Scope, error) {
	return r.svc.Resolve(ctx, userID, requested)
}

func (r resolver) Companies(ctx context.Context) ([]kernel.Scope, error) {
	return r.svc.ActiveCompanies(ctx)
}

// Client returns the module's public contract.
func (m *Module) Client() contracts.OrganizationClient { return client{m.svc} }

type client struct{ svc *application.Service }

func (c client) Company(ctx context.Context, id int64) (contracts.Company, error) {
	co, err := c.svc.Company(ctx, id)
	if err != nil {
		return contracts.Company{}, contractErr(err)
	}
	return contracts.Company{ID: co.ID, Code: co.Code, Name: co.Name, Timezone: co.Timezone, IsActive: co.IsActive}, nil
}

func toContractBranch(b domain.Branch) contracts.Branch {
	return contracts.Branch{ID: b.ID, CompanyID: b.CompanyID, Code: b.Code, Name: b.Name, Timezone: b.Timezone, NITKU: b.NITKU, Address: b.Address, IsActive: b.IsActive}
}

func (c client) Branch(ctx context.Context, id int64) (contracts.Branch, error) {
	b, err := c.svc.Branch(ctx, id)
	if err != nil {
		return contracts.Branch{}, contractErr(err)
	}
	return toContractBranch(b), nil
}

func (c client) Branches(ctx context.Context, companyID int64) ([]contracts.Branch, error) {
	bs, err := c.svc.BranchesOf(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.Branch, len(bs))
	for i, b := range bs {
		out[i] = toContractBranch(b)
	}
	return out, nil
}

func (c client) PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error) {
	return c.svc.PlacedUserIDs(ctx, companyID)
}

func (c client) IsPlaced(ctx context.Context, userID, companyID int64) (bool, error) {
	return c.svc.IsPlaced(ctx, userID, companyID)
}

func (c client) PlacesOf(ctx context.Context, userID int64) ([]contracts.Workplace, error) {
	ps, err := c.svc.Places(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.Workplace, len(ps))
	for i, p := range ps {
		out[i] = contracts.Workplace{CompanyID: p.CompanyID, BranchID: p.BranchID}
	}
	return out, nil
}

func (c client) PlaceNew(ctx context.Context, userID int64) error { return c.svc.PlaceNew(ctx, userID) }

func contractErr(err error) error {
	if errors.Is(err, application.ErrNotFound) {
		return contracts.ErrNotFound
	}
	return err
}

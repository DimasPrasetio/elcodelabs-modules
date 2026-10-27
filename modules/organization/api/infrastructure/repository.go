// Package infrastructure stores companies, branches, and placements in MySQL through the
// module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/organization/application"
	"__GO_MODULE__/internal/modules/organization/domain"
	"__GO_MODULE__/internal/modules/organization/infrastructure/sqlc"
)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func toCompany(row sqlc.OrgCompany) domain.Company {
	return domain.Company{ID: row.ID, Code: row.Code, Name: row.Name, Timezone: row.Timezone, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (r *Repository) Companies(ctx context.Context, activeOnly bool) ([]domain.Company, error) {
	var rows []sqlc.OrgCompany
	var err error
	if activeOnly {
		rows, err = r.q(ctx).ListActiveCompanies(ctx)
	} else {
		rows, err = r.q(ctx).ListCompanies(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]domain.Company, len(rows))
	for i, row := range rows {
		out[i] = toCompany(row)
	}
	return out, nil
}

func (r *Repository) Company(ctx context.Context, id int64) (domain.Company, error) {
	row, err := r.q(ctx).GetCompany(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Company{}, application.ErrNotFound
	}
	if err != nil {
		return domain.Company{}, err
	}
	return toCompany(row), nil
}

func (r *Repository) CreateCompany(ctx context.Context, c domain.Company) (int64, error) {
	id, err := r.q(ctx).CreateCompany(ctx, sqlc.CreateCompanyParams{Code: c.Code, Name: c.Name, Timezone: c.Timezone, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt})
	return id, duplicate(err)
}

func (r *Repository) UpdateCompany(ctx context.Context, c domain.Company) error {
	n, err := r.q(ctx).UpdateCompany(ctx, sqlc.UpdateCompanyParams{Code: c.Code, Name: c.Name, Timezone: c.Timezone, UpdatedAt: c.UpdatedAt, ID: c.ID})
	return affected(n, duplicate(err))
}

func (r *Repository) SetCompanyActive(ctx context.Context, id int64, active bool, at time.Time) error {
	n, err := r.q(ctx).SetCompanyActive(ctx, sqlc.SetCompanyActiveParams{IsActive: active, UpdatedAt: at, ID: id})
	return affected(n, err)
}

func toBranch(row sqlc.OrgBranch) domain.Branch {
	return domain.Branch{ID: row.ID, CompanyID: row.CompanyID, Code: row.Code, Name: row.Name, Timezone: row.Timezone.String, NITKU: row.Nitku.String,
		Address: row.Address, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func nullable(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func (r *Repository) Branches(ctx context.Context, companyID int64) ([]domain.Branch, error) {
	rows, err := r.q(ctx).ListBranches(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Branch, len(rows))
	for i, row := range rows {
		out[i] = toBranch(row)
	}
	return out, nil
}

func (r *Repository) Branch(ctx context.Context, id int64) (domain.Branch, error) {
	row, err := r.q(ctx).GetBranch(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Branch{}, application.ErrNotFound
	}
	if err != nil {
		return domain.Branch{}, err
	}
	return toBranch(row), nil
}

func (r *Repository) CreateBranch(ctx context.Context, b domain.Branch) (int64, error) {
	id, err := r.q(ctx).CreateBranch(ctx, sqlc.CreateBranchParams{CompanyID: b.CompanyID, Code: b.Code, Name: b.Name, Timezone: nullable(b.Timezone),
		Nitku: nullable(b.NITKU), Address: b.Address, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt})
	return id, duplicate(err)
}

func (r *Repository) UpdateBranch(ctx context.Context, b domain.Branch) error {
	n, err := r.q(ctx).UpdateBranch(ctx, sqlc.UpdateBranchParams{Code: b.Code, Name: b.Name, Timezone: nullable(b.Timezone), Nitku: nullable(b.NITKU),
		Address: b.Address, UpdatedAt: b.UpdatedAt, ID: b.ID, CompanyID: b.CompanyID})
	return affected(n, duplicate(err))
}

func (r *Repository) SetBranchActive(ctx context.Context, companyID, id int64, active bool, at time.Time) error {
	n, err := r.q(ctx).SetBranchActive(ctx, sqlc.SetBranchActiveParams{IsActive: active, UpdatedAt: at, ID: id, CompanyID: companyID})
	return affected(n, err)
}

// PlacementsOfUser is where the user may work: placements in active companies — across them,
// or in one of their active branches.
func (r *Repository) PlacementsOfUser(ctx context.Context, userID int64) ([]application.UnitPlacement, error) {
	rows, err := r.q(ctx).PlacementsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	branches, err := r.q(ctx).BranchesOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	active := make(map[int64]sqlc.BranchesOfUserRow, len(branches))
	for _, b := range branches {
		active[b.ID] = b
	}
	out := make([]application.UnitPlacement, 0, len(rows))
	for _, row := range rows {
		p := application.UnitPlacement{CompanyID: row.CompanyID, BranchID: row.BranchID, IsDefault: row.IsDefault,
			CompanyCode: row.CompanyCode, CompanyName: row.CompanyName, CompanyTimezone: row.CompanyTimezone}
		if row.BranchID != 0 {
			b, ok := active[row.BranchID]
			if !ok || b.CompanyID != row.CompanyID {
				continue // a deactivated branch
			}
			p.BranchCode, p.BranchName, p.BranchTimezone = b.Code, b.Name, b.Timezone.String
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *Repository) PlacesOfUser(ctx context.Context, userID int64) ([]domain.Placement, error) {
	rows, err := r.q(ctx).PlacesOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Placement, len(rows))
	for i, row := range rows {
		out[i] = domain.Placement{UserID: row.UserID, CompanyID: row.CompanyID, BranchID: row.BranchID, IsDefault: row.IsDefault, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy}
	}
	return out, nil
}

func (r *Repository) Placements(ctx context.Context, companyID int64) ([]domain.Placement, error) {
	rows, err := r.q(ctx).ListPlacements(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Placement, len(rows))
	for i, row := range rows {
		out[i] = domain.Placement{UserID: row.UserID, CompanyID: row.CompanyID, BranchID: row.BranchID, IsDefault: row.IsDefault, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy}
	}
	return out, nil
}

func (r *Repository) AddPlacement(ctx context.Context, p domain.Placement) (bool, error) {
	n, err := r.q(ctx).AddPlacement(ctx, sqlc.AddPlacementParams{UserID: p.UserID, CompanyID: p.CompanyID, BranchID: p.BranchID, IsDefault: p.IsDefault,
		CreatedAt: p.CreatedAt, CreatedBy: p.CreatedBy})
	return n == 1, err
}

func (r *Repository) RemovePlacement(ctx context.Context, userID, companyID, branchID int64) (bool, error) {
	n, err := r.q(ctx).RemovePlacement(ctx, sqlc.RemovePlacementParams{UserID: userID, CompanyID: companyID, BranchID: branchID})
	return n == 1, err
}

// SetDefault moves the user's default to that placement — false (and nothing changed, once the
// caller's transaction rolls back) when there is no such placement.
func (r *Repository) SetDefault(ctx context.Context, userID, companyID, branchID int64) (bool, error) {
	if err := r.q(ctx).ClearDefault(ctx, userID); err != nil {
		return false, err
	}
	n, err := r.q(ctx).SetDefault(ctx, sqlc.SetDefaultParams{UserID: userID, CompanyID: companyID, BranchID: branchID})
	return n == 1, err
}

func (r *Repository) PlacedUserIDs(ctx context.Context, companyID int64) ([]int64, error) {
	return r.q(ctx).PlacedUserIDs(ctx, companyID)
}

func (r *Repository) IsPlaced(ctx context.Context, userID, companyID int64) (bool, error) {
	n, err := r.q(ctx).CountUserInCompany(ctx, sqlc.CountUserInCompanyParams{UserID: userID, CompanyID: companyID})
	return n > 0, err
}

func (r *Repository) CountPlacementsOfUser(ctx context.Context, userID int64) (int64, error) {
	return r.q(ctx).CountPlacementsOfUser(ctx, userID)
}

func (r *Repository) CountPlacements(ctx context.Context) (int64, error) {
	return r.q(ctx).CountPlacements(ctx)
}

func (r *Repository) UsersWithAWorkplace(ctx context.Context) ([]int64, error) {
	return r.q(ctx).UsersWithAWorkplace(ctx)
}

// affected is ErrNotFound when an update matched no row.
func affected(n int64, err error) error {
	if err == nil && n == 0 {
		return application.ErrNotFound
	}
	return err
}

// duplicate maps MySQL's duplicate-key error (a code in use) to ErrCodeTaken.
func duplicate(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return application.ErrCodeTaken
	}
	return err
}

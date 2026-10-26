// Package infrastructure stores roles in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/rbac/application"
	"__GO_MODULE__/internal/modules/rbac/domain"
	"__GO_MODULE__/internal/modules/rbac/infrastructure/sqlc"
)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository {
	return &Repository{tx: tx}
}

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func (r *Repository) ListRoles(ctx context.Context, company int64) ([]domain.Role, error) {
	rows, err := r.q(ctx).ListRoles(ctx, company)
	if err != nil {
		return nil, err
	}
	perms, err := r.q(ctx).ListRolePermissions(ctx)
	if err != nil {
		return nil, err
	}
	byRole := map[int64][]string{}
	for _, p := range perms {
		byRole[p.RoleID] = append(byRole[p.RoleID], p.Permission)
	}
	out := make([]domain.Role, len(rows))
	for i, row := range rows {
		out[i] = domain.Role{
			ID: row.ID, Name: row.Name, Description: row.Description, IsSystem: row.IsSystem,
			Permissions: nonNil(byRole[row.ID]), MemberCount: row.MemberCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
	}
	return out, nil
}

func (r *Repository) GetRole(ctx context.Context, company, id int64) (domain.Role, error) {
	row, err := r.q(ctx).GetRole(ctx, sqlc.GetRoleParams{CompanyID: company, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Role{}, application.ErrNotFound
	}
	if err != nil {
		return domain.Role{}, err
	}
	perms, err := r.q(ctx).ListPermissionsOfRole(ctx, id)
	if err != nil {
		return domain.Role{}, err
	}
	return domain.Role{
		ID: row.ID, Name: row.Name, Description: row.Description, IsSystem: row.IsSystem,
		Permissions: nonNil(perms), MemberCount: row.MemberCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *Repository) RoleIDByName(ctx context.Context, name string) (int64, error) {
	id, err := r.q(ctx).GetRoleByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, application.ErrNotFound
	}
	return id, err
}

// CreateRole inserts the role and its permissions in one transaction.
func (r *Repository) CreateRole(ctx context.Context, name, description string, permissions []string) (int64, error) {
	var id int64
	err := r.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		if id, err = q.CreateRole(ctx, sqlc.CreateRoleParams{Name: name, Description: description}); err != nil {
			return duplicate(err)
		}
		return setPermissions(ctx, q, id, permissions)
	})
	return id, err
}

// UpdateRole changes the role and replaces its permissions in one transaction.
func (r *Repository) UpdateRole(ctx context.Context, id int64, name, description string, permissions []string) error {
	return r.inTx(ctx, func(q *sqlc.Queries) error {
		if err := q.UpdateRole(ctx, sqlc.UpdateRoleParams{Name: name, Description: description, ID: id}); err != nil {
			return duplicate(err)
		}
		if err := q.DeleteRolePermissions(ctx, id); err != nil {
			return err
		}
		return setPermissions(ctx, q, id, permissions)
	})
}

func (r *Repository) DeleteRole(ctx context.Context, id int64) error {
	n, err := r.q(ctx).DeleteRole(ctx, id)
	if err == nil && n == 0 {
		return application.ErrNotFound
	}
	return err
}

func (r *Repository) Members(ctx context.Context, company, roleID int64) ([]application.Assignment, error) {
	rows, err := r.q(ctx).ListMembers(ctx, sqlc.ListMembersParams{RoleID: roleID, CompanyID: company})
	if err != nil {
		return nil, err
	}
	out := make([]application.Assignment, len(rows))
	for i, row := range rows {
		out[i] = application.Assignment{UserID: row.UserID, BranchID: row.BranchID}
	}
	return out, nil
}

func (r *Repository) AddMember(ctx context.Context, company, branch, roleID, userID int64) error {
	return r.q(ctx).AddMember(ctx, sqlc.AddMemberParams{UserID: userID, RoleID: roleID, CompanyID: company, BranchID: branch})
}

func (r *Repository) RemoveMember(ctx context.Context, company, branch, roleID, userID int64) error {
	n, err := r.q(ctx).RemoveMember(ctx, sqlc.RemoveMemberParams{UserID: userID, RoleID: roleID, CompanyID: company, BranchID: branch})
	if err == nil && n == 0 {
		return application.ErrNotFound
	}
	return err
}

func (r *Repository) LockMembers(ctx context.Context, company, roleID int64) ([]application.Assignment, error) {
	rows, err := r.q(ctx).LockMembers(ctx, sqlc.LockMembersParams{RoleID: roleID, CompanyID: company})
	if err != nil {
		return nil, err
	}
	out := make([]application.Assignment, len(rows))
	for i, row := range rows {
		out[i] = application.Assignment{UserID: row.UserID, BranchID: row.BranchID}
	}
	return out, nil
}

func (r *Repository) LockRole(ctx context.Context, id int64, exclusive bool) error {
	var err error
	if exclusive {
		_, err = r.q(ctx).LockRoleForUpdate(ctx, id)
	} else {
		_, err = r.q(ctx).LockRoleForShare(ctx, id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func (r *Repository) RoleCompanies(ctx context.Context, roleID int64) ([]int64, error) {
	return r.q(ctx).RoleCompanies(ctx, roleID)
}

func (r *Repository) GrantsOfUser(ctx context.Context, userID int64) ([]application.Grant, error) {
	rows, err := r.q(ctx).GrantsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]application.Grant, len(rows))
	for i, row := range rows {
		out[i] = application.Grant{CompanyID: row.CompanyID, BranchID: row.BranchID, Permission: row.Permission}
	}
	return out, nil
}

// inTx runs fn on queries inside d.Tx — joining the caller's transaction when there is one.
func (r *Repository) inTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	return r.tx.Do(ctx, func(ctx context.Context) error { return fn(r.q(ctx)) })
}

func setPermissions(ctx context.Context, q *sqlc.Queries, roleID int64, permissions []string) error {
	for _, p := range permissions {
		if err := q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: roleID, Permission: p}); err != nil {
			return err
		}
	}
	return nil
}

// duplicate maps MySQL's duplicate-key error on the role name to ErrNameTaken.
func duplicate(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return application.ErrNameTaken
	}
	return err
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

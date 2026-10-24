// Package infrastructure stores roles in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"

	"__GO_MODULE__/internal/modules/rbac/application"
	"__GO_MODULE__/internal/modules/rbac/domain"
	"__GO_MODULE__/internal/modules/rbac/infrastructure/sqlc"
)

// Repository implements application.Repository.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

func (r *Repository) ListRoles(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.q.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	perms, err := r.q.ListRolePermissions(ctx)
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

func (r *Repository) GetRole(ctx context.Context, id int64) (domain.Role, error) {
	row, err := r.q.GetRole(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Role{}, application.ErrNotFound
	}
	if err != nil {
		return domain.Role{}, err
	}
	perms, err := r.q.ListPermissionsOfRole(ctx, id)
	if err != nil {
		return domain.Role{}, err
	}
	return domain.Role{
		ID: row.ID, Name: row.Name, Description: row.Description, IsSystem: row.IsSystem,
		Permissions: nonNil(perms), MemberCount: row.MemberCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *Repository) RoleIDByName(ctx context.Context, name string) (int64, error) {
	id, err := r.q.GetRoleByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, application.ErrNotFound
	}
	return id, err
}

// CreateRole inserts the role and its permissions in one transaction.
func (r *Repository) CreateRole(ctx context.Context, name, description string, permissions []string) (int64, error) {
	var id int64
	err := r.tx(ctx, func(q *sqlc.Queries) error {
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
	return r.tx(ctx, func(q *sqlc.Queries) error {
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
	n, err := r.q.DeleteRole(ctx, id)
	if err == nil && n == 0 {
		return application.ErrNotFound
	}
	return err
}

func (r *Repository) Members(ctx context.Context, roleID int64) ([]int64, error) {
	return r.q.ListMembers(ctx, roleID)
}

func (r *Repository) AddMember(ctx context.Context, roleID, userID int64) error {
	return r.q.AddMember(ctx, sqlc.AddMemberParams{UserID: userID, RoleID: roleID})
}

func (r *Repository) RemoveMember(ctx context.Context, roleID, userID int64) error {
	n, err := r.q.RemoveMember(ctx, sqlc.RemoveMemberParams{UserID: userID, RoleID: roleID})
	if err == nil && n == 0 {
		return application.ErrNotFound
	}
	return err
}

func (r *Repository) PermissionsOfUser(ctx context.Context, userID int64) ([]string, error) {
	return r.q.PermissionsOfUser(ctx, userID)
}

func (r *Repository) tx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(r.q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
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

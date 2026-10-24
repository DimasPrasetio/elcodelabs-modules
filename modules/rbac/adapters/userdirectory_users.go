package modules

import (
	"context"
	"errors"

	rbacports "__GO_MODULE__/internal/modules/rbac/ports"
	userscontracts "__GO_MODULE__/internal/modules/users/contracts"
)

// rbacUserDirectory satisfies rbac's UserDirectory port with the users module's client.
// Mapping only: no logic lives here.
type rbacUserDirectory struct {
	users userscontracts.UserClient
}

func (a rbacUserDirectory) Get(ctx context.Context, id int64) (rbacports.User, error) {
	u, err := a.users.GetByID(ctx, id)
	return toRbacUser(u), rbacUserErr(err)
}

func (a rbacUserDirectory) FindByEmail(ctx context.Context, email string) (rbacports.User, error) {
	u, err := a.users.GetByEmail(ctx, email)
	return toRbacUser(u), rbacUserErr(err)
}

func (a rbacUserDirectory) Search(ctx context.Context, q string, limit int) ([]rbacports.User, error) {
	users, err := a.users.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]rbacports.User, len(users))
	for i, u := range users {
		out[i] = toRbacUser(u)
	}
	return out, nil
}

func toRbacUser(u userscontracts.User) rbacports.User {
	return rbacports.User{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive}
}

func rbacUserErr(err error) error {
	if errors.Is(err, userscontracts.ErrNotFound) {
		return rbacports.ErrUserNotFound
	}
	return err
}

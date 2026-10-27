package modules

import (
	"context"
	"errors"

	organizationports "__GO_MODULE__/internal/modules/organization/ports"
	userscontracts "__GO_MODULE__/internal/modules/users/contracts"
)

// organizationUserDirectory satisfies organization's UserDirectory port with the users module's
// client. Mapping only: no logic lives here.
type organizationUserDirectory struct {
	users userscontracts.UserClient
}

func (a organizationUserDirectory) Get(ctx context.Context, id int64) (organizationports.User, error) {
	u, err := a.users.GetByID(ctx, id)
	return toOrganizationUser(u), organizationUserErr(err)
}

func (a organizationUserDirectory) FindByEmail(ctx context.Context, email string) (organizationports.User, error) {
	u, err := a.users.GetByEmail(ctx, email)
	return toOrganizationUser(u), organizationUserErr(err)
}

func (a organizationUserDirectory) Search(ctx context.Context, q string, limit int) ([]organizationports.User, error) {
	users, err := a.users.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]organizationports.User, len(users))
	for i, u := range users {
		out[i] = toOrganizationUser(u)
	}
	return out, nil
}

func (a organizationUserDirectory) SearchAmong(ctx context.Context, ids []int64, q string, limit int) ([]organizationports.User, error) {
	users, err := a.users.SearchAmong(ctx, ids, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]organizationports.User, len(users))
	for i, u := range users {
		out[i] = toOrganizationUser(u)
	}
	return out, nil
}

func (a organizationUserDirectory) IDs(ctx context.Context) ([]int64, error) { return a.users.IDs(ctx) }

func toOrganizationUser(u userscontracts.User) organizationports.User {
	return organizationports.User{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive}
}

func organizationUserErr(err error) error {
	if errors.Is(err, userscontracts.ErrNotFound) {
		return organizationports.ErrUserNotFound
	}
	return err
}

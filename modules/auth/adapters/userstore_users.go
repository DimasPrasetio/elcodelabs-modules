package modules

import (
	"context"
	"errors"

	authports "__GO_MODULE__/internal/modules/auth/ports"
	userscontracts "__GO_MODULE__/internal/modules/users/contracts"
)

// authUserStore satisfies auth's UserStore port with the users module's client. Mapping only:
// no logic lives here.
type authUserStore struct {
	users userscontracts.UserClient
}

func (a authUserStore) FindCredentials(ctx context.Context, email string) (authports.Credentials, error) {
	c, err := a.users.GetCredentialsByEmail(ctx, email)
	if errors.Is(err, userscontracts.ErrNotFound) {
		return authports.Credentials{}, authports.ErrUserNotFound
	}
	if err != nil {
		return authports.Credentials{}, err
	}
	return authports.Credentials{UserID: c.UserID, PasswordHash: c.PasswordHash, IsActive: c.IsActive}, nil
}

func (a authUserStore) FindUser(ctx context.Context, id int64) (authports.User, error) {
	u, err := a.users.GetByID(ctx, id)
	if errors.Is(err, userscontracts.ErrNotFound) {
		return authports.User{}, authports.ErrUserNotFound
	}
	if err != nil {
		return authports.User{}, err
	}
	return authports.User{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive, PasswordChangedAt: u.PasswordChangedAt}, nil
}

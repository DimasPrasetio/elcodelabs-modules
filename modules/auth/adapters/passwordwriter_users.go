package modules

import (
	"context"
	"errors"
	"fmt"
	"strings"

	authports "__GO_MODULE__/internal/modules/auth/ports"
	userscontracts "__GO_MODULE__/internal/modules/users/contracts"
)

// authPasswordWriter satisfies auth's PasswordWriter port with the users module's client.
// Mapping only.
type authPasswordWriter struct {
	users userscontracts.UserClient
}

func (a authPasswordWriter) SetPassword(ctx context.Context, userID int64, password string) error {
	err := a.users.SetPassword(ctx, userID, password)
	if errors.Is(err, userscontracts.ErrInvalidPassword) {
		return fmt.Errorf("%w: %s", authports.ErrWeakPassword, strings.TrimPrefix(err.Error(), userscontracts.ErrInvalidPassword.Error()+": "))
	}
	return err
}

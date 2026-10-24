package modules

import (
	"context"
	"strings"

	"golang.org/x/crypto/bcrypt"

	authports "__GO_MODULE__/internal/modules/auth/ports"
)

// projectUserStore is what a project writes when its users do not come from the users
// module: here, one fixed account. The testbed uses it to prove auth runs on its port alone.
type projectUserStore struct{}

var projectHash, _ = bcrypt.GenerateFromPassword([]byte("fixture-pass-123"), bcrypt.MinCost)

func (projectUserStore) FindCredentials(_ context.Context, email string) (authports.Credentials, error) {
	if strings.EqualFold(email, "fixture@example.com") {
		return authports.Credentials{UserID: 7, PasswordHash: string(projectHash), IsActive: true}, nil
	}
	return authports.Credentials{}, authports.ErrUserNotFound
}

func (projectUserStore) FindUser(_ context.Context, id int64) (authports.User, error) {
	if id == 7 {
		return authports.User{ID: 7, Name: "Fixture User", Email: "fixture@example.com", IsActive: true}, nil
	}
	return authports.User{}, authports.ErrUserNotFound
}

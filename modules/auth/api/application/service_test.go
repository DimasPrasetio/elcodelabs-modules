package application_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"__GO_MODULE__/internal/modules/auth/application"
	"__GO_MODULE__/internal/modules/auth/ports"
	"__GO_MODULE__/internal/shared/apperror"
)

// fakeStore satisfies the UserStore port — the module is tested with no users module at all.
type fakeStore struct {
	users map[string]ports.Credentials
	byID  map[int64]ports.User
}

func newFakeStore(t *testing.T, email, password string, active bool) *fakeStore {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeStore{
		users: map[string]ports.Credentials{email: {UserID: 1, PasswordHash: string(hash), IsActive: active}},
		byID:  map[int64]ports.User{1: {ID: 1, Name: "Ana", Email: email, IsActive: active}},
	}
}

func (f *fakeStore) FindCredentials(_ context.Context, email string) (ports.Credentials, error) {
	c, ok := f.users[email]
	if !ok {
		return ports.Credentials{}, ports.ErrUserNotFound
	}
	return c, nil
}

func (f *fakeStore) FindUser(_ context.Context, id int64) (ports.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return ports.User{}, ports.ErrUserNotFound
	}
	return u, nil
}

func newService(store ports.UserStore) *application.Service {
	return application.NewService(store, application.NewTokens("test-secret-test-secret-test-secret", time.Hour), application.NewLimiter(3, time.Minute))
}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func TestLoginAndResolve(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	s, err := svc.Login(context.Background(), " Ana@Example.com ", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	u, err := svc.Resolve(context.Background(), s.Token)
	if err != nil || u.ID != 1 {
		t.Fatalf("resolve: user=%+v err=%v", u, err)
	}
}

func TestWrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	_, wrong := svc.Login(context.Background(), "ana@example.com", "nope-nope")
	_, unknown := svc.Login(context.Background(), "who@example.com", "nope-nope")
	if status(t, wrong) != http.StatusUnauthorized || status(t, unknown) != http.StatusUnauthorized {
		t.Fatalf("want 401 for both, got %v / %v", wrong, unknown)
	}
	if wrong.Error() != unknown.Error() {
		t.Fatalf("messages differ: %q vs %q", wrong, unknown)
	}
}

func TestDeactivatedUserCannotSignIn(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", false))
	_, err := svc.Login(context.Background(), "ana@example.com", "secret123")
	if got := status(t, err); got != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", got)
	}
}

func TestDeactivationEndsAnOpenSession(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc := newService(store)
	s, err := svc.Login(context.Background(), "ana@example.com", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	u := store.byID[1]
	u.IsActive = false
	store.byID[1] = u
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("want 401 after deactivation, got %v", err)
	}
}

func TestTooManyFailuresAreRefused(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	for i := 0; i < 3; i++ {
		_, _ = svc.Login(context.Background(), "ana@example.com", "wrong-pass")
	}
	_, err := svc.Login(context.Background(), "ana@example.com", "secret123")
	if got := status(t, err); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
}

func TestTamperedTokenIsRejected(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	s, _ := svc.Login(context.Background(), "ana@example.com", "secret123")
	other := application.NewService(newFakeStore(t, "ana@example.com", "secret123", true),
		application.NewTokens("another-secret-another-secret-xx", time.Hour), application.NewLimiter(3, time.Minute))
	if _, err := other.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("token signed with another secret was accepted: %v", err)
	}
}

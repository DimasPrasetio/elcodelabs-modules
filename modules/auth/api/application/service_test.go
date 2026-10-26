package application_test

import (
	"context"
	"net/http"
	"sync"
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

// fakeSessions keeps sessions in memory.
type fakeSessions struct {
	mu   sync.Mutex
	rows map[string]application.SessionRecord
}

func (f *fakeSessions) Create(_ context.Context, r application.SessionRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[r.ID] = r
	return nil
}

func (f *fakeSessions) Get(_ context.Context, id string) (application.SessionRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok {
		return application.SessionRecord{}, application.ErrNoSession
	}
	return r, nil
}

func (f *fakeSessions) Revoke(_ context.Context, id string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.rows[id]
	r.Revoked = true
	f.rows[id] = r
	return nil
}

func (f *fakeSessions) DeleteExpired(context.Context, time.Time) error { return nil }

const secret = "test-secret-test-secret-test-secret"

func newService(store ports.UserStore) *application.Service {
	return application.NewService(store, &fakeSessions{rows: map[string]application.SessionRecord{}},
		application.NewTokens(secret, time.Hour, "app"), application.NewLimiter(time.Minute, 1000))
}

var office = application.Client{IP: "203.0.113.7"}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func login(t *testing.T, svc *application.Service, email, password string) application.Session {
	t.Helper()
	s, err := svc.Login(context.Background(), email, password, office)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLoginAndResolve(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	s := login(t, svc, " Ana@Example.com ", "secret123")
	u, err := svc.Resolve(context.Background(), s.Token)
	if err != nil || u.ID != 1 {
		t.Fatalf("resolve: user=%+v err=%v", u, err)
	}
}

func TestWrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	_, wrong := svc.Login(context.Background(), "ana@example.com", "nope-nope", office)
	_, unknown := svc.Login(context.Background(), "who@example.com", "nope-nope", office)
	if status(t, wrong) != http.StatusUnauthorized || status(t, unknown) != http.StatusUnauthorized {
		t.Fatalf("want 401 for both, got %v / %v", wrong, unknown)
	}
	if wrong.Error() != unknown.Error() {
		t.Fatalf("messages differ: %q vs %q", wrong, unknown)
	}
}

func TestDeactivatedUserCannotSignIn(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", false))
	_, err := svc.Login(context.Background(), "ana@example.com", "secret123", office)
	if got := status(t, err); got != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", got)
	}
}

func TestDeactivationEndsAnOpenSession(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc := newService(store)
	s := login(t, svc, "ana@example.com", "secret123")
	u := store.byID[1]
	u.IsActive = false
	store.byID[1] = u
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("want 401 after deactivation, got %v", err)
	}
}

func TestLogoutEndsTheTokenEverywhere(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	s := login(t, svc, "ana@example.com", "secret123")
	other := login(t, svc, "ana@example.com", "secret123") // another device
	if err := svc.Logout(context.Background(), s.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a signed-out token (e.g. copied before) still works: %v", err)
	}
	if _, err := svc.Resolve(context.Background(), other.Token); err != nil {
		t.Fatalf("signing out one device ended another: %v", err)
	}
}

func TestPasswordChangeEndsEarlierSessions(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc := newService(store)
	s := login(t, svc, "ana@example.com", "secret123")
	u := store.byID[1]
	u.PasswordChangedAt = time.Now().Add(time.Minute)
	store.byID[1] = u
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a session from before the password change still works: %v", err)
	}
}

func TestPasswordChangeRightAfterSignInStillEndsTheSession(t *testing.T) {
	store := newFakeStore(t, "ana@example.com", "secret123", true)
	svc := newService(store)
	s := login(t, svc, "ana@example.com", "secret123")
	u := store.byID[1]
	u.PasswordChangedAt = time.Now() // the same second as the sign-in
	store.byID[1] = u
	if _, err := svc.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
		t.Fatalf("a session opened just before the password change survived: %v", err)
	}
}

func TestTooManyFailuresAreRefused(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(context.Background(), "ana@example.com", "wrong-pass", office)
	}
	_, err := svc.Login(context.Background(), "ana@example.com", "secret123", office)
	if got := status(t, err); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
	// The same person from another address is not locked out by someone else's guesses here.
	if _, err := svc.Login(context.Background(), "ana@example.com", "secret123", application.Client{IP: "198.51.100.9"}); err != nil {
		t.Fatalf("another address was locked out: %v", err)
	}
}

func TestParallelGuessesCannotSlipPastTheLimit(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	var wg sync.WaitGroup
	var mu sync.Mutex
	tried := 0
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Login(context.Background(), "ana@example.com", "wrong-pass", office)
			if e, ok := apperror.As(err); ok && e.Status == http.StatusUnauthorized {
				mu.Lock()
				tried++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if tried > 5 {
		t.Fatalf("%d passwords were checked in parallel, want at most 5", tried)
	}
}

func TestSprayingFromOneAddressIsLimited(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	refused := false
	for i := 0; i < 30; i++ {
		_, err := svc.Login(context.Background(), string(rune('a'+i%26))+"x@example.com", "wrong-pass", office)
		if e, ok := apperror.As(err); ok && e.Status == http.StatusTooManyRequests {
			refused = true
		}
	}
	if !refused {
		t.Fatal("30 accounts guessed from one address without being refused")
	}
}

func TestLimiterMemoryIsBounded(t *testing.T) {
	l := application.NewLimiter(time.Minute, 10)
	for i := 0; i < 100; i++ {
		l.Reserve(application.Rule{Key: string(rune(1000 + i)), Max: 5})
	}
	if l.Reserve(application.Rule{Key: "fresh", Max: 5}) {
		t.Fatal("a new key was accepted past MaxKeys")
	}
}

func TestOversizedEmailIsRefusedBeforeTheLimiter(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := svc.Login(context.Background(), string(long)+"@example.com", "x", office); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for an oversized email, got %v", err)
	}
}

func TestTokenFromAnotherAppOrSecretIsRejected(t *testing.T) {
	svc := newService(newFakeStore(t, "ana@example.com", "secret123", true))
	s := login(t, svc, "ana@example.com", "secret123")
	for name, tokens := range map[string]*application.Tokens{
		"other secret": application.NewTokens("another-secret-another-secret-xx", time.Hour, "app"),
		"other app":    application.NewTokens(secret, time.Hour, "another-app"),
	} {
		other := application.NewService(newFakeStore(t, "ana@example.com", "secret123", true),
			&fakeSessions{rows: map[string]application.SessionRecord{}}, tokens, application.NewLimiter(time.Minute, 1000))
		if _, err := other.Resolve(context.Background(), s.Token); status(t, err) != http.StatusUnauthorized {
			t.Fatalf("%s: token was accepted: %v", name, err)
		}
	}
}

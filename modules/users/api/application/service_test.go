package application_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/users/application"
	"__GO_MODULE__/internal/modules/users/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
)

// memRepo is an in-memory Repository: the module is tested alone, without MySQL.
type memRepo struct {
	users  map[int64]domain.User
	hashes map[int64]string
	nextID int64
}

func newMemRepo() *memRepo {
	return &memRepo{users: map[int64]domain.User{}, hashes: map[int64]string{}}
}

func (m *memRepo) List(_ context.Context, _ string, _ pagination.Params) ([]domain.User, int64, error) {
	var out []domain.User
	for _, u := range m.users {
		out = append(out, u)
	}
	return out, int64(len(out)), nil
}

func (m *memRepo) CountAll(context.Context) (int64, error) { return int64(len(m.users)), nil }

func (m *memRepo) Get(_ context.Context, id int64) (domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return domain.User{}, application.ErrNotFound
	}
	return u, nil
}

func (m *memRepo) GetByEmail(_ context.Context, email string) (domain.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return domain.User{}, application.ErrNotFound
}

func (m *memRepo) GetCredentialsByEmail(_ context.Context, email string) (application.Credentials, error) {
	for id, u := range m.users {
		if u.Email == email {
			return application.Credentials{UserID: id, PasswordHash: m.hashes[id], IsActive: u.IsActive}, nil
		}
	}
	return application.Credentials{}, application.ErrNotFound
}

func (m *memRepo) Create(_ context.Context, n application.NewUser) (int64, error) {
	for _, u := range m.users {
		if u.Email == n.Email {
			return 0, application.ErrEmailTaken
		}
	}
	m.nextID++
	m.users[m.nextID] = domain.User{ID: m.nextID, Name: n.Name, Email: n.Email, IsActive: n.IsActive}
	m.hashes[m.nextID] = n.PasswordHash
	return m.nextID, nil
}

func (m *memRepo) Update(_ context.Context, u domain.User) error {
	if _, ok := m.users[u.ID]; !ok {
		return application.ErrNotFound
	}
	for id, other := range m.users {
		if id != u.ID && other.Email == u.Email {
			return application.ErrEmailTaken
		}
	}
	m.users[u.ID] = u
	return nil
}

func (m *memRepo) UpdateName(_ context.Context, id int64, name string) error {
	u, ok := m.users[id]
	if !ok {
		return application.ErrNotFound
	}
	u.Name = name
	m.users[id] = u
	return nil
}

func (m *memRepo) UpdatePassword(_ context.Context, id int64, hash string, at time.Time) error {
	m.hashes[id] = hash
	u := m.users[id]
	u.PasswordChangedAt = at
	m.users[id] = u
	return nil
}

// guard covers every account except those listed (the "administrators" an actor cannot touch).
type guard map[int64]bool

func (g guard) Covers(_ context.Context, target int64) bool { return !g[target] }

// audit keeps what was recorded.
type audit struct{ got []kernel.AuditEntry }

func (a *audit) Record(_ context.Context, e kernel.AuditEntry) { a.got = append(a.got, e) }

func (a *audit) actions() []string {
	out := make([]string, len(a.got))
	for i, e := range a.got {
		out[i] = e.Action
	}
	return out
}

func newService(repo application.Repository, protected ...int64) *application.Service {
	svc, _ := newAudited(repo, protected...)
	return svc
}

func newAudited(repo application.Repository, protected ...int64) (*application.Service, *audit) {
	g := guard{}
	for _, id := range protected {
		g[id] = true
	}
	a := &audit{}
	return application.NewService(repo, plainHasher{}, g, a), a
}

func boolPtr(b bool) *bool { return &b }

type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error) { return "hash:" + p, nil }

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func TestCreateNormalizesEmailAndHashes(t *testing.T) {
	repo := newMemRepo()
	svc := newService(repo)
	u, err := svc.Create(context.Background(), application.CreateInput{Name: "Ana", Email: "  Ana@Example.COM ", Password: "secret123", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@example.com" {
		t.Errorf("email = %q, want normalized", u.Email)
	}
	if repo.hashes[u.ID] != "hash:secret123" {
		t.Errorf("password stored as %q, want the hash", repo.hashes[u.ID])
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	svc := newService(newMemRepo())
	_, err := svc.Create(context.Background(), application.CreateInput{Name: "", Email: "nope", Password: "short"})
	if got := status(t, err); got != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", got)
	}
}

func TestCreateDuplicateEmailIsAFieldError(t *testing.T) {
	svc := newService(newMemRepo())
	in := application.CreateInput{Name: "Ana", Email: "ana@example.com", Password: "secret123", IsActive: true}
	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(context.Background(), in)
	if got := status(t, err); got != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", got)
	}
}

func TestCannotDeactivateYourself(t *testing.T) {
	svc := newService(newMemRepo())
	u, _ := svc.Create(context.Background(), application.CreateInput{Name: "Ana", Email: "ana@example.com", Password: "secret123", IsActive: true})
	_, err := svc.Update(context.Background(), u.ID, u.ID, application.UpdateInput{Name: "Ana", Email: u.Email, IsActive: boolPtr(false)})
	if got := status(t, err); got != http.StatusConflict {
		t.Errorf("deactivate self: status = %d, want 409", got)
	}
}

func TestCannotTouchAMorePrivilegedAccount(t *testing.T) {
	repo := newMemRepo()
	admin := create(t, newService(repo), "admin@example.com")
	operator := create(t, newService(repo), "op@example.com")
	svc := newService(repo, admin.ID) // the operator does not cover the admin
	for name, in := range map[string]application.UpdateInput{
		"reset password": {Name: "Admin", Email: admin.Email, Password: "taken-over-1"},
		"change email":   {Name: "Admin", Email: "op2@example.com"},
		"deactivate":     {Name: "Admin", Email: admin.Email, IsActive: boolPtr(false)},
	} {
		if _, err := svc.Update(context.Background(), operator.ID, admin.ID, in); status(t, err) != http.StatusForbidden {
			t.Errorf("%s: want 403, got %v", name, err)
		}
	}
	if repo.hashes[admin.ID] != "hash:secret123" {
		t.Fatal("the administrator's password changed")
	}
}

func TestOmittedIsActiveKeepsTheState(t *testing.T) {
	repo := newMemRepo()
	svc := newService(repo)
	u := create(t, svc, "ana@example.com")
	got, err := svc.Update(context.Background(), 99, u.ID, application.UpdateInput{Name: "Ana B", Email: u.Email})
	if err != nil || !got.IsActive {
		t.Fatalf("an update without is_active deactivated the user: %+v %v", got, err)
	}
}

func TestPasswordChangeIsRecorded(t *testing.T) {
	repo := newMemRepo()
	svc := newService(repo)
	u := create(t, svc, "ana@example.com")
	if _, err := svc.Update(context.Background(), 99, u.ID, application.UpdateInput{Name: "Ana", Email: u.Email, Password: "new-secret-1"}); err != nil {
		t.Fatal(err)
	}
	if repo.users[u.ID].PasswordChangedAt.IsZero() {
		t.Fatal("PasswordChangedAt not set: earlier sessions would survive the change")
	}
}

func create(t *testing.T, svc *application.Service, email string) domain.User {
	t.Helper()
	u, err := svc.Create(context.Background(), application.CreateInput{Name: "User", Email: email, Password: "secret123", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestBootstrapOnlyWhenEmpty(t *testing.T) {
	svc := newService(newMemRepo())
	created, err := svc.Bootstrap(context.Background(), "Admin", "admin@example.com", "secret123")
	if err != nil || !created {
		t.Fatalf("first bootstrap: created=%v err=%v", created, err)
	}
	created, err = svc.Bootstrap(context.Background(), "Admin", "other@example.com", "secret123")
	if err != nil || created {
		t.Fatalf("second bootstrap: created=%v err=%v, want no-op", created, err)
	}
}

func TestGetMissingIs404(t *testing.T) {
	svc := newService(newMemRepo())
	_, err := svc.Get(context.Background(), 99)
	if got := status(t, err); got != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", got)
	}
}

func TestSetPasswordForAnAuthProvider(t *testing.T) {
	repo := newMemRepo()
	svc := newService(repo)
	u := create(t, svc, "ana@example.com")
	if err := svc.SetPassword(context.Background(), u.ID, "short"); !errors.Is(err, application.ErrInvalidPassword) || err.Error() != "invalid password: minimal 8 karakter" {
		t.Fatalf("a short password must be refused with the rule it breaks, got %v", err)
	}
	if err := svc.SetPassword(context.Background(), u.ID, "new-secret-1"); err != nil {
		t.Fatal(err)
	}
	if repo.hashes[u.ID] != "hash:new-secret-1" || repo.users[u.ID].PasswordChangedAt.IsZero() {
		t.Fatal("password not replaced, or its change not recorded")
	}
}

func TestOwnProfileChangesTheNameOnly(t *testing.T) {
	repo := newMemRepo()
	svc := newService(repo)
	u := create(t, svc, "ana@example.com")
	got, err := svc.UpdateProfile(context.Background(), u.ID, "Ana Baru")
	if err != nil || got.Name != "Ana Baru" || got.Email != "ana@example.com" {
		t.Fatalf("profile: %+v %v", got, err)
	}
	if _, err := svc.UpdateProfile(context.Background(), u.ID, ""); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatal("an empty name must be refused")
	}
}

func TestChangesAreAudited(t *testing.T) {
	repo := newMemRepo()
	svc, a := newAudited(repo)
	u := create(t, svc, "ana@example.com")
	_, _ = svc.Update(context.Background(), 99, u.ID, application.UpdateInput{Name: "Ana", Email: u.Email, Password: "new-secret-1", IsActive: boolPtr(false)})
	got := strings.Join(a.actions(), ",")
	if got != "user.created,user.deactivated,user.updated" {
		t.Fatalf("audit = %s", got)
	}
	for _, e := range a.got {
		if strings.Contains(fmt.Sprint(e.Data), "new-secret-1") {
			t.Fatal("a password reached the audit trail")
		}
	}
}

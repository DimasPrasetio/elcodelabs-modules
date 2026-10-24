package application_test

import (
	"context"
	"net/http"
	"testing"

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

func (m *memRepo) UpdatePassword(_ context.Context, id int64, hash string) error {
	m.hashes[id] = hash
	return nil
}

func (m *memRepo) Delete(_ context.Context, id int64) error {
	if _, ok := m.users[id]; !ok {
		return application.ErrNotFound
	}
	delete(m.users, id)
	return nil
}

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
	svc := application.NewService(repo, plainHasher{})
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
	svc := application.NewService(newMemRepo(), plainHasher{})
	_, err := svc.Create(context.Background(), application.CreateInput{Name: "", Email: "nope", Password: "short"})
	if got := status(t, err); got != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", got)
	}
}

func TestCreateDuplicateEmailIsAFieldError(t *testing.T) {
	svc := application.NewService(newMemRepo(), plainHasher{})
	in := application.CreateInput{Name: "Ana", Email: "ana@example.com", Password: "secret123", IsActive: true}
	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(context.Background(), in)
	if got := status(t, err); got != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", got)
	}
}

func TestCannotDeactivateOrDeleteYourself(t *testing.T) {
	svc := application.NewService(newMemRepo(), plainHasher{})
	u, _ := svc.Create(context.Background(), application.CreateInput{Name: "Ana", Email: "ana@example.com", Password: "secret123", IsActive: true})
	_, err := svc.Update(context.Background(), u.ID, u.ID, application.UpdateInput{Name: "Ana", Email: u.Email, IsActive: false})
	if got := status(t, err); got != http.StatusConflict {
		t.Errorf("deactivate self: status = %d, want 409", got)
	}
	if got := status(t, svc.Delete(context.Background(), u.ID, u.ID)); got != http.StatusConflict {
		t.Errorf("delete self: status = %d, want 409", got)
	}
}

func TestBootstrapOnlyWhenEmpty(t *testing.T) {
	svc := application.NewService(newMemRepo(), plainHasher{})
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
	svc := application.NewService(newMemRepo(), plainHasher{})
	_, err := svc.Get(context.Background(), 99)
	if got := status(t, err); got != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", got)
	}
}

package application_test

import (
	"context"
	"net/http"
	"sort"
	"testing"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/rbac/application"
	"__GO_MODULE__/internal/modules/rbac/domain"
	"__GO_MODULE__/internal/modules/rbac/ports"
	"__GO_MODULE__/internal/shared/apperror"
)

// memRepo is an in-memory Repository seeded like the migration: Administrator holds "*".
type memRepo struct {
	roles   map[int64]domain.Role
	members map[int64]map[int64]bool // role → users
	nextID  int64
}

func newMemRepo() *memRepo {
	return &memRepo{
		roles:   map[int64]domain.Role{1: {ID: 1, Name: domain.AdminRole, IsSystem: true, Permissions: []string{domain.Wildcard}}},
		members: map[int64]map[int64]bool{1: {}},
		nextID:  1,
	}
}

func (m *memRepo) ListRoles(context.Context) ([]domain.Role, error) {
	var out []domain.Role
	for _, r := range m.roles {
		out = append(out, r)
	}
	return out, nil
}

func (m *memRepo) GetRole(_ context.Context, id int64) (domain.Role, error) {
	r, ok := m.roles[id]
	if !ok {
		return domain.Role{}, application.ErrNotFound
	}
	r.MemberCount = int64(len(m.members[id]))
	return r, nil
}

func (m *memRepo) RoleIDByName(_ context.Context, name string) (int64, error) {
	for id, r := range m.roles {
		if r.Name == name {
			return id, nil
		}
	}
	return 0, application.ErrNotFound
}

func (m *memRepo) CreateRole(_ context.Context, name, desc string, perms []string) (int64, error) {
	for _, r := range m.roles {
		if r.Name == name {
			return 0, application.ErrNameTaken
		}
	}
	m.nextID++
	m.roles[m.nextID] = domain.Role{ID: m.nextID, Name: name, Description: desc, Permissions: perms}
	m.members[m.nextID] = map[int64]bool{}
	return m.nextID, nil
}

func (m *memRepo) UpdateRole(_ context.Context, id int64, name, desc string, perms []string) error {
	r := m.roles[id]
	r.Name, r.Description, r.Permissions = name, desc, perms
	m.roles[id] = r
	return nil
}

func (m *memRepo) DeleteRole(_ context.Context, id int64) error {
	delete(m.roles, id)
	delete(m.members, id)
	return nil
}

func (m *memRepo) Members(_ context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	for id := range m.members[roleID] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (m *memRepo) AddMember(_ context.Context, roleID, userID int64) error {
	m.members[roleID][userID] = true
	return nil
}

func (m *memRepo) RemoveMember(_ context.Context, roleID, userID int64) error {
	if !m.members[roleID][userID] {
		return application.ErrNotFound
	}
	delete(m.members[roleID], userID)
	return nil
}

func (m *memRepo) PermissionsOfUser(_ context.Context, userID int64) ([]string, error) {
	var out []string
	for roleID, users := range m.members {
		if users[userID] {
			out = append(out, m.roles[roleID].Permissions...)
		}
	}
	return out, nil
}

// directory is a fake UserDirectory: the module is tested with no users module at all.
type directory map[int64]ports.User

func (d directory) Get(_ context.Context, id int64) (ports.User, error) {
	u, ok := d[id]
	if !ok {
		return ports.User{}, ports.ErrUserNotFound
	}
	return u, nil
}

func (d directory) FindByEmail(_ context.Context, email string) (ports.User, error) {
	for _, u := range d {
		if u.Email == email {
			return u, nil
		}
	}
	return ports.User{}, ports.ErrUserNotFound
}

func (d directory) Search(context.Context, string, int) ([]ports.User, error) { return nil, nil }

var catalog = func() []kernel.Permission {
	return []kernel.Permission{{Name: "rbac.manage"}, {Name: "users.manage"}, {Name: "users.read"}}
}

func setup() (*application.Service, *memRepo) {
	repo := newMemRepo()
	users := directory{
		1: {ID: 1, Email: "admin@example.com", IsActive: true},
		2: {ID: 2, Email: "ana@example.com", IsActive: true},
		3: {ID: 3, Email: "budi@example.com", IsActive: true},
	}
	return application.NewService(repo, users, catalog, noAudit{}), repo
}

// admin is the bootstrapped Administrator; tests act as them unless they say otherwise.
const admin = int64(1)

func setupWithAdmin(t *testing.T) (*application.Service, *memRepo) {
	t.Helper()
	svc, repo := setup()
	if ok, err := svc.Bootstrap(context.Background(), "admin@example.com"); err != nil || !ok {
		t.Fatalf("bootstrap: %v %v", ok, err)
	}
	return svc, repo
}

type noAudit struct{}

func (noAudit) Record(context.Context, kernel.AuditEntry) {}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func TestBootstrapThenAdministratorCanEverything(t *testing.T) {
	svc, _ := setup()
	ctx := context.Background()
	if ok, err := svc.Bootstrap(ctx, "admin@example.com"); err != nil || !ok {
		t.Fatalf("bootstrap: %v %v", ok, err)
	}
	if ok, _ := svc.Bootstrap(ctx, "ana@example.com"); ok {
		t.Fatal("second bootstrap must be a no-op once someone is Administrator")
	}
	if !svc.Can(ctx, 1, "users.manage") || !svc.Can(ctx, 1, "anything.declared.later") {
		t.Fatal("Administrator's * must grant every permission")
	}
	if svc.Can(ctx, 2, "users.read") {
		t.Fatal("a user without roles must hold nothing")
	}
}

func TestRoleGrantsItsPermissionsOnly(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	role, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Viewer", Permissions: []string{"users.read", "users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(role.Permissions) != 1 {
		t.Fatalf("duplicates must collapse: %v", role.Permissions)
	}
	if err := svc.AddMember(ctx, admin, role.ID, 2); err != nil {
		t.Fatal(err)
	}
	if !svc.Can(ctx, 2, "users.read") || svc.Can(ctx, 2, "users.manage") {
		t.Fatal("Viewer must read users and nothing more")
	}
	got, _ := svc.Effective(ctx, 2)
	if len(got) != 1 || got[0] != "users.read" {
		t.Fatalf("effective = %v", got)
	}
}

func TestUnknownPermissionAndReservedNameAreRejected(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	_, err := svc.CreateRole(context.Background(), admin, application.RoleInput{Name: "X", Permissions: []string{"nope.never"}})
	if status(t, err) != http.StatusUnprocessableEntity {
		t.Fatal("unknown permission must be a 422")
	}
	_, err = svc.CreateRole(context.Background(), admin, application.RoleInput{Name: "administrator"})
	if status(t, err) != http.StatusUnprocessableEntity {
		t.Fatal("the Administrator name is reserved")
	}
}

func TestAdministratorRoleIsProtected(t *testing.T) {
	svc, _ := setup()
	ctx := context.Background()
	_, _ = svc.Bootstrap(ctx, "admin@example.com")
	if _, err := svc.UpdateRole(ctx, admin, 1, application.RoleInput{Name: "Root"}); status(t, err) != http.StatusConflict {
		t.Error("Administrator must not change")
	}
	if err := svc.DeleteRole(ctx, admin, 1); status(t, err) != http.StatusConflict {
		t.Error("Administrator must not be deleted")
	}
	if err := svc.RemoveMember(ctx, admin, 1, 1); status(t, err) != http.StatusConflict {
		t.Error("the last Administrator must keep the role")
	}
}

func TestChangesApplyImmediately(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	role, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Editor", Permissions: []string{"users.manage"}})
	_ = svc.AddMember(ctx, admin, role.ID, 2)
	if !svc.Can(ctx, 2, "users.manage") {
		t.Fatal("granted")
	}
	if err := svc.RemoveMember(ctx, admin, role.ID, 2); err != nil {
		t.Fatal(err)
	}
	if svc.Can(ctx, 2, "users.manage") {
		t.Fatal("the cache must not outlive a change")
	}
}

func TestUnknownUserCannotGetARole(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	if err := svc.AddMember(context.Background(), admin, 1, 99); status(t, err) != http.StatusNotFound {
		t.Fatal("want 404 for a user the directory does not know")
	}
}

// A role manager holds rbac.manage and users.read — but not users.manage, and is no administrator.
func roleManager(t *testing.T, svc *application.Service) int64 {
	t.Helper()
	ctx := context.Background()
	role, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Role manager", Permissions: []string{"rbac.manage", "users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, admin, role.ID, 2); err != nil {
		t.Fatal(err)
	}
	return 2
}

func TestRbacManageIsNoPathToMorePermissions(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	mgr := roleManager(t, svc)
	if err := svc.AddMember(ctx, mgr, 1, mgr); status(t, err) != http.StatusForbidden {
		t.Errorf("making yourself Administrator: want 403, got %v", err)
	}
	if _, err := svc.CreateRole(ctx, mgr, application.RoleInput{Name: "Mine", Permissions: []string{"users.manage"}}); status(t, err) != http.StatusForbidden {
		t.Errorf("a role with a permission you lack: want 403, got %v", err)
	}
	editor, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Editor", Permissions: []string{"users.manage"}})
	if err := svc.AddMember(ctx, mgr, editor.ID, mgr); status(t, err) != http.StatusForbidden {
		t.Errorf("joining a stronger role: want 403, got %v", err)
	}
	if _, err := svc.UpdateRole(ctx, mgr, editor.ID, application.RoleInput{Name: "Editor", Permissions: []string{"users.read"}}); status(t, err) != http.StatusForbidden {
		t.Errorf("editing a stronger role: want 403, got %v", err)
	}
	_ = svc.AddMember(ctx, admin, 1, 3) // a second administrator
	if err := svc.RemoveMember(ctx, mgr, 1, 3); status(t, err) != http.StatusForbidden {
		t.Errorf("demoting an administrator: want 403, got %v", err)
	}
	if svc.Can(ctx, mgr, "users.manage") {
		t.Fatal("the role manager gained users.manage")
	}
}

func TestCovers(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	mgr := roleManager(t, svc)
	if svc.Covers(ctx, mgr, admin) {
		t.Error("a role manager must not cover the administrator")
	}
	if !svc.Covers(ctx, admin, mgr) {
		t.Error("the administrator covers everyone")
	}
	if !svc.Covers(ctx, mgr, 3) {
		t.Error("anyone covers a user with no permissions")
	}
}

func TestBootstrapReturnsWhenNoActiveAdministratorIsLeft(t *testing.T) {
	repo := newMemRepo()
	users := directory{1: {ID: 1, Email: "admin@example.com", IsActive: true}}
	svc := application.NewService(repo, users, catalog, noAudit{})
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("first bootstrap")
	}
	delete(users, 1) // the only administrator is gone (removed from the user store)
	users[2] = ports.User{ID: 2, Email: "new@example.com", IsActive: true}
	if ok, err := svc.Bootstrap(ctx, "new@example.com"); err != nil || !ok {
		t.Fatalf("bootstrap must give the role again when no active administrator is left: %v %v", ok, err)
	}
}

package application_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
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
	members map[int64]map[unit]bool // role → assignments
	nextID  int64
}

type unit struct{ user, company, branch int64 }

func newMemRepo() *memRepo {
	return &memRepo{
		roles:   map[int64]domain.Role{1: {ID: 1, Name: domain.AdminRole, IsSystem: true, Permissions: []string{domain.Wildcard}}},
		members: map[int64]map[unit]bool{1: {}},
		nextID:  1,
	}
}

func (m *memRepo) ListRoles(_ context.Context, company int64) ([]domain.Role, error) {
	var out []domain.Role
	for id := range m.roles {
		r, _ := m.GetRole(context.Background(), company, id)
		out = append(out, r)
	}
	return out, nil
}

func (m *memRepo) GetRole(_ context.Context, company, id int64) (domain.Role, error) {
	r, ok := m.roles[id]
	if !ok {
		return domain.Role{}, application.ErrNotFound
	}
	r.MemberCount = 0
	for u := range m.members[id] {
		if u.company == company {
			r.MemberCount++
		}
	}
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
	m.members[m.nextID] = map[unit]bool{}
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

func (m *memRepo) Members(_ context.Context, company, roleID int64) ([]application.Assignment, error) {
	var out []application.Assignment
	for u := range m.members[roleID] {
		if u.company == company {
			out = append(out, application.Assignment{UserID: u.user, BranchID: u.branch})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out, nil
}

func (m *memRepo) AddMember(_ context.Context, company, branch, roleID, userID int64) error {
	m.members[roleID][unit{userID, company, branch}] = true
	return nil
}

func (m *memRepo) RemoveMember(_ context.Context, company, branch, roleID, userID int64) error {
	k := unit{userID, company, branch}
	if !m.members[roleID][k] {
		return application.ErrNotFound
	}
	delete(m.members[roleID], k)
	return nil
}

func (m *memRepo) LockMembers(ctx context.Context, company, roleID int64) ([]application.Assignment, error) {
	return m.Members(ctx, company, roleID)
}

func (m *memRepo) LockRole(_ context.Context, id int64, _ bool) error {
	if _, ok := m.roles[id]; !ok {
		return application.ErrNotFound
	}
	return nil
}

func (m *memRepo) RoleCompanies(_ context.Context, roleID int64) ([]int64, error) {
	seen := map[int64]bool{}
	var out []int64
	for u := range m.members[roleID] {
		if !seen[u.company] {
			seen[u.company] = true
			out = append(out, u.company)
		}
	}
	return out, nil
}

func (m *memRepo) GrantsOfUser(_ context.Context, userID int64) ([]application.Grant, error) {
	var out []application.Grant
	for roleID, units := range m.members {
		for u := range units {
			if u.user == userID {
				for _, p := range m.roles[roleID].Permissions {
					out = append(out, application.Grant{CompanyID: u.company, BranchID: u.branch, Permission: p})
				}
			}
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

func (d directory) Search(_ context.Context, q string, _ int) ([]ports.User, error) {
	var out []ports.User
	for _, u := range d {
		if strings.Contains(u.Email, q) || strings.Contains(u.Name, q) {
			out = append(out, u)
		}
	}
	return out, nil
}

var catalog = func() []kernel.Permission {
	return []kernel.Permission{{Name: "rbac.manage"}, {Name: "users.manage"}, {Name: "users.read"}}
}

var people = directory{
	1: {ID: 1, Email: "admin@example.com", IsActive: true},
	2: {ID: 2, Email: "ana@example.com", IsActive: true},
	3: {ID: 3, Email: "budi@example.com", IsActive: true},
}

func setup() (*application.Service, *memRepo) {
	repo := newMemRepo()
	return application.NewService(repo, tx, everywhere{}, people, catalog, noAudit{}), repo
}

// fresh is a new service over repo — grants are cached, and these tests change repo directly.
func fresh(repo *memRepo) *application.Service {
	return application.NewService(repo, tx, everywhere{}, people, catalog, noAudit{})
}

// everywhere places every user in every company and branch (an organization module's answer).
type everywhere struct{}

func (everywhere) Resolve(_ context.Context, _ int64, requested string) (kernel.Scope, error) {
	var sc kernel.Scope
	c, b, _ := strings.Cut(requested, ":")
	sc.CompanyID, _ = strconv.ParseInt(c, 10, 64)
	sc.BranchID, _ = strconv.ParseInt(b, 10, 64)
	return sc, nil
}

// tx is the kernel's d.Tx without a database: fn runs, hooks follow.
var tx = kernel.NewTxSlot(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))

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
	if err := svc.AddMember(ctx, admin, role.ID, 2, 0); err != nil {
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
	if err := svc.RemoveMember(ctx, admin, 1, 1, 0); status(t, err) != http.StatusConflict {
		t.Error("the last Administrator must keep the role")
	}
}

func TestChangesApplyImmediately(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	role, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Editor", Permissions: []string{"users.manage"}})
	_ = svc.AddMember(ctx, admin, role.ID, 2, 0)
	if !svc.Can(ctx, 2, "users.manage") {
		t.Fatal("granted")
	}
	if err := svc.RemoveMember(ctx, admin, role.ID, 2, 0); err != nil {
		t.Fatal(err)
	}
	if svc.Can(ctx, 2, "users.manage") {
		t.Fatal("the cache must not outlive a change")
	}
}

func TestUnknownUserCannotGetARole(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	if err := svc.AddMember(context.Background(), admin, 1, 99, 0); status(t, err) != http.StatusNotFound {
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
	if err := svc.AddMember(ctx, admin, role.ID, 2, 0); err != nil {
		t.Fatal(err)
	}
	return 2
}

func TestRbacManageIsNoPathToMorePermissions(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	mgr := roleManager(t, svc)
	if err := svc.AddMember(ctx, mgr, 1, mgr, 0); status(t, err) != http.StatusForbidden {
		t.Errorf("making yourself Administrator: want 403, got %v", err)
	}
	if _, err := svc.CreateRole(ctx, mgr, application.RoleInput{Name: "Mine", Permissions: []string{"users.manage"}}); status(t, err) != http.StatusForbidden {
		t.Errorf("a role with a permission you lack: want 403, got %v", err)
	}
	editor, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Editor", Permissions: []string{"users.manage"}})
	if err := svc.AddMember(ctx, mgr, editor.ID, mgr, 0); status(t, err) != http.StatusForbidden {
		t.Errorf("joining a stronger role: want 403, got %v", err)
	}
	if _, err := svc.UpdateRole(ctx, mgr, editor.ID, application.RoleInput{Name: "Editor", Permissions: []string{"users.read"}}); status(t, err) != http.StatusForbidden {
		t.Errorf("editing a stronger role: want 403, got %v", err)
	}
	_ = svc.AddMember(ctx, admin, 1, 3, 0) // a second administrator
	if err := svc.RemoveMember(ctx, mgr, 1, 3, 0); status(t, err) != http.StatusForbidden {
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
		t.Error("someone with a company-wide role covers a user with no permissions")
	}
}

func TestABranchRoleReachesNoAccount(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	if err := svc.AddMember(ctx, admin, 1, 2, 5); err != nil { // Administrator in branch 5 only
		t.Fatal(err)
	}
	if svc.Covers(in(1, 5), 2, 3) {
		t.Fatal("a branch administrator may act on an account — accounts are app-wide")
	}
}

// nowhere places nobody beyond company 1 itself (an app without an organization module).
type nowhere struct{}

func (nowhere) Resolve(_ context.Context, _ int64, requested string) (kernel.Scope, error) {
	if requested == "1" {
		return kernel.Scope{CompanyID: 1}, nil
	}
	return kernel.Scope{}, kernel.ErrScopeDenied
}

func TestARoleGoesOnlyWhereTheUserIsPlaced(t *testing.T) {
	repo := newMemRepo()
	svc := application.NewService(repo, tx, nowhere{}, people, catalog, noAudit{})
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("bootstrap")
	}
	if err := svc.AddMember(ctx, admin, 1, 2, 5); status(t, err) != http.StatusBadRequest {
		t.Fatalf("a role in a branch the user is not placed in: %v", err)
	}
	if err := svc.AddMember(ctx, admin, 1, 2, 0); err != nil {
		t.Fatalf("in the company they are placed in: %v", err)
	}
}

func TestBootstrapReturnsWhenNoActiveAdministratorIsLeft(t *testing.T) {
	repo := newMemRepo()
	users := directory{1: {ID: 1, Email: "admin@example.com", IsActive: true}}
	svc := application.NewService(repo, tx, everywhere{}, users, catalog, noAudit{})
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

func in(company, branch int64) context.Context {
	return kernel.WithScope(context.Background(), kernel.Scope{CompanyID: company, BranchID: branch})
}

func TestBranchRoleReachesOnlyItsBranch(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	reader, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, admin, reader.ID, 2, 5); err != nil {
		t.Fatal(err)
	}
	if svc.Can(in(1, 0), 2, "users.read") || !svc.Can(in(1, 5), 2, "users.read") || svc.Can(in(1, 6), 2, "users.read") {
		t.Fatal("a branch role must reach its branch only — not the whole company, not another branch")
	}
	if u, _ := svc.Units(in(1, 0), 2, "users.read"); u.AllBranches || len(u.BranchIDs) != 1 || u.BranchIDs[0] != 5 {
		t.Fatalf("units at the company: %+v", u)
	}
	if u, _ := svc.Units(in(1, 0), admin, "users.read"); !u.AllBranches {
		t.Fatalf("a company-wide administrator reaches every branch: %+v", u)
	}
	if !svc.Can(in(2, 0), admin, "users.read") || svc.Can(in(2, 0), 2, "users.read") {
		t.Fatal("the home company's Administrator administers every company; a role in company 1 reaches only company 1")
	}
}

func TestBranchAdministratorIsNoCompanyAdministrator(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	if err := svc.AddMember(ctx, admin, 1, 2, 5); err != nil { // Administrator in branch 5
		t.Fatal(err)
	}
	if svc.Covers(ctx, 2, admin) {
		t.Fatal("a branch administrator may not act on a company-wide administrator")
	}
	if !svc.Covers(ctx, admin, 2) {
		t.Fatal("the company-wide administrator covers a branch administrator")
	}
	if err := svc.AddMember(ctx, 2, 1, 3, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch administrator made someone company-wide Administrator: %v", err)
	}
	if err := svc.AddMember(ctx, 2, 1, 3, 5); err != nil {
		t.Fatalf("within their branch they may: %v", err)
	}
	if err := svc.RemoveMember(ctx, admin, 1, admin, 0); status(t, err) != http.StatusConflict {
		t.Fatalf("the last company-wide Administrator was removed: %v", err)
	}
}

func TestAnAccountIsCoveredInEveryCompany(t *testing.T) {
	svc, repo := setupWithAdmin(t)                       // user 1: Administrator of company 1 (home)
	_ = repo.AddMember(context.Background(), 2, 0, 1, 2) // user 2: Administrator of company 2 only
	_ = repo.AddMember(context.Background(), 3, 0, 1, 3) // user 3: Administrator of company 3 only
	svc = fresh(repo)
	if svc.Covers(in(2, 0), 2, 3) || svc.Covers(in(3, 0), 3, 2) {
		t.Fatal("one company's administrator may act on the account of another's")
	}
	if svc.Covers(in(2, 0), 2, admin) {
		t.Fatal("company 2's administrator may act on the account of the home company's")
	}
	if !svc.Covers(in(1, 0), admin, 2) || !svc.Covers(in(1, 0), admin, 3) {
		t.Fatal("the home company's Administrator administers the installation")
	}
	_ = repo.AddMember(context.Background(), 3, 0, 1, 2)
	if !fresh(repo).Covers(in(2, 0), 2, 3) {
		t.Fatal("an administrator of both companies covers the other's administrator")
	}
}

func TestARoleChangeNeedsEveryCompanyItReaches(t *testing.T) {
	svc, repo := setupWithAdmin(t)
	ctx := context.Background()
	reader, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = repo.AddMember(ctx, 2, 0, 1, 2)         // Ana: Administrator of company 2 only
	_ = repo.AddMember(ctx, 3, 0, reader.ID, 3) // the role has a member in company 3
	svc = fresh(repo)
	inTwo := in(2, 0)
	if _, err := svc.UpdateRole(inTwo, 2, reader.ID, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.manage"}}); status(t, err) != http.StatusForbidden {
		t.Fatalf("a change reaching company 3, by someone with no rights there: %v", err)
	}
	if err := svc.DeleteRole(inTwo, 2, reader.ID); status(t, err) != http.StatusForbidden {
		t.Fatalf("a deletion reaching company 3, by someone with no rights there: %v", err)
	}
	_ = repo.AddMember(ctx, 3, 0, 1, 2)
	if _, err := fresh(repo).UpdateRole(inTwo, 2, reader.ID, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.manage"}}); err != nil {
		t.Fatalf("an administrator of both companies may: %v", err)
	}
}

func TestBranchManagersManageMembersOnlyInTheirBranch(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	ctx := context.Background()
	mgr, err := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pengelola", Permissions: []string{"rbac.manage", "users.read"}})
	if err != nil {
		t.Fatal(err)
	}
	reader, _ := svc.CreateRole(ctx, admin, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	if err := svc.AddMember(ctx, admin, mgr.ID, 2, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRole(in(1, 5), 2, application.RoleInput{Name: "Baru", Permissions: []string{"users.read"}}); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch manager defined a role for the whole app: %v", err)
	}
	if err := svc.AddMember(in(1, 5), 2, reader.ID, 3, 5); err != nil {
		t.Fatalf("in their branch they may: %v", err)
	}
	if err := svc.AddMember(in(1, 5), 2, reader.ID, 3, 6); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch manager gave a role in another branch: %v", err)
	}
	if err := svc.AddMember(in(1, 5), 2, reader.ID, 3, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch manager gave a role across the company: %v", err)
	}
}

func TestTheLastActiveAdministratorStays(t *testing.T) {
	repo := newMemRepo()
	users := directory{
		1: {ID: 1, Email: "admin@example.com", IsActive: true},
		2: {ID: 2, Email: "gone@example.com", IsActive: false},
	}
	svc := application.NewService(repo, tx, everywhere{}, users, catalog, noAudit{})
	ctx := context.Background()
	if ok, _ := svc.Bootstrap(ctx, "admin@example.com"); !ok {
		t.Fatal("bootstrap")
	}
	_ = repo.AddMember(ctx, 1, 0, 1, 2) // a deactivated administrator does not count
	if err := svc.RemoveMember(ctx, admin, 1, admin, 0); status(t, err) != http.StatusConflict {
		t.Fatalf("the last active administrator was removed: %v", err)
	}
	if err := svc.RemoveMember(ctx, admin, 1, 2, 0); err != nil {
		t.Fatalf("removing the deactivated one: %v", err)
	}
}

// placements is a fake organization module: user → companies.
type placements map[int64][]int64

func (p placements) PlacesOf(_ context.Context, user int64) ([]ports.Workplace, error) {
	var out []ports.Workplace
	for _, c := range p[user] {
		out = append(out, ports.Workplace{CompanyID: c})
	}
	return out, nil
}

func (p placements) PlacedUserIDs(_ context.Context, company int64) ([]int64, error) {
	var out []int64
	for u, cs := range p {
		for _, c := range cs {
			if c == company {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

func TestWithPlacementsARolelessAccountBelongsWhereItIsPlaced(t *testing.T) {
	svc, repo := setupWithAdmin(t)                       // user 1: Administrator of company 1
	_ = repo.AddMember(context.Background(), 2, 0, 1, 2) // user 2: Administrator of company 2 only
	svc = fresh(repo)
	svc.UsePlacements(placements{1: {1, 2}, 2: {2}, 3: {1}})
	if svc.Covers(in(2, 0), 2, 3) {
		t.Fatal("company 2's administrator may act on an account placed only in company 1")
	}
	if !svc.Covers(in(1, 0), admin, 3) {
		t.Fatal("company 1's administrator covers its own roleless account")
	}
	if got, _ := svc.SearchUsers(in(2, 0), "example.com"); len(got) != 2 {
		t.Fatalf("company 2's member search lists %d users, want the 2 placed there", len(got))
	}
	svc.UsePlacements(placements{1: {1, 2}, 2: {2}, 3: {1, 2}})
	if svc.Covers(in(2, 0), 2, 3) {
		t.Fatal("an account placed in two companies is covered only with rights in both")
	}
}

func TestTheFounderOfACompanyBecomesItsAdministrator(t *testing.T) {
	svc, _ := setupWithAdmin(t)
	if err := svc.AppointFounder(context.Background(), 7, 3); err != nil {
		t.Fatal(err)
	}
	if err := svc.AppointFounder(context.Background(), 7, 3); err != nil {
		t.Fatalf("repeated: %v", err)
	}
	if !svc.Can(in(7, 0), 3, "users.manage") || svc.Can(in(1, 0), 3, "users.manage") || svc.Can(in(8, 0), 3, "users.manage") {
		t.Fatal("the founder is Administrator of their company — and only there")
	}
}

func TestARoleElsewhereNeverHandsAnAccountToThatCompany(t *testing.T) {
	svc, repo := setupWithAdmin(t)                       // admin: Administrator of company 1
	_ = repo.AddMember(context.Background(), 2, 0, 1, 2) // Ana: Administrator of company 2 only
	viewer, _ := svc.CreateRole(context.Background(), admin, application.RoleInput{Name: "Pembaca", Permissions: []string{"users.read"}})
	_ = repo.AddMember(context.Background(), 2, 0, viewer.ID, 3) // Budi: a role in company 2 …
	svc = fresh(repo)
	svc.UsePlacements(placements{1: {1}, 2: {2}, 3: {1, 2}}) // … and works in both companies
	if svc.Covers(in(2, 0), 2, 3) {
		t.Fatal("a role in company 2 handed company 2 an account that also works in company 1")
	}
	if !svc.Covers(in(1, 0), admin, 3) {
		t.Fatal("the home company's Administrator covers anyone")
	}
}

func TestACompanysFounderStaysWithinTheHomeAdministratorsReach(t *testing.T) {
	svc, repo := setupWithAdmin(t)
	if err := svc.AppointFounder(context.Background(), 7, 2); err != nil { // Ana created company 7
		t.Fatal(err)
	}
	svc = fresh(repo)
	svc.UsePlacements(placements{2: {1, 7}})
	if !svc.Covers(in(1, 0), admin, 2) {
		t.Fatal("creating a company made its founder untouchable by the home company's Administrator")
	}
}

func TestABranchAdministratorReachesAccountsThatLiveInTheBranch(t *testing.T) {
	svc, repo := setupWithAdmin(t)
	_ = repo.AddMember(context.Background(), 1, 5, 1, 2) // Ana: Administrator of branch 5
	svc = fresh(repo)
	svc.UsePlacements(placementsAt{3: {{CompanyID: 1, BranchID: 5}}, 4: {{CompanyID: 1}}})
	if !svc.Covers(in(1, 5), 2, 3) {
		t.Fatal("a branch administrator covers someone who works only in that branch")
	}
	if svc.Covers(in(1, 5), 2, 4) {
		t.Fatal("a branch administrator covers someone working across the company")
	}
}

// placementsAt is a fake organization module with branches: user → workplaces.
type placementsAt map[int64][]ports.Workplace

func (p placementsAt) PlacesOf(_ context.Context, user int64) ([]ports.Workplace, error) {
	return p[user], nil
}

func (p placementsAt) PlacedUserIDs(context.Context, int64) ([]int64, error) { return nil, nil }

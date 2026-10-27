package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/kernel/kerneltest"
	"__GO_MODULE__/internal/modules/organization/application"
	"__GO_MODULE__/internal/modules/organization/domain"
	"__GO_MODULE__/internal/modules/organization/ports"
	"__GO_MODULE__/internal/shared/apperror"
)

// memRepo is the three tables in memory, seeded like migration 001: company 1.
type memRepo struct {
	companies  map[int64]domain.Company
	branches   map[int64]domain.Branch
	placements map[[3]int64]domain.Placement // user, company, branch
	nextID     int64
}

func newMemRepo() *memRepo {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &memRepo{
		companies:  map[int64]domain.Company{1: {ID: 1, Code: "PUSAT", Name: "Perusahaan utama", Timezone: "Asia/Jakarta", IsActive: true, CreatedAt: at, UpdatedAt: at}},
		branches:   map[int64]domain.Branch{},
		placements: map[[3]int64]domain.Placement{},
		nextID:     100,
	}
}

func (m *memRepo) Companies(_ context.Context, activeOnly bool) ([]domain.Company, error) {
	var out []domain.Company
	for _, c := range m.companies {
		if !activeOnly || c.IsActive {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *memRepo) Company(_ context.Context, id int64) (domain.Company, error) {
	c, ok := m.companies[id]
	if !ok {
		return domain.Company{}, application.ErrNotFound
	}
	return c, nil
}

func (m *memRepo) CreateCompany(_ context.Context, c domain.Company) (int64, error) {
	for _, o := range m.companies {
		if o.Code == c.Code {
			return 0, application.ErrCodeTaken
		}
	}
	m.nextID++
	c.ID, c.IsActive = m.nextID, true
	m.companies[c.ID] = c
	return c.ID, nil
}

func (m *memRepo) UpdateCompany(_ context.Context, c domain.Company) error {
	if _, ok := m.companies[c.ID]; !ok {
		return application.ErrNotFound
	}
	m.companies[c.ID] = c
	return nil
}

func (m *memRepo) SetCompanyActive(_ context.Context, id int64, active bool, _ time.Time) error {
	c, ok := m.companies[id]
	if !ok {
		return application.ErrNotFound
	}
	c.IsActive = active
	m.companies[id] = c
	return nil
}

func (m *memRepo) Branches(_ context.Context, company int64) ([]domain.Branch, error) {
	var out []domain.Branch
	for _, b := range m.branches {
		if b.CompanyID == company {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (m *memRepo) Branch(_ context.Context, id int64) (domain.Branch, error) {
	b, ok := m.branches[id]
	if !ok {
		return domain.Branch{}, application.ErrNotFound
	}
	return b, nil
}

func (m *memRepo) CreateBranch(_ context.Context, b domain.Branch) (int64, error) {
	for _, o := range m.branches {
		if o.CompanyID == b.CompanyID && o.Code == b.Code {
			return 0, application.ErrCodeTaken
		}
	}
	m.nextID++
	b.ID = m.nextID
	m.branches[b.ID] = b
	return b.ID, nil
}

func (m *memRepo) UpdateBranch(_ context.Context, b domain.Branch) error {
	if o, ok := m.branches[b.ID]; !ok || o.CompanyID != b.CompanyID {
		return application.ErrNotFound
	}
	m.branches[b.ID] = b
	return nil
}

func (m *memRepo) SetBranchActive(_ context.Context, company, id int64, active bool, _ time.Time) error {
	b, ok := m.branches[id]
	if !ok || b.CompanyID != company {
		return application.ErrNotFound
	}
	b.IsActive = active
	m.branches[id] = b
	return nil
}

func (m *memRepo) PlacementsOfUser(_ context.Context, user int64) ([]application.UnitPlacement, error) {
	var out []application.UnitPlacement
	for k, p := range m.placements {
		if k[0] != user {
			continue
		}
		c := m.companies[p.CompanyID]
		if !c.IsActive {
			continue
		}
		u := application.UnitPlacement{CompanyID: c.ID, BranchID: p.BranchID, IsDefault: p.IsDefault, CompanyCode: c.Code, CompanyName: c.Name, CompanyTimezone: c.Timezone}
		if p.BranchID != 0 {
			b := m.branches[p.BranchID]
			if !b.IsActive || b.CompanyID != c.ID {
				continue
			}
			u.BranchCode, u.BranchName, u.BranchTimezone = b.Code, b.Name, b.Timezone
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CompanyID < out[j].CompanyID || (out[i].CompanyID == out[j].CompanyID && out[i].BranchID < out[j].BranchID)
	})
	return out, nil
}

func (m *memRepo) PlacesOfUser(_ context.Context, user int64) ([]domain.Placement, error) {
	var out []domain.Placement
	for k, p := range m.placements {
		if k[0] == user {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *memRepo) Placements(_ context.Context, company int64) ([]domain.Placement, error) {
	var out []domain.Placement
	for _, p := range m.placements {
		if p.CompanyID == company {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out, nil
}

func (m *memRepo) AddPlacement(_ context.Context, p domain.Placement) (bool, error) {
	k := [3]int64{p.UserID, p.CompanyID, p.BranchID}
	if _, ok := m.placements[k]; ok {
		return false, nil
	}
	m.placements[k] = p
	return true, nil
}

func (m *memRepo) RemovePlacement(_ context.Context, user, company, branch int64) (bool, error) {
	k := [3]int64{user, company, branch}
	_, ok := m.placements[k]
	delete(m.placements, k)
	return ok, nil
}

func (m *memRepo) SetDefault(_ context.Context, user, company, branch int64) (bool, error) {
	k := [3]int64{user, company, branch}
	if _, ok := m.placements[k]; !ok {
		return false, nil
	}
	for key, p := range m.placements {
		if key[0] == user {
			p.IsDefault = key == k
			m.placements[key] = p
		}
	}
	return true, nil
}

func (m *memRepo) PlacedUserIDs(_ context.Context, company int64) ([]int64, error) {
	seen := map[int64]bool{}
	var out []int64
	for k := range m.placements {
		if k[1] == company && !seen[k[0]] {
			seen[k[0]] = true
			out = append(out, k[0])
		}
	}
	return out, nil
}

func (m *memRepo) IsPlaced(_ context.Context, user, company int64) (bool, error) {
	for k := range m.placements {
		if k[0] == user && k[1] == company {
			return true, nil
		}
	}
	return false, nil
}

func (m *memRepo) CompaniesOf(_ context.Context, user int64) ([]int64, error) {
	seen := map[int64]bool{}
	var out []int64
	for k := range m.placements {
		if k[0] == user && !seen[k[1]] {
			seen[k[1]] = true
			out = append(out, k[1])
		}
	}
	return out, nil
}

func (m *memRepo) CountPlacementsOfUser(_ context.Context, user int64) (int64, error) {
	n := int64(0)
	for k := range m.placements {
		if k[0] == user {
			n++
		}
	}
	return n, nil
}

func (m *memRepo) CountPlacements(context.Context) (int64, error) {
	return int64(len(m.placements)), nil
}

func (m *memRepo) UsersWithAWorkplace(ctx context.Context) ([]int64, error) {
	seen := map[int64]bool{}
	var out []int64
	for k := range m.placements {
		if seen[k[0]] {
			continue
		}
		if units, _ := m.PlacementsOfUser(ctx, k[0]); len(units) > 0 {
			seen[k[0]] = true
			out = append(out, k[0])
		}
	}
	return out, nil
}

// directory is a fake UserDirectory.
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
		if strings.Contains(u.Name, q) || strings.Contains(u.Email, q) {
			out = append(out, u)
		}
	}
	return out, nil
}

func (d directory) SearchAmong(ctx context.Context, ids []int64, q string, limit int) ([]ports.User, error) {
	among := map[int64]bool{}
	for _, id := range ids {
		among[id] = true
	}
	found, _ := d.Search(ctx, q, limit)
	var out []ports.User
	for _, u := range found {
		if among[u.ID] {
			out = append(out, u)
		}
	}
	return out, nil
}

func (d directory) IDs(context.Context) ([]int64, error) {
	var out []int64
	for id := range d {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// authz grants per (company, branch): what rbac would answer. foreign are accounts the actor
// does not cover (rbac: they hold something the actor lacks).
type authz struct {
	m       map[kernel.Scope]map[string]bool
	foreign map[int64]bool
}

func (a authz) Can(ctx context.Context, perm string) bool {
	sc := kernel.ScopeFrom(ctx)
	return a.m[kernel.Scope{CompanyID: sc.CompanyID, BranchID: sc.BranchID}][perm]
}

func (a authz) Units(ctx context.Context, perm string) (kernel.Units, error) {
	sc := kernel.ScopeFrom(ctx)
	if a.m[kernel.Scope{CompanyID: sc.CompanyID}][perm] && sc.BranchID == 0 {
		return kernel.Units{AllBranches: true}, nil
	}
	var u kernel.Units
	for s, perms := range a.m {
		if s.CompanyID == sc.CompanyID && s.BranchID != 0 && perms[perm] && (sc.BranchID == 0 || sc.BranchID == s.BranchID) {
			u.BranchIDs = append(u.BranchIDs, s.BranchID)
		}
	}
	return u, nil
}

func (a authz) Covers(_ context.Context, target int64) bool { return !a.foreign[target] }

// signedIn is whoever the test acts as.
type signedIn struct{ id int64 }

func (s *signedIn) UserID(context.Context) (int64, bool) { return s.id, s.id != 0 }

const admin, ana, budi, citra = int64(1), int64(2), int64(3), int64(4)

type world struct {
	svc   *application.Service
	repo  *memRepo
	kit   *kerneltest.Kit
	authz authz
	who   *signedIn
}

func setup(t *testing.T) world {
	t.Helper()
	k := kerneltest.Deps(t)
	repo := newMemRepo()
	users := directory{
		admin: {ID: admin, Name: "Admin", Email: "admin@a.test", IsActive: true},
		ana:   {ID: ana, Name: "Ana", Email: "ana@a.test", IsActive: true},
		budi:  {ID: budi, Name: "Budi", Email: "budi@b.test", IsActive: true},
		citra: {ID: citra, Name: "Citra", Email: "citra@b.test", IsActive: true},
	}
	az := authz{m: map[kernel.Scope]map[string]bool{{CompanyID: 1}: {application.PermCompanies: true, application.PermManage: true, application.PermRead: true}}, foreign: map[int64]bool{}}
	who := &signedIn{id: admin}
	svc := application.NewService(repo, k.Deps.Tx, users, k.Deps.Events, k.Deps.Audit, az, who, k.Clock)
	if n, err := svc.Bootstrap(context.Background(), "Toko Maju"); err != nil || n != 4 {
		t.Fatalf("bootstrap placed %d: %v", n, err)
	}
	return world{svc: svc, repo: repo, kit: k, authz: az, who: who}
}

func in(company, branch int64) context.Context {
	return kernel.WithScope(context.Background(), kernel.Scope{CompanyID: company, BranchID: branch})
}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func TestBootstrapNamesTheHomeCompanyAndPlacesEveryoneOnce(t *testing.T) {
	w := setup(t)
	home, _ := w.repo.Company(context.Background(), 1)
	if home.Name != "Toko Maju" || home.Timezone != w.kit.Clock.Location().String() {
		t.Fatalf("home company = %+v", home)
	}
	if n, _ := w.svc.Bootstrap(context.Background(), "Other"); n != 0 {
		t.Fatalf("a second bootstrap placed %d", n)
	}
	if home2, _ := w.repo.Company(context.Background(), 1); home2.Name != "Toko Maju" {
		t.Fatalf("a second bootstrap renamed the home company: %q", home2.Name)
	}
	sc, err := w.svc.Resolve(context.Background(), ana, "")
	if err != nil || sc.CompanyID != 1 || sc.BranchID != 0 {
		t.Fatalf("after install everyone works in company 1 as before: %+v %v", sc, err)
	}
}

func TestCreatingACompanyPlacesItsCreatorAndAnnouncesIt(t *testing.T) {
	w := setup(t)
	c, err := w.svc.CreateCompany(in(1, 0), admin, application.CompanyInput{Code: " pt-b ", Name: "PT B", Timezone: "Asia/Makassar"})
	if err != nil || c.Code != "PT-B" {
		t.Fatalf("create: %+v %v", c, err)
	}
	if sc, err := w.svc.Resolve(context.Background(), admin, itoa(c.ID)); err != nil || sc.CompanyID != c.ID || sc.Location.String() != "Asia/Makassar" {
		t.Fatalf("the creator works in the new company, in its zone: %+v %v", sc, err)
	}
	pub := w.kit.Published()
	if len(pub) != 1 || pub[0].Name != "organization.company.created" {
		t.Fatalf("published = %v", pub)
	}
	var p struct {
		CompanyID int64 `json:"company_id"`
		CreatedBy int64 `json:"created_by"`
	}
	_ = json.Unmarshal(pub[0].Payload, &p)
	if p.CompanyID != c.ID || p.CreatedBy != admin {
		t.Fatalf("payload = %+v", p)
	}
	if _, err := w.svc.CreateCompany(in(1, 0), admin, application.CompanyInput{Code: "PT-B", Name: "Again", Timezone: "Asia/Jakarta"}); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("a code in use: %v", err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestCompaniesAreManagedOnlyFromTheHomeCompanyAsAWhole(t *testing.T) {
	w := setup(t)
	c, _ := w.svc.CreateCompany(in(1, 0), admin, application.CompanyInput{Code: "B", Name: "PT B", Timezone: "Asia/Jakarta"})
	w.authz.m[kernel.Scope{CompanyID: c.ID}] = map[string]bool{application.PermCompanies: true}
	if _, err := w.svc.CreateCompany(in(c.ID, 0), admin, application.CompanyInput{Code: "C", Name: "PT C", Timezone: "Asia/Jakarta"}); status(t, err) != http.StatusForbidden {
		t.Fatalf("a company made from inside another company: %v", err)
	}
	if err := w.svc.SetCompanyActive(in(1, 0), 1, false); status(t, err) != http.StatusConflict {
		t.Fatalf("the home company was deactivated: %v", err)
	}
	if err := w.svc.SetCompanyActive(in(1, 0), c.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Resolve(context.Background(), admin, itoa(c.ID)); !errors.Is(err, kernel.ErrScopeDenied) {
		t.Fatalf("a deactivated company is entered: %v", err)
	}
	if scopes, _ := w.svc.ActiveCompanies(context.Background()); len(scopes) != 1 || scopes[0].CompanyID != 1 {
		t.Fatalf("background work runs for a deactivated company: %+v", scopes)
	}
}

func TestResolveAnswersOnlyWhereTheUserIsPlaced(t *testing.T) {
	w := setup(t)
	ctx := in(1, 0)
	bdg, err := w.svc.CreateBranch(ctx, application.BranchInput{Code: "BDG", Name: "Bandung"})
	if err != nil {
		t.Fatal(err)
	}
	mks, _ := w.svc.CreateBranch(ctx, application.BranchInput{Code: "MKS", Name: "Makassar", Timezone: "Asia/Makassar", NITKU: "0123456789012345000001"})
	other, _ := w.svc.CreateCompany(ctx, admin, application.CompanyInput{Code: "B", Name: "PT B", Timezone: "Asia/Jakarta"})
	_, _ = w.repo.RemovePlacement(context.Background(), budi, 1, 0)
	if err := w.svc.Place(ctx, admin, budi, mks.ID); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		user          int64
		requested     string
		company, unit int64
		zone          string
		denied        bool
	}{
		{user: ana, requested: "", company: 1},
		{user: ana, requested: "1", company: 1},
		{user: ana, requested: "1:" + itoa(bdg.ID), company: 1, unit: bdg.ID, zone: "Asia/Jakarta"}, // across the company: any branch
		{user: ana, requested: "1:" + itoa(mks.ID), company: 1, unit: mks.ID, zone: "Asia/Makassar"},
		{user: budi, requested: "", company: 1, unit: mks.ID, zone: "Asia/Makassar"},
		{user: budi, requested: "1:" + itoa(mks.ID), company: 1, unit: mks.ID},
		{user: budi, requested: "1", denied: true},                 // placed in one branch, not across the company
		{user: budi, requested: "1:" + itoa(bdg.ID), denied: true}, // another branch
		{user: ana, requested: itoa(other.ID), denied: true},       // another company
		{user: ana, requested: itoa(other.ID) + ":" + itoa(mks.ID), denied: true},
		{user: admin, requested: itoa(other.ID) + ":" + itoa(mks.ID), denied: true}, // a branch of another company
		{user: ana, requested: "x", denied: true},
		{user: ana, requested: "1:0", denied: true},
	} {
		sc, err := w.svc.Resolve(context.Background(), c.user, c.requested)
		if c.denied {
			if !errors.Is(err, kernel.ErrScopeDenied) {
				t.Errorf("user %d asking %q: want denied, got %+v %v", c.user, c.requested, sc, err)
			}
			continue
		}
		if err != nil || sc.CompanyID != c.company || sc.BranchID != c.unit || (c.zone != "" && sc.Location.String() != c.zone) {
			t.Errorf("user %d asking %q: got %+v %v", c.user, c.requested, sc, err)
		}
	}
	if _, err := w.svc.Resolve(context.Background(), 99, ""); status(t, err) != http.StatusForbidden || errors.Is(err, kernel.ErrScopeDenied) {
		t.Errorf("placed nowhere: want a plain 403, got %v", err)
	}
	if err := w.svc.SetBranchActive(ctx, mks.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Resolve(context.Background(), budi, ""); status(t, err) != http.StatusForbidden {
		t.Fatalf("a user placed only in a deactivated branch still works: %v", err)
	}
}

func TestPlacingNeedsTheRightReach(t *testing.T) {
	w := setup(t)
	ctx := in(1, 0)
	bdg, _ := w.svc.CreateBranch(ctx, application.BranchInput{Code: "BDG", Name: "Bandung"})
	smg, _ := w.svc.CreateBranch(ctx, application.BranchInput{Code: "SMG", Name: "Semarang"})
	if err := w.svc.Unplace(ctx, admin, admin, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("changing one's own placement: %v", err)
	}
	// Ana manages Bandung only.
	w.authz.m[kernel.Scope{CompanyID: 1, BranchID: bdg.ID}] = map[string]bool{application.PermManage: true}
	asAna := in(1, bdg.ID)
	if err := w.svc.Place(asAna, ana, citra, bdg.ID); err != nil {
		t.Fatalf("in her branch: %v", err)
	}
	if err := w.svc.Place(asAna, ana, citra, smg.ID); status(t, err) != http.StatusForbidden {
		t.Fatalf("in another branch: %v", err)
	}
	if err := w.svc.Unplace(asAna, ana, budi, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("across the company: %v", err)
	}
	if _, err := w.svc.CreateBranch(asAna, application.BranchInput{Code: "X", Name: "X"}); status(t, err) != http.StatusForbidden {
		t.Fatalf("a branch manager made a branch: %v", err)
	}
	if err := w.svc.Place(ctx, admin, citra, bdg.ID); status(t, err) != http.StatusConflict {
		t.Fatalf("placed twice: %v", err)
	}
	if _, err := w.svc.CreateBranch(ctx, application.BranchInput{Code: "Y", Name: "Y", NITKU: "123"}); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("a short NITKU: %v", err)
	}
}

func TestANewUserIsPlacedWhereTheyWereCreated(t *testing.T) {
	w := setup(t)
	bdg, _ := w.svc.CreateBranch(in(1, 0), application.BranchInput{Code: "BDG", Name: "Bandung"})
	if err := w.svc.PlaceNew(in(1, bdg.ID), 50); err != nil {
		t.Fatal(err)
	}
	if sc, err := w.svc.Resolve(context.Background(), 50, ""); err != nil || sc.BranchID != bdg.ID {
		t.Fatalf("created in Bandung, works in Bandung: %+v %v", sc, err)
	}
	if err := w.svc.PlaceNew(context.Background(), 51); err == nil {
		t.Fatal("without a scope there is no place to choose — an error, never a default")
	}
}

func TestCandidatesNeverListAnotherCompanysPeopleByPartialSearch(t *testing.T) {
	w := setup(t)
	other, _ := w.svc.CreateCompany(in(1, 0), admin, application.CompanyInput{Code: "B", Name: "PT B", Timezone: "Asia/Jakarta"})
	w.authz.m[kernel.Scope{CompanyID: other.ID}] = map[string]bool{application.PermManage: true}
	inB := in(other.ID, 0)
	got, _ := w.svc.Candidates(inB, "a")
	for _, u := range got {
		if u.ID != admin {
			t.Fatalf("company B's search shows %s, placed only in company 1", u.Name)
		}
	}
	got, _ = w.svc.Candidates(inB, "ana@a.test")
	if len(got) != 1 || got[0].ID != ana {
		t.Fatalf("an exact email finds an account the actor may place: %v", got)
	}
	w.authz.foreign[ana] = true
	if got, _ = w.svc.Candidates(inB, "ana@a.test"); len(got) != 0 {
		t.Fatalf("an exact email disclosed an account the actor may not place: %v", got)
	}
}

func TestMineListsBranchesOfCompaniesPlacedAcross(t *testing.T) {
	w := setup(t)
	bdg, _ := w.svc.CreateBranch(in(1, 0), application.BranchInput{Code: "BDG", Name: "Bandung"})
	units, def, err := w.svc.Mine(context.Background(), ana)
	if err != nil || len(units) != 2 || units[1].BranchID != bdg.ID || def.CompanyID != 1 || def.BranchID != 0 {
		t.Fatalf("units=%+v default=%+v err=%v", units, def, err)
	}
	if err := w.svc.SetDefault(context.Background(), ana, 1, bdg.ID); status(t, err) != http.StatusNotFound {
		t.Fatalf("a default must be one of the user's own placements: %v", err)
	}
}

func TestPlacingSomeoneNeedsAuthorityOverTheirAccount(t *testing.T) {
	w := setup(t)
	other, _ := w.svc.CreateCompany(in(1, 0), admin, application.CompanyInput{Code: "B", Name: "PT B", Timezone: "Asia/Jakarta"})
	w.authz.m[kernel.Scope{CompanyID: other.ID}] = map[string]bool{application.PermManage: true, application.PermRead: true}
	// As company B's administrator alone: rbac covers neither Ana (company 1's) nor an account
	// that does not exist (placed nowhere: the home company's to place).
	w.authz.foreign[ana], w.authz.foreign[999] = true, true
	if err := w.svc.Place(in(other.ID, 0), admin, ana, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("claiming another company's account by placing it: %v", err)
	}
	if err := w.svc.Place(in(other.ID, 0), admin, 999, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("an unknown account must answer like one not yours (no enumeration): %v", err)
	}
	if err := w.svc.Unplace(in(1, 0), admin, ana, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("unplacing an account one does not cover: %v", err)
	}
}

func TestReadingPlacementsFromOneBranchShowsThatBranch(t *testing.T) {
	w := setup(t)
	bdg, _ := w.svc.CreateBranch(in(1, 0), application.BranchInput{Code: "BDG", Name: "Bandung"})
	_ = w.svc.Place(in(1, 0), admin, citra, bdg.ID)
	people, err := w.svc.People(in(1, 0))
	if err != nil || len(people) != 5 {
		t.Fatalf("company-wide: %d people, %v", len(people), err)
	}
	w.authz.m[kernel.Scope{CompanyID: 1, BranchID: bdg.ID}] = map[string]bool{application.PermRead: true}
	delete(w.authz.m, kernel.Scope{CompanyID: 1})
	people, _ = w.svc.People(in(1, bdg.ID))
	if len(people) != 1 || people[0].ID != citra {
		t.Fatalf("from Bandung: %+v", people)
	}
}

func TestReadingACompanyIsNoAuthorityOverItsAccounts(t *testing.T) {
	w := setup(t)
	other, _ := w.svc.CreateCompany(in(1, 0), admin, application.CompanyInput{Code: "B", Name: "PT B", Timezone: "Asia/Jakarta"})
	w.authz.m[kernel.Scope{CompanyID: other.ID}] = map[string]bool{application.PermManage: true}
	// The actor manages company B, and only reads company 1 (rbac would still cover Ana there).
	w.authz.m[kernel.Scope{CompanyID: 1}] = map[string]bool{application.PermRead: true}
	if err := w.svc.Place(in(other.ID, 0), admin, ana, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("placing an account whose company the actor only reads: %v", err)
	}
}

func TestDeactivatingACompanyHandsItsPeopleToNoOne(t *testing.T) {
	w := setup(t)
	other, _ := w.svc.CreateCompany(in(1, 0), admin, application.CompanyInput{Code: "B", Name: "PT B", Timezone: "Asia/Jakarta"})
	_, _ = w.repo.RemovePlacement(context.Background(), budi, 1, 0)
	_, _ = w.repo.AddPlacement(context.Background(), domain.Placement{UserID: budi, CompanyID: other.ID, IsDefault: true})
	_ = w.svc.SetCompanyActive(in(1, 0), other.ID, false)
	// The actor manages company 1 (the fake grants nothing in B).
	if err := w.svc.Place(in(1, 0), admin, budi, 0); status(t, err) != http.StatusForbidden {
		t.Fatalf("company 1 claimed a deactivated company's employee: %v", err)
	}
	// Recovery from the home company needs what placing needs: organization.manage in that
	// company and authority over the account — a holder of organization.companies alone has none.
	if err := w.svc.PlaceInCompany(in(1, 0), admin, other.ID, "citra@b.test"); status(t, err) != http.StatusForbidden {
		t.Fatalf("placing across a company the actor does not manage: %v", err)
	}
	w.authz.m[kernel.Scope{CompanyID: other.ID}] = map[string]bool{application.PermManage: true} // as the home Administrator
	w.authz.foreign[citra] = true
	if err := w.svc.PlaceInCompany(in(1, 0), admin, other.ID, "citra@b.test"); status(t, err) != http.StatusForbidden {
		t.Fatalf("placing an account the actor has no authority over: %v", err)
	}
	if err := w.svc.PlaceInCompany(in(1, 0), admin, other.ID, "nobody@x.test"); status(t, err) != http.StatusForbidden {
		t.Fatalf("an unknown email must answer like one not yours: %v", err)
	}
	delete(w.authz.foreign, citra)
	if err := w.svc.PlaceInCompany(in(1, 0), admin, other.ID, "citra@b.test"); err != nil {
		t.Fatalf("the recovery path: %v", err)
	}
	if err := w.svc.PlaceInCompany(in(other.ID, 0), admin, other.ID, "citra@b.test"); status(t, err) != http.StatusForbidden {
		t.Fatalf("placing across a company from inside another: %v", err)
	}
}

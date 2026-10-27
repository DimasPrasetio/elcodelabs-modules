package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/organization/domain"
	"__GO_MODULE__/internal/modules/organization/ports"
	"__GO_MODULE__/internal/shared/apperror"
)

// Person is a placement with who it is.
type Person struct {
	ports.User
	BranchID  int64
	IsDefault bool
	Missing   bool // the directory no longer knows them; the placement can still be removed
}

// People lists who is placed in the company the request works in, and where — all of it for
// someone reading it company-wide; otherwise only the branches organization.read reaches.
func (s *Service) People(ctx context.Context) ([]Person, error) {
	placed, err := s.repo.Placements(ctx, here(ctx).CompanyID)
	if err != nil {
		return nil, err
	}
	reach, err := s.authz.Units(ctx, PermRead)
	if err != nil {
		return nil, err
	}
	out := make([]Person, 0, len(placed))
	for _, p := range placed {
		if !reach.AllBranches && (p.BranchID == 0 || !reach.Includes(p.BranchID)) {
			continue
		}
		u, err := s.users.Get(ctx, p.UserID)
		switch {
		case errors.Is(err, ports.ErrUserNotFound):
			out = append(out, Person{User: ports.User{ID: p.UserID}, BranchID: p.BranchID, IsDefault: p.IsDefault, Missing: true})
		case err != nil:
			return nil, err
		default:
			out = append(out, Person{User: u, BranchID: p.BranchID, IsDefault: p.IsDefault})
		}
	}
	return out, nil
}

// mayPlace: placing (or unplacing) someone across the company needs organization.manage across
// it; in a branch, organization.manage reaching that branch. And — since a placement is where an
// account belongs — authority over the account itself (mayManageAccount): otherwise placing
// someone here would be a way to claim another company's account, shield it from its own
// administrators, or list everyone's name and email; and unplacing, a way to lock administrators
// out. Nobody changes their own placements.
func (s *Service) mayPlace(ctx context.Context, actor, userID, branch int64) error {
	if actor == userID {
		return apperror.Forbidden("Anda tidak dapat mengubah penempatan Anda sendiri")
	}
	if branch == 0 {
		if err := s.mayManageWholeCompany(ctx); err != nil {
			return err
		}
	} else {
		u, err := s.authz.Units(ctx, PermManage)
		if err != nil {
			return err
		}
		if !u.Includes(branch) {
			return apperror.Forbidden("Anda tidak dapat mengelola penempatan di cabang ini")
		}
	}
	ok, err := s.mayManageAccount(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return errNotYours
	}
	return nil
}

// mayManageAccount: the account belongs where it is placed — in active units or not, so
// deactivating a branch or a company hands its people to no one; placed nowhere, to the home
// company as a whole. Placing it needs organization.manage in every one of those places — read
// access elsewhere is no authority — and rbac's word that the actor holds, there, everything the
// account holds (d.Authz.Covers). An account that does not exist answers like one that is not
// the actor's, to anyone without the home company's authority.
func (s *Service) mayManageAccount(ctx context.Context, userID int64) (bool, error) {
	places, err := s.repo.PlacesOfUser(ctx, userID)
	if err != nil {
		return false, err
	}
	if len(places) == 0 {
		places = []domain.Placement{{CompanyID: domain.HomeCompany}}
	}
	for _, p := range places {
		if !s.authz.Can(kernel.WithScope(ctx, kernel.Scope{CompanyID: p.CompanyID, BranchID: p.BranchID}), PermManage) {
			return false, nil
		}
	}
	return s.authz.Covers(ctx, userID), nil
}

var errNotYours = apperror.Forbidden("Anda tidak dapat menempatkan atau mencabut pengguna ini — akunnya di luar wewenang Anda")

func where(companyName string, b domain.Branch) string {
	if b.ID == 0 {
		return "seluruh " + companyName
	}
	return companyName + " › " + b.Name
}

// Place puts a user in the company the request works in: across it (branch 0) or in one of its
// active branches. A user's first placement becomes their default.
func (s *Service) Place(ctx context.Context, actor, userID, branch int64) error {
	if branch < 0 {
		return apperror.BadRequest("Cabang tidak valid")
	}
	if err := s.mayPlace(ctx, actor, userID, branch); err != nil {
		return err
	}
	company := here(ctx).CompanyID
	var b domain.Branch
	if branch != 0 {
		var err error
		if b, err = s.branchHere(ctx, branch); err != nil {
			return err
		}
		if !b.IsActive {
			return apperror.Conflict("Cabang ini nonaktif")
		}
	}
	u, err := s.users.Get(ctx, userID)
	if errors.Is(err, ports.ErrUserNotFound) {
		return apperror.NotFound("Pengguna tidak ditemukan")
	}
	if err != nil {
		return err
	}
	c, err := s.repo.Company(ctx, company)
	if err != nil {
		return err
	}
	added := false
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		added, err = s.place(ctx, actor, userID, company, branch)
		return err
	})
	if err != nil {
		return err
	}
	if !added {
		return apperror.Conflict("Pengguna sudah ditempatkan di sana")
	}
	s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "organization", Action: "user.placed", Target: userTarget(userID),
		Summary: fmt.Sprintf("Menempatkan %s di %s", u.Name, where(c.Name, b)), Data: map[string]any{"company_id": company, "branch_id": branch}})
	return nil
}

// place adds the placement — as the user's default when it is their first.
func (s *Service) place(ctx context.Context, actor, userID, company, branch int64) (bool, error) {
	n, err := s.repo.CountPlacementsOfUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return s.repo.AddPlacement(ctx, domain.Placement{UserID: userID, CompanyID: company, BranchID: branch, IsDefault: n == 0, CreatedAt: s.clock.Now(), CreatedBy: actor})
}

// Unplace takes a placement away. The user keeps their account and roles; they just can no
// longer work there.
func (s *Service) Unplace(ctx context.Context, actor, userID, branch int64) error {
	if err := s.mayPlace(ctx, actor, userID, branch); err != nil {
		return err
	}
	company := here(ctx).CompanyID
	ok, err := s.repo.RemovePlacement(ctx, userID, company, branch)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.NotFound("Penempatan tidak ditemukan")
	}
	s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "organization", Action: "user.unplaced", Target: userTarget(userID),
		Summary: "Mencabut penempatan", Data: map[string]any{"company_id": company, "branch_id": branch}})
	return nil
}

// PlaceInCompany places someone — found by exact email — across another company, from the home
// company: how a company whose administrators can no longer get in (unplaced, deactivated) gets
// one back, or a new company its first people. It needs what placing needs anywhere —
// organization.manage in that company and authority over the account (mayManageAccount) — so in
// practice it is the home company's Administrators' (who administer the installation) or that
// company's own founder's. Audited in both companies.
func (s *Service) PlaceInCompany(ctx context.Context, actor, companyID int64, email string) error {
	if err := s.mayManageCompanies(ctx); err != nil {
		return err
	}
	c, err := s.repo.Company(ctx, companyID)
	if err != nil {
		return notFound(err, "Perusahaan")
	}
	if !s.authz.Can(kernel.WithScope(ctx, kernel.Scope{CompanyID: companyID}), PermManage) {
		return apperror.Forbidden("Anda tidak mengelola organisasi perusahaan itu")
	}
	u, err := s.users.FindByEmail(ctx, strings.TrimSpace(email))
	if errors.Is(err, ports.ErrUserNotFound) {
		return errNotYours // whether an email exists is not told
	}
	if err != nil {
		return err
	}
	if ok, err := s.mayManageAccount(ctx, u.ID); err != nil {
		return err
	} else if !ok {
		return errNotYours
	}
	added := false
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		added, err = s.place(ctx, actor, u.ID, companyID, 0)
		return err
	})
	if err != nil {
		return err
	}
	if !added {
		return apperror.Conflict("Pengguna sudah ditempatkan di seluruh perusahaan itu")
	}
	entry := kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "organization", Action: "user.placed",
		Target: userTarget(u.ID), Summary: fmt.Sprintf("Menempatkan %s di seluruh %s (dari perusahaan induk)", u.Name, c.Name), Data: map[string]any{"company_id": companyID, "branch_id": 0}}
	s.audit.Record(ctx, entry) // the home company, where it was done
	if companyID != domain.HomeCompany {
		s.audit.Record(kernel.WithScope(ctx, kernel.Scope{CompanyID: companyID}), entry) // and where it takes effect
	}
	return nil
}

// PlaceNew places a user just created in the scope ctx works in — the users module calls it
// (through its Placements port) inside the transaction that creates the account. Without a scope
// there is no place to choose (the app has several companies): an error, never a default.
func (s *Service) PlaceNew(ctx context.Context, userID int64) error {
	if !kernel.HasScope(ctx) {
		return errors.New("organization: a new user is placed where they were created — no company in ctx")
	}
	sc := kernel.ScopeFrom(ctx)
	company, branch := sc.CompanyID, sc.BranchID
	actor, ok := s.who.UserID(ctx)
	if !ok {
		actor = kernel.ActorFrom(ctx)
	}
	_, err := s.place(ctx, actor, userID, company, branch)
	return err
}

// Candidates finds users to place in the company the request works in: those already placed
// here whose name or email contains q, and — matched by exact email only — anyone else. Other
// companies' people are never listed by a partial search.
func (s *Service) Candidates(ctx context.Context, q string) ([]ports.User, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []ports.User{}, nil
	}
	// People already placed where the actor's organization.manage reaches.
	reach, err := s.authz.Units(ctx, PermManage)
	if err != nil {
		return nil, err
	}
	placements, err := s.repo.Placements(ctx, here(ctx).CompanyID)
	if err != nil {
		return nil, err
	}
	var placed []int64
	for _, p := range placements {
		if reach.AllBranches || (p.BranchID != 0 && reach.Includes(p.BranchID)) {
			placed = append(placed, p.UserID)
		}
	}
	out := []ports.User{}
	seen := map[int64]bool{}
	if len(placed) > 0 {
		found, err := s.users.SearchAmong(ctx, placed, q, 10)
		if err != nil {
			return nil, err
		}
		for _, u := range found {
			out = append(out, u)
			seen[u.ID] = true
		}
	}
	// Someone else, by exact email — only an account the actor may place, so the search tells
	// nothing about anyone else's.
	if strings.Contains(q, "@") {
		u, err := s.users.FindByEmail(ctx, q)
		if err != nil && !errors.Is(err, ports.ErrUserNotFound) {
			return nil, err
		}
		if err == nil && !seen[u.ID] {
			ok, err := s.mayManageAccount(ctx, u.ID)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

// Unplaced lists users who can work nowhere — placed nowhere, or only in deactivated companies
// or branches. For whoever manages companies, to place them.
func (s *Service) Unplaced(ctx context.Context) ([]ports.User, error) {
	if err := s.mayManageCompanies(ctx); err != nil {
		return nil, err
	}
	all, err := s.users.IDs(ctx)
	if err != nil {
		return nil, err
	}
	placed, err := s.repo.UsersWithAWorkplace(ctx)
	if err != nil {
		return nil, err
	}
	in := map[int64]bool{}
	for _, id := range placed {
		in[id] = true
	}
	out := []ports.User{}
	for _, id := range all {
		if in[id] {
			continue
		}
		u, err := s.users.Get(ctx, id)
		if errors.Is(err, ports.ErrUserNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// homePlaceholder is the home company's name as migration 001 creates it.
const homePlaceholder = "Perusahaan utama"

// Bootstrap prepares a fresh install: names the home company after the app (APP_NAME,
// APP_TIMEZONE) while nobody has changed it, and places every user across it while nobody is
// placed anywhere — so installing the module changes nothing for anyone. It reports how many
// users it placed.
func (s *Service) Bootstrap(ctx context.Context, appName string) (int, error) {
	placed := 0
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		placed = 0
		home, err := s.repo.Company(ctx, domain.HomeCompany)
		if err != nil {
			return err
		}
		// Still as the migration made it (never edited: updated_at is left at created_at here).
		if home.UpdatedAt.Equal(home.CreatedAt) && home.Name == homePlaceholder {
			name, tz := strings.TrimSpace(appName), s.clock.Location().String()
			if name == "" || len(name) > domain.MaxNameLength {
				name = homePlaceholder
			}
			if name != home.Name || tz != home.Timezone {
				home.Name, home.Timezone = name, tz
				if err := s.repo.UpdateCompany(ctx, home); err != nil {
					return err
				}
			}
		}
		n, err := s.repo.CountPlacements(ctx)
		if err != nil || n > 0 {
			return err
		}
		ids, err := s.users.IDs(ctx)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := s.repo.AddPlacement(ctx, domain.Placement{UserID: id, CompanyID: domain.HomeCompany, IsDefault: true, CreatedAt: s.clock.Now()}); err != nil {
				return err
			}
			placed++
		}
		return nil
	})
	return placed, err
}

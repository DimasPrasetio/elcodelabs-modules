package application_test

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/kernel/kerneltest"
	"__GO_MODULE__/internal/modules/customers/application"
	"__GO_MODULE__/internal/modules/customers/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/taxid"
	"__GO_MODULE__/internal/shared/validator"
)

// memRepo keeps every company's rows in memory, keyed like the tables.
type memRepo struct {
	customers map[int64]domain.Customer
	addresses map[int64]domain.Address
	contacts  map[int64]domain.Contact
	settings  map[int64]application.Settings
	nextID    int64
}

func newMemRepo() *memRepo {
	return &memRepo{customers: map[int64]domain.Customer{}, addresses: map[int64]domain.Address{}, contacts: map[int64]domain.Contact{}, settings: map[int64]application.Settings{}}
}

func (m *memRepo) id() int64 { m.nextID++; return m.nextID }

func (m *memRepo) List(_ context.Context, co int64, f application.ListFilter) ([]domain.Customer, int64, error) {
	var out []domain.Customer
	for _, c := range m.customers {
		if c.CompanyID != co {
			continue
		}
		switch f.Status {
		case "active":
			if !c.IsActive {
				continue
			}
		case "on_hold":
			if !c.IsActive || !c.OnHold {
				continue
			}
		case "inactive":
			if c.IsActive {
				continue
			}
		}
		if f.Search != "" && !strings.Contains(c.Name, f.Search) && !strings.Contains(c.Code, f.Search) && !strings.HasPrefix(c.TaxID, f.Search) {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, int64(len(out)), nil
}

func (m *memRepo) Search(ctx context.Context, co int64, q string, limit int) ([]domain.Customer, error) {
	rows, _, err := m.List(ctx, co, application.ListFilter{Search: q, Status: "active"})
	return rows[:min(limit, len(rows))], err
}

func (m *memRepo) Get(_ context.Context, co, id int64) (domain.Customer, error) {
	c, ok := m.customers[id]
	if !ok || c.CompanyID != co {
		return domain.Customer{}, application.ErrNotFound
	}
	return c, nil
}

func (m *memRepo) GetShared(ctx context.Context, co, id int64) (domain.Customer, error) {
	return m.Get(ctx, co, id)
}

func (m *memRepo) GetForUpdate(ctx context.Context, co, id int64) (domain.Customer, error) {
	return m.Get(ctx, co, id)
}

func (m *memRepo) WithTaxID(_ context.Context, co int64, taxID string, except int64) ([]application.Duplicate, error) {
	var out []application.Duplicate
	for _, c := range m.customers {
		if c.CompanyID == co && c.TaxID == taxID && c.ID != except {
			out = append(out, application.Duplicate{ID: c.ID, Code: c.Code, Name: c.Name, IsActive: c.IsActive})
		}
	}
	return out, nil
}

func (m *memRepo) CodeTaken(_ context.Context, co int64, code string) (bool, error) {
	for _, c := range m.customers {
		if c.CompanyID == co && c.Code == code {
			return true, nil
		}
	}
	return false, nil
}

func (m *memRepo) Insert(ctx context.Context, c domain.Customer) (int64, error) {
	if taken, _ := m.CodeTaken(ctx, c.CompanyID, c.Code); taken {
		return 0, application.ErrCodeTaken
	}
	c.ID = m.id()
	m.customers[c.ID] = c
	return c.ID, nil
}

func (m *memRepo) Update(_ context.Context, c domain.Customer) error {
	for _, o := range m.customers {
		if o.CompanyID == c.CompanyID && o.Code == c.Code && o.ID != c.ID {
			return application.ErrCodeTaken
		}
	}
	m.customers[c.ID] = c
	return nil
}

func (m *memRepo) SetActive(_ context.Context, _, id int64, active bool, by int64, at time.Time) error {
	c := m.customers[id]
	c.IsActive, c.UpdatedBy, c.UpdatedAt = active, by, at
	m.customers[id] = c
	return nil
}

func (m *memRepo) SetHold(_ context.Context, _, id int64, hold bool, reason string, by int64, at time.Time) error {
	c := m.customers[id]
	c.OnHold, c.HoldReason, c.UpdatedBy, c.UpdatedAt = hold, reason, by, at
	m.customers[id] = c
	return nil
}

func (m *memRepo) Addresses(_ context.Context, co, customer int64) ([]domain.Address, error) {
	var out []domain.Address
	for _, a := range m.addresses {
		if a.CompanyID == co && a.CustomerID == customer {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *memRepo) Address(_ context.Context, co, customer, id int64) (domain.Address, error) {
	a, ok := m.addresses[id]
	if !ok || a.CompanyID != co || a.CustomerID != customer {
		return domain.Address{}, application.ErrNotFound
	}
	return a, nil
}

func (m *memRepo) InsertAddress(_ context.Context, a domain.Address) (int64, error) {
	a.ID = m.id()
	m.addresses[a.ID] = a
	return a.ID, nil
}

func (m *memRepo) UpdateAddress(_ context.Context, a domain.Address) error {
	m.addresses[a.ID] = a
	return nil
}

func (m *memRepo) SetAddressActive(_ context.Context, _, _, id int64, active bool, at time.Time) error {
	a := m.addresses[id]
	a.IsActive, a.UpdatedAt = active, at
	if !active {
		a.IsDefaultBilling, a.IsDefaultShipping = false, false
	}
	m.addresses[id] = a
	return nil
}

func (m *memRepo) ClearDefaults(_ context.Context, co, customer, except int64, billing, shipping bool) error {
	for id, a := range m.addresses {
		if a.CompanyID == co && a.CustomerID == customer && id != except {
			a.IsDefaultBilling = a.IsDefaultBilling && !billing
			a.IsDefaultShipping = a.IsDefaultShipping && !shipping
			m.addresses[id] = a
		}
	}
	return nil
}

func (m *memRepo) Contacts(_ context.Context, co, customer int64) ([]domain.Contact, error) {
	var out []domain.Contact
	for _, c := range m.contacts {
		if c.CompanyID == co && c.CustomerID == customer {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memRepo) Contact(_ context.Context, co, customer, id int64) (domain.Contact, error) {
	c, ok := m.contacts[id]
	if !ok || c.CompanyID != co || c.CustomerID != customer {
		return domain.Contact{}, application.ErrNotFound
	}
	return c, nil
}

func (m *memRepo) InsertContact(_ context.Context, c domain.Contact) (int64, error) {
	c.ID = m.id()
	m.contacts[c.ID] = c
	return c.ID, nil
}

func (m *memRepo) UpdateContact(_ context.Context, c domain.Contact) error {
	m.contacts[c.ID] = c
	return nil
}

func (m *memRepo) DeleteContact(ctx context.Context, co, customer, id int64) error {
	if _, err := m.Contact(ctx, co, customer, id); err != nil {
		return err
	}
	delete(m.contacts, id)
	return nil
}

func (m *memRepo) ClearPrimaryContact(_ context.Context, co, customer, except int64) error {
	for id, c := range m.contacts {
		if c.CompanyID == co && c.CustomerID == customer && id != except {
			c.IsPrimary = false
			m.contacts[id] = c
		}
	}
	return nil
}

func (m *memRepo) Settings(_ context.Context, co int64) (application.Settings, error) {
	if s, ok := m.settings[co]; ok {
		return s, nil
	}
	return application.Settings{CodePrefix: application.DefaultPrefix}, nil
}

func (m *memRepo) LockSettings(ctx context.Context, co int64, _ time.Time) (application.Settings, error) {
	s, _ := m.Settings(ctx, co)
	m.settings[co] = s
	return s, nil
}

func (m *memRepo) SetLastNumber(_ context.Context, co, n int64) error {
	s := m.settings[co]
	s.LastNumber = n
	m.settings[co] = s
	return nil
}

func (m *memRepo) SetPrefix(_ context.Context, co int64, prefix string, _ time.Time) error {
	s := m.settings[co]
	s.CodePrefix = prefix
	m.settings[co] = s
	return nil
}

type authz map[string]kernel.Units

func (a authz) Units(_ context.Context, p string) (kernel.Units, error) { return a[p], nil }

type signedIn struct{}

func (signedIn) UserID(context.Context) (int64, bool) { return 7, true }

var all = kernel.Units{AllBranches: true}

func everything() authz {
	return authz{application.PermManage: all, application.PermCredit: all, application.PermSensitive: all}
}

func setup(t *testing.T, perms authz) (*application.Service, *memRepo, *kerneltest.Kit, context.Context) {
	t.Helper()
	k := kerneltest.Deps(t)
	repo := newMemRepo()
	svc := application.NewService(repo, k.Deps.Tx, k.Deps.Audit, perms, signedIn{}, k.Clock)
	return svc, repo, k, kerneltest.In(context.Background(), 1, 0)
}

func company(name string) domain.Customer {
	return domain.Customer{Name: name, PartyType: domain.Company, TaxIDKind: taxid.None}
}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

func fields(t *testing.T, err error) map[string]bool {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok || e.Status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %v", err)
	}
	out := map[string]bool{}
	for _, f := range e.Details.([]validator.FieldError) {
		out[f.Field] = true
	}
	return out
}

func create(t *testing.T, svc *application.Service, ctx context.Context, c domain.Customer) domain.Customer {
	t.Helper()
	s, err := svc.Create(ctx, application.Input{Customer: c})
	if err != nil {
		t.Fatalf("Create(%s): %v", c.Name, err)
	}
	return s.Customer
}

func TestCreateGeneratesCodesAndSkipsTypedOnes(t *testing.T) {
	svc, _, k, ctx := setup(t, everything())
	if c := create(t, svc, ctx, company("Toko A")); c.Code != "PLG-00001" {
		t.Errorf("first code = %s", c.Code)
	}
	typed := company("Toko B")
	typed.Code = "plg-00002"
	if c := create(t, svc, ctx, typed); c.Code != "PLG-00002" {
		t.Errorf("typed code = %s; want it in capitals", c.Code)
	}
	if c := create(t, svc, ctx, company("Toko C")); c.Code != "PLG-00003" {
		t.Errorf("a generated code passes over a typed one: %s", c.Code)
	}
	if _, err := svc.Create(ctx, application.Input{Customer: typed}); !fields(t, err)["code"] {
		t.Errorf("a code in use: %v", err)
	}
	if err := svc.SetPrefix(ctx, "c/"); err != nil {
		t.Fatal(err)
	}
	if c := create(t, svc, ctx, company("Toko D")); c.Code != "C/00004" {
		t.Errorf("after a prefix change = %s", c.Code)
	}
	// Another company has its own counter.
	if c := create(t, svc, kerneltest.In(context.Background(), 2, 0), company("Toko Z")); c.Code != "PLG-00001" {
		t.Errorf("company 2 = %s", c.Code)
	}
	if n := len(k.Audit.Records()); n == 0 {
		t.Error("no audit records")
	}
}

func TestTaxIDsAreNormalizedAndChecked(t *testing.T) {
	svc, _, _, ctx := setup(t, everything())
	c := company("PT Lama")
	c.TaxIDKind, c.TaxID, c.IsPKP = taxid.NPWP, "01.302.442.7-092.000", true
	saved := create(t, svc, ctx, c)
	if saved.TaxID != "0013024427092000" {
		t.Errorf("a 15-digit NPWP is stored as %q", saved.TaxID)
	}

	suspect := company("PT Curiga")
	suspect.TaxIDKind, suspect.TaxID = taxid.NPWP, "0013024420092000"
	s, err := svc.Create(ctx, application.Input{Customer: suspect})
	if err != nil || len(s.Warnings) != 1 {
		t.Errorf("a suspect check digit saves with a warning: %v %v", s.Warnings, err)
	}

	for name, bad := range map[string]domain.Customer{
		"NIK for a company":     {Name: "X", PartyType: domain.Company, TaxIDKind: taxid.NIK, TaxID: "3201011508900001"},
		"bad NIK date":          {Name: "X", PartyType: domain.Individual, TaxIDKind: taxid.NIK, TaxID: "3201013208900001"},
		"passport at home":      {Name: "X", PartyType: domain.Individual, TaxIDKind: taxid.Passport, TaxID: "E1234567"},
		"PKP without a tax ID":  {Name: "X", PartyType: domain.Company, TaxIDKind: taxid.None, IsPKP: true},
		"foreign with ID":       {Name: "X", PartyType: domain.Company, TaxIDKind: taxid.None, IsForeign: true, Country: "ID"},
		"government collector":  {Name: "X", PartyType: domain.Company, TaxIDKind: taxid.None, VATCollector: domain.GovernmentCollector},
		"no name":               {PartyType: domain.Company, TaxIDKind: taxid.None},
		"negative payment days": {Name: "X", PartyType: domain.Company, TaxIDKind: taxid.None, PaymentDays: ptr(-1)},
	} {
		if _, err := svc.Create(ctx, application.Input{Customer: bad}); err == nil || status(t, err) != http.StatusUnprocessableEntity {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func ptr(n int) *int { return &n }

func TestDuplicateTaxIDNeedsConfirmation(t *testing.T) {
	svc, _, _, ctx := setup(t, everything())
	c := company("PT Satu")
	c.TaxIDKind, c.TaxID = taxid.NPWP, "0013024427092000"
	first := create(t, svc, ctx, c)
	c.Name = "PT Satu Cabang Bandung"
	_, err := svc.Create(ctx, application.Input{Customer: c})
	var dup *application.DuplicateTaxIDError
	if !errors.As(err, &dup) || len(dup.Duplicates) != 1 || dup.Duplicates[0].ID != first.ID {
		t.Fatalf("a duplicate NPWP: %v", err)
	}
	if _, err := svc.Create(ctx, application.Input{Customer: c, ConfirmDuplicate: true}); err != nil {
		t.Errorf("confirmed: %v", err)
	}
	// Another company's customer with the same NPWP is no duplicate.
	if _, err := svc.Create(ctx, application.Input{Customer: c}); err == nil {
		t.Error("third time without confirmation saved")
	}
	if _, err := svc.Create(kerneltest.In(context.Background(), 2, 0), application.Input{Customer: c}); err != nil {
		t.Errorf("company 2: %v", err)
	}
}

func TestTermsNeedCreditAcrossTheCompany(t *testing.T) {
	branchOnly := authz{application.PermManage: {BranchIDs: []int64{3}}, application.PermCredit: {BranchIDs: []int64{3}}}
	svc, repo, k, ctx := setup(t, branchOnly)
	c := company("Toko Kredit")
	c.PaymentDays = ptr(30)
	if _, err := svc.Create(ctx, application.Input{Customer: c}); err == nil || status(t, err) != http.StatusForbidden {
		t.Errorf("terms from a branch-scoped role: %v", err)
	}
	saved := create(t, svc, ctx, company("Toko Kredit"))
	// Changing other fields with the stored terms sent back is fine.
	saved.Phone = "022 123"
	if _, err := svc.Update(ctx, saved.ID, application.Input{Customer: saved}); err != nil {
		t.Errorf("an update that keeps the terms: %v", err)
	}
	limit := money.MustAmount("50000000")
	saved.CreditLimit = &limit
	if _, err := svc.Update(ctx, saved.ID, application.Input{Customer: saved}); err == nil || status(t, err) != http.StatusForbidden {
		t.Errorf("a new limit from a branch-scoped role: %v", err)
	}

	full := application.NewService(repo, k.Deps.Tx, k.Deps.Audit, everything(), signedIn{}, k.Clock)
	if _, err := full.Update(ctx, saved.ID, application.Input{Customer: saved}); err != nil {
		t.Fatal(err)
	}
	var security bool
	for _, r := range k.Audit.Records() {
		security = security || (r.Kind == kernel.AuditSecurity && r.Action == "customer.terms_changed")
	}
	if !security {
		t.Error("a terms change is not a security record")
	}
}

func TestPersonalDataIsMaskedAndRevealIsAudited(t *testing.T) {
	svc, repo, k, ctx := setup(t, everything())
	p := domain.Customer{Name: "Budi", PartyType: domain.Individual, TaxIDKind: taxid.NIK, TaxID: "3201011508900001"}
	saved := create(t, svc, ctx, p)
	if saved.TaxID != "3201********0001" {
		t.Errorf("Create returned %q", saved.TaxID)
	}
	d, _ := svc.Detail(ctx, saved.ID)
	if d.Customer.TaxID != "3201********0001" {
		t.Errorf("Detail returned %q", d.Customer.TaxID)
	}
	// The form sends the masked value back: the stored NIK stays.
	edit := d.Customer
	edit.Name = "Budi Santoso"
	if _, err := svc.Update(ctx, saved.ID, application.Input{Customer: edit}); err != nil {
		t.Fatal(err)
	}
	if got := repo.customers[saved.ID].TaxID; got != "3201011508900001" {
		t.Errorf("after an update with the masked value the NIK is %q", got)
	}

	whole, err := svc.Reveal(ctx, saved.ID)
	if err != nil || whole.TaxID != "3201011508900001" {
		t.Fatalf("Reveal = %q, %v", whole.TaxID, err)
	}
	var access bool
	for _, r := range k.Audit.Records() {
		access = access || r.Kind == kernel.AuditAccess
	}
	if !access {
		t.Error("reveal left no access record")
	}

	noSensitive := application.NewService(repo, k.Deps.Tx, k.Deps.Audit, authz{application.PermManage: all}, signedIn{}, k.Clock)
	if _, err := noSensitive.Reveal(ctx, saved.ID); err == nil || status(t, err) != http.StatusForbidden {
		t.Errorf("reveal without customers.sensitive: %v", err)
	}
}

func TestUsableHoldAndDeactivation(t *testing.T) {
	svc, _, _, ctx := setup(t, everything())
	c := create(t, svc, ctx, company("Toko Tahan"))
	if err := svc.Usable(ctx, c.ID); err != nil {
		t.Fatalf("a fresh customer: %v", err)
	}
	if err := svc.Hold(ctx, c.ID, " "); err == nil || status(t, err) != http.StatusUnprocessableEntity {
		t.Errorf("hold without a reason: %v", err)
	}
	if err := svc.Hold(ctx, c.ID, "tunggakan 90 hari"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Usable(ctx, c.ID); !errors.Is(err, application.ErrOnHold) {
		t.Errorf("held: %v", err)
	}
	if err := svc.Release(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, c.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Usable(ctx, c.ID); !errors.Is(err, application.ErrInactive) {
		t.Errorf("inactive: %v", err)
	}
	if _, err := svc.Get(ctx, c.ID); err != nil {
		t.Errorf("Get of an inactive customer: %v", err)
	}
	if err := svc.Usable(kerneltest.In(context.Background(), 2, 0), c.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("from company 2: %v", err)
	}
}

func TestAddressDefaultsAndNITKU(t *testing.T) {
	svc, _, _, ctx := setup(t, everything())
	c := company("PT Alamat")
	c.TaxIDKind, c.TaxID = taxid.NPWP, "0013024427092000"
	cu := create(t, svc, ctx, c)
	hq, err := svc.SaveAddress(ctx, cu.ID, domain.Address{Label: "Pusat", Line: "Jl. A 1", ForBilling: true, ForShipping: true, IsDefaultBilling: true, IsDefaultShipping: true, NITKU: "0013024427092000000000"})
	if err != nil {
		t.Fatal(err)
	}
	wh, err := svc.SaveAddress(ctx, cu.ID, domain.Address{Label: "Gudang", Line: "Jl. B 2", ForShipping: true, IsDefaultShipping: true})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := svc.Detail(ctx, cu.ID)
	for _, a := range d.Addresses {
		if a.ID == hq.ID && (!a.IsDefaultBilling || a.IsDefaultShipping) {
			t.Errorf("headquarters keeps billing, loses shipping: %+v", a)
		}
		if a.ID == wh.ID && !a.IsDefaultShipping {
			t.Errorf("the warehouse is the default shipping address: %+v", a)
		}
	}
	if _, err := svc.SaveAddress(ctx, cu.ID, domain.Address{Label: "X", Line: "Jl. C", ForBilling: true, NITKU: "0099999999999999000001"}); !fields(t, err)["nitku"] {
		t.Errorf("another NPWP's NITKU: %v", err)
	}
	if _, err := svc.SaveAddress(ctx, cu.ID, domain.Address{Label: "X", Line: "Jl. C"}); !fields(t, err)["for_billing"] {
		t.Errorf("an address for nothing: %v", err)
	}

	// Snapshot: the default billing address, its NITKU, and the whole NPWP.
	snapC, snapA, err := svc.Snapshot(ctx, cu.ID, 0)
	if err != nil || snapA.ID != hq.ID || snapA.NITKU != "0013024427092000000000" || snapC.TaxID != "0013024427092000" {
		t.Errorf("Snapshot = %+v %+v %v", snapC, snapA, err)
	}
	if err := svc.SetAddressActive(ctx, cu.ID, hq.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, a, _ := svc.Snapshot(ctx, cu.ID, 0); a.ID != 0 {
		t.Errorf("with no active billing address, Snapshot gave %+v", a)
	}
}

func TestContacts(t *testing.T) {
	svc, _, _, ctx := setup(t, everything())
	cu := create(t, svc, ctx, company("PT Kontak"))
	a, err := svc.SaveContact(ctx, cu.ID, domain.Contact{Name: "Ani", IsPrimary: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveContact(ctx, cu.ID, domain.Contact{Name: "Budi", IsPrimary: true, Email: "BUDI@x.id"}); err != nil {
		t.Fatal(err)
	}
	d, _ := svc.Detail(ctx, cu.ID)
	for _, c := range d.Contacts {
		if c.ID == a.ID && c.IsPrimary {
			t.Error("two primary contacts")
		}
		if c.Name == "Budi" && c.Email != "budi@x.id" {
			t.Errorf("email kept as %q", c.Email)
		}
	}
	if err := svc.DeleteContact(ctx, cu.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteContact(ctx, cu.ID, a.ID); err == nil || status(t, err) != http.StatusNotFound {
		t.Errorf("deleting twice: %v", err)
	}
	if _, err := svc.SaveContact(ctx, cu.ID, domain.Contact{Name: "C", Email: "bukan email"}); !fields(t, err)["email"] {
		t.Errorf("a bad email: %v", err)
	}
}

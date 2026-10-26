package application_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/settings/application"
	"__GO_MODULE__/internal/modules/settings/contracts"
	"__GO_MODULE__/internal/modules/settings/domain"
	"__GO_MODULE__/internal/modules/settings/ports"
	"__GO_MODULE__/internal/shared/apperror"
)

// memRepo keeps settings as JSON, like the table.
type memRepo struct{ rows map[string][]byte }

func (m *memRepo) Get(_ context.Context, name string, dst any) (bool, error) {
	raw, ok := m.rows[name]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

func (m *memRepo) Update(_ context.Context, name string, _ int64, dst any, change func() error) error {
	if raw, ok := m.rows[name]; ok {
		if err := json.Unmarshal(raw, dst); err != nil {
			return err
		}
	}
	if err := change(); err != nil {
		return err
	}
	raw, err := json.Marshal(dst)
	m.rows[name] = raw
	return err
}

// memFiles is the FileStore port in memory.
type memFiles struct {
	next    int64
	files   map[int64][]byte
	deleted []int64
}

func (f *memFiles) Save(_ context.Context, _ string, r io.Reader, max int64, _ []string) (int64, error) {
	b, _ := io.ReadAll(r)
	if int64(len(b)) > max {
		return 0, ports.ErrFileRejected
	}
	f.next++
	f.files[f.next] = b
	return f.next, nil
}

func (f *memFiles) Open(_ context.Context, id int64) (io.ReadCloser, string, error) {
	b, ok := f.files[id]
	if !ok {
		return nil, "", ports.ErrFileNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), "image/png", nil
}

func (f *memFiles) Delete(_ context.Context, id int64) error {
	delete(f.files, id)
	f.deleted = append(f.deleted, id)
	return nil
}

type audit struct{ got []kernel.AuditEntry }

func (a *audit) Record(_ context.Context, e kernel.AuditEntry) { a.got = append(a.got, e) }

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := apperror.As(err)
	if !ok {
		t.Fatalf("want *apperror.Error, got %v", err)
	}
	return e.Status
}

var valid = domain.Company{Name: "Toko Maju", TaxID: "01.234.567.8-901.000", Email: "Info@TokoMaju.com"}

func TestCompanyDefaultsAndValidation(t *testing.T) {
	svc := application.NewService(&memRepo{rows: map[string][]byte{}}, nil, &audit{})
	c, err := svc.Company(context.Background())
	if err != nil || c.Timezone != "Asia/Jakarta" {
		t.Fatalf("a fresh install: %+v %v", c, err)
	}
	for name, in := range map[string]domain.Company{
		"no name":      {TaxID: "012345678901000"},
		"bad email":    {Name: "X", Email: "not-an-email"},
		"bad NPWP":     {Name: "X", TaxID: "123"},
		"bad timezone": {Name: "X", Timezone: "Mars/Olympus"},
	} {
		if _, err := svc.UpdateCompany(context.Background(), 1, in); status(t, err) != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %v", name, err)
		}
	}
	got, err := svc.UpdateCompany(context.Background(), 1, valid)
	if err != nil || got.Email != "info@tokomaju.com" || got.Timezone != "Asia/Jakarta" {
		t.Fatalf("save: %+v %v", got, err)
	}
}

func TestOnlyChangesAreAudited(t *testing.T) {
	a := &audit{}
	svc := application.NewService(&memRepo{rows: map[string][]byte{}}, nil, a)
	_, _ = svc.UpdateCompany(context.Background(), 1, valid)
	_, _ = svc.UpdateCompany(context.Background(), 1, valid) // nothing changed
	next := valid
	next.Phone = "021-555"
	_, _ = svc.UpdateCompany(context.Background(), 1, next)
	if len(a.got) != 2 || !strings.Contains(fmtData(a.got[1].Data), "phone") || strings.Contains(fmtData(a.got[1].Data), "name") {
		t.Fatalf("audit: %+v", a.got)
	}
}

func fmtData(d map[string]any) string {
	b, _ := json.Marshal(d)
	return string(b)
}

func TestLogoReplacesAndDeletesTheOldFile(t *testing.T) {
	files := &memFiles{files: map[int64][]byte{}}
	svc := application.NewService(&memRepo{rows: map[string][]byte{}}, files, &audit{})
	_, _ = svc.UpdateCompany(context.Background(), 1, valid)
	if _, err := svc.SetLogo(context.Background(), 1, "a.png", strings.NewReader("png-1")); err != nil {
		t.Fatal(err)
	}
	c, err := svc.SetLogo(context.Background(), 1, "b.png", strings.NewReader("png-2"))
	if err != nil || c.LogoFileID != 2 || len(files.deleted) != 1 || files.deleted[0] != 1 {
		t.Fatalf("logo: %+v %v deleted=%v", c, err, files.deleted)
	}
	// Saving the profile keeps the logo.
	c, _ = svc.UpdateCompany(context.Background(), 1, valid)
	if c.LogoFileID != 2 {
		t.Fatal("saving the profile dropped the logo")
	}
	rc, _, err := svc.Logo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	if string(b) != "png-2" {
		t.Fatalf("logo content %q", b)
	}
	if _, err := svc.SetLogo(context.Background(), 1, "big.png", strings.NewReader(strings.Repeat("x", application.LogoMaxBytes+1))); status(t, err) != http.StatusUnprocessableEntity {
		t.Fatalf("an oversized logo: want 422, got %v", err)
	}
}

func TestWithoutAFileStoreThereIsNoLogo(t *testing.T) {
	svc := application.NewService(&memRepo{rows: map[string][]byte{}}, nil, &audit{})
	if svc.Options().LogoUpload {
		t.Fatal("logo upload offered without a file store")
	}
	if _, err := svc.SetLogo(context.Background(), 1, "a.png", strings.NewReader("x")); status(t, err) != http.StatusNotImplemented {
		t.Fatalf("want 501, got %v", err)
	}
	if _, _, err := svc.Logo(context.Background()); !errors.Is(err, contracts.ErrNoLogo) {
		t.Fatalf("want ErrNoLogo, got %v", err)
	}
}

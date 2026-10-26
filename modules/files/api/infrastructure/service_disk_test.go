package infrastructure_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/files/application"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/modules/files/domain"
	"__GO_MODULE__/internal/modules/files/infrastructure"
	"__GO_MODULE__/internal/shared/pagination"
)

type memRepo struct {
	rows    map[int64]*domain.Stored
	deleted map[int64]time.Time
	next    int64
}

func (m *memRepo) Create(_ context.Context, f domain.Stored) (int64, error) {
	m.next++
	f.ID = m.next
	m.rows[f.ID] = &f
	return f.ID, nil
}

func (m *memRepo) Get(_ context.Context, id int64) (domain.Stored, error) {
	f, ok := m.rows[id]
	if _, gone := m.deleted[id]; !ok || gone {
		return domain.Stored{}, contracts.ErrNotFound
	}
	return *f, nil
}

func (m *memRepo) SoftDelete(_ context.Context, id int64, at time.Time) (bool, error) {
	if _, gone := m.deleted[id]; gone {
		return false, nil
	}
	m.deleted[id] = at
	return true, nil
}

func (m *memRepo) List(context.Context, string, string, pagination.Params) ([]domain.Stored, int64, error) {
	return nil, 0, nil
}

func (m *memRepo) PurgeCandidates(_ context.Context, before time.Time) ([]application.Purgeable, error) {
	var out []application.Purgeable
	for id, at := range m.deleted {
		if at.Before(before) {
			out = append(out, application.Purgeable{ID: id, Key: m.rows[id].Key})
		}
	}
	return out, nil
}

func (m *memRepo) DeleteRow(_ context.Context, id int64) error {
	delete(m.rows, id)
	delete(m.deleted, id)
	return nil
}

type noAudit struct{}

func (noAudit) Record(context.Context, kernel.AuditEntry) {}

type nobody struct{}

func (nobody) UserID(context.Context) (int64, bool) { return 7, true }

var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")

func setup(t *testing.T) (*application.Service, *memRepo, string) {
	t.Helper()
	root := t.TempDir()
	disk := infrastructure.Disk{Root: root}
	if err := disk.Check(); err != nil {
		t.Fatal(err)
	}
	repo := &memRepo{rows: map[int64]*domain.Stored{}, deleted: map[int64]time.Time{}}
	return application.NewService(repo, disk, noAudit{}, nobody{}, 1<<20), repo, root
}

func filesUnder(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestSaveOpenDelete(t *testing.T) {
	svc, _, root := setup(t)
	f, err := svc.Save(context.Background(), contracts.Upload{Owner: "settings", Name: "../logo.png", Reader: bytes.NewReader(png)})
	if err != nil {
		t.Fatal(err)
	}
	if f.ContentType != "image/png" || f.Name != "logo.png" || f.Size != int64(len(png)) || len(f.SHA256) != 64 {
		t.Fatalf("file = %+v", f)
	}
	_, rc, err := svc.Open(context.Background(), f.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, png) {
		t.Fatal("content differs")
	}
	if err := svc.Delete(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Open(context.Background(), f.ID); !errors.Is(err, contracts.ErrNotFound) {
		t.Fatalf("a deleted file opened: %v", err)
	}
	if err := svc.Purge(context.Background()); err != nil || len(filesUnder(t, root)) != 1 {
		t.Fatal("the bytes must stay for 30 days after a delete")
	}
}

func TestRefusedUploadsLeaveNothingBehind(t *testing.T) {
	svc, repo, root := setup(t)
	for name, u := range map[string]contracts.Upload{
		"svg":                         {Owner: "settings", Name: "x.svg", Reader: strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
		"html as png":                 {Owner: "settings", Name: "x.png", Reader: strings.NewReader("<html><script>alert(1)</script></html>")},
		"too large":                   {Owner: "settings", Name: "big.png", Reader: io.MultiReader(bytes.NewReader(png), bytes.NewReader(make([]byte, 2<<20)))},
		"over the module's own limit": {Owner: "settings", Name: "a.png", Reader: io.MultiReader(bytes.NewReader(png), bytes.NewReader(make([]byte, 600))), MaxBytes: 100},
		"not in the module's list":    {Owner: "settings", Name: "a.png", Reader: bytes.NewReader(png), Allowed: []string{"application/pdf"}},
		"empty":                       {Owner: "settings", Name: "a.png", Reader: strings.NewReader("")},
	} {
		if _, err := svc.Save(context.Background(), u); !errors.Is(err, contracts.ErrRejected) {
			t.Errorf("%s: want ErrRejected, got %v", name, err)
		}
	}
	if left := filesUnder(t, root); len(left) != 0 || len(repo.rows) != 0 {
		t.Fatalf("refused uploads left files %v / rows %d", left, len(repo.rows))
	}
	if _, err := svc.Save(context.Background(), contracts.Upload{Owner: "../etc", Name: "a.png", Reader: bytes.NewReader(png)}); err == nil {
		t.Fatal("an owner that is not a module id was accepted")
	}
}

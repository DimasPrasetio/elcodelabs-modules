// Package application holds the files module's use cases.
package application

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/modules/files/domain"
	"__GO_MODULE__/internal/shared/pagination"
)

// keepDeleted is how long a deleted file's bytes stay on disk (and in the backups).
const keepDeleted = 30 * 24 * time.Hour

// ErrTooLarge is what a Storage reports when the content passes the limit.
var ErrTooLarge = errors.New("too large")

// Purgeable is a deleted file whose bytes can go.
type Purgeable struct {
	ID  int64
	Key string
}

// Repository is the files table.
type Repository interface {
	Create(ctx context.Context, f domain.Stored) (int64, error)
	// Get returns an undeleted file, or contracts.ErrNotFound.
	Get(ctx context.Context, id int64) (domain.Stored, error)
	SoftDelete(ctx context.Context, id int64, at time.Time) (bool, error)
	List(ctx context.Context, owner, search string, p pagination.Params) ([]domain.Stored, int64, error)
	PurgeCandidates(ctx context.Context, before time.Time) ([]Purgeable, error)
	DeleteRow(ctx context.Context, id int64) error
}

// Storage keeps the bytes.
type Storage interface {
	// Write streams r into a new object, refusing (ErrTooLarge) past limit bytes.
	Write(ctx context.Context, r io.Reader, limit int64) (key string, size int64, sha256 string, err error)
	Open(key string) (io.ReadSeekCloser, error)
	Remove(key string) error
	// CleanPartials removes writes abandoned half-way (a crash, a cut connection).
	CleanPartials(olderThan time.Duration) error
}

// Auditor records deletions — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Identity tells who is signed in — the kernel's d.Auth.
type Identity interface {
	UserID(ctx context.Context) (int64, bool)
}

// Service stores and serves files.
type Service struct {
	repo     Repository
	store    Storage
	audit    Auditor
	who      Identity
	maxBytes int64
	now      func() time.Time
}

func NewService(repo Repository, store Storage, audit Auditor, who Identity, maxBytes int64) *Service {
	return &Service{repo: repo, store: store, audit: audit, who: who, maxBytes: maxBytes, now: func() time.Time { return time.Now().UTC() }}
}

var ownerPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

func rejected(format string, args ...any) error {
	return fmt.Errorf("%w: %s", contracts.ErrRejected, fmt.Sprintf(format, args...))
}

// Save checks what the upload is from its first bytes, then streams it to disk — nothing is
// written for a refused type, and a too-large file is removed as soon as it passes the limit.
func (s *Service) Save(ctx context.Context, u contracts.Upload) (contracts.File, error) {
	if !ownerPattern.MatchString(u.Owner) {
		return contracts.File{}, fmt.Errorf("files: owner %q is not a module id", u.Owner)
	}
	limit := s.maxBytes
	if u.MaxBytes > 0 && u.MaxBytes < limit {
		limit = u.MaxBytes
	}
	allowed := u.Allowed
	if allowed == nil {
		allowed = domain.DefaultTypes
	}
	name := domain.CleanName(u.Name)
	br := bufio.NewReaderSize(u.Reader, 512)
	head, _ := br.Peek(512)
	if len(head) == 0 {
		return contracts.File{}, rejected("berkas kosong")
	}
	kind := domain.Detect(head, name)
	if !domain.Allowed(kind, allowed) || !domain.Allowed(kind, domain.DefaultTypes) {
		return contracts.File{}, rejected("jenis berkas tidak diizinkan (%s)", kind)
	}
	key, size, sum, err := s.store.Write(ctx, br, limit)
	if errors.Is(err, ErrTooLarge) {
		return contracts.File{}, rejected("berkas lebih besar dari %s", humanSize(limit))
	}
	if err != nil {
		return contracts.File{}, err
	}
	actor, _ := s.who.UserID(ctx)
	f := domain.Stored{Owner: u.Owner, Name: name, ContentType: kind, Size: size, SHA256: sum, Key: key, CreatedAt: s.now(), CreatedBy: actor}
	if f.ID, err = s.repo.Create(ctx, f); err != nil {
		_ = s.store.Remove(key)
		return contracts.File{}, err
	}
	return toContract(f), nil
}

func (s *Service) Get(ctx context.Context, id int64) (contracts.File, error) {
	f, err := s.repo.Get(ctx, id)
	if err != nil {
		return contracts.File{}, err
	}
	return toContract(f), nil
}

// Open returns the file and its content, ready for http.ServeContent.
func (s *Service) Open(ctx context.Context, id int64) (contracts.File, io.ReadSeekCloser, error) {
	f, err := s.repo.Get(ctx, id)
	if err != nil {
		return contracts.File{}, nil, err
	}
	rc, err := s.store.Open(f.Key)
	if err != nil {
		return contracts.File{}, nil, err
	}
	return toContract(f), rc, nil
}

// Delete takes the file out of use now; Purge removes its bytes later.
func (s *Service) Delete(ctx context.Context, id int64) error {
	f, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	ok, err := s.repo.SoftDelete(ctx, id, s.now())
	if err != nil {
		return err
	}
	if !ok {
		return contracts.ErrNotFound
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "files", Action: "file.deleted", Target: fmt.Sprintf("file:%d", id), Summary: "Menghapus berkas " + f.Name,
		Data: map[string]any{"owner": f.Owner, "name": f.Name, "size": f.Size}})
	return nil
}

func (s *Service) List(ctx context.Context, owner, search string, p pagination.Params) ([]contracts.File, int64, error) {
	rows, total, err := s.repo.List(ctx, owner, search, p)
	if err != nil {
		return nil, 0, err
	}
	out := make([]contracts.File, len(rows))
	for i, f := range rows {
		out[i] = toContract(f)
	}
	return out, total, nil
}

// Purge removes the bytes of files deleted more than 30 days ago, and abandoned partial
// writes — the job the module schedules.
func (s *Service) Purge(ctx context.Context) error {
	if err := s.store.CleanPartials(24 * time.Hour); err != nil {
		return err
	}
	gone, err := s.repo.PurgeCandidates(ctx, s.now().Add(-keepDeleted))
	if err != nil {
		return err
	}
	for _, g := range gone {
		if err := s.store.Remove(g.Key); err != nil {
			return err
		}
		if err := s.repo.DeleteRow(ctx, g.ID); err != nil {
			return err
		}
	}
	return nil
}

func toContract(f domain.Stored) contracts.File {
	return contracts.File{ID: f.ID, Owner: f.Owner, Name: f.Name, ContentType: f.ContentType, Size: f.Size, SHA256: f.SHA256, CreatedAt: f.CreatedAt}
}

func humanSize(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%d MB", n>>20)
	}
	return fmt.Sprintf("%d KB", n>>10)
}

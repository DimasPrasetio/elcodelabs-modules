// Package application holds the settings module's use cases.
package application

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/settings/contracts"
	"__GO_MODULE__/internal/modules/settings/domain"
	"__GO_MODULE__/internal/modules/settings/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// What a logo may be.
const (
	LogoMaxBytes = 2 << 20
	companyKey   = "company"
)

// LogoTypes are the image types a logo may have. No SVG: it can carry script.
var LogoTypes = []string{"image/png", "image/jpeg", "image/webp"}

// Repository stores settings as JSON documents by name.
type Repository interface {
	// Get decodes the named setting into dst; false when it was never saved.
	Get(ctx context.Context, name string, dst any) (bool, error)
	// Update reads the named setting under a row lock, lets change modify it, and saves the
	// result — so two people saving at once never lose each other's change.
	Update(ctx context.Context, name string, by int64, dst any, change func() error) error
}

// Auditor records changes — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Service is the settings module's use cases.
type Service struct {
	repo  Repository
	files ports.FileStore // nil: no logo upload
	audit Auditor
}

func NewService(repo Repository, files ports.FileStore, audit Auditor) *Service {
	return &Service{repo: repo, files: files, audit: audit}
}

var errNoFiles = apperror.New(http.StatusNotImplemented, "Unggah logo belum tersedia di aplikasi ini")

// Options says which features this install offers.
type Options struct {
	LogoUpload bool `json:"logo_upload"`
}

func (s *Service) Options() Options { return Options{LogoUpload: s.files != nil} }

// Company is the company profile; a fresh install has an empty one in Asia/Jakarta.
func (s *Service) Company(ctx context.Context) (domain.Company, error) {
	var c domain.Company
	if _, err := s.repo.Get(ctx, companyKey, &c); err != nil {
		return domain.Company{}, err
	}
	return c.Normalize(), nil
}

// UpdateCompany saves the profile (not the logo, which has its own calls).
func (s *Service) UpdateCompany(ctx context.Context, actor int64, in domain.Company) (domain.Company, error) {
	in = in.Normalize()
	var v validator.Validator
	in.Validate(&v)
	if err := v.Err(); err != nil {
		return domain.Company{}, err
	}
	var current domain.Company
	var before domain.Company
	err := s.repo.Update(ctx, companyKey, actor, &current, func() error {
		before = current.Normalize()
		in.LogoFileID = current.LogoFileID
		current = in
		return nil
	})
	if err != nil {
		return domain.Company{}, err
	}
	if changes := diff(before, in); len(changes) > 0 {
		s.audit.Record(ctx, kernel.AuditEntry{Module: "settings", Action: "settings.company_updated", Target: "settings:company", Summary: "Mengubah profil perusahaan", Data: changes})
	}
	return in, nil
}

// SetLogo stores an uploaded image as the company logo, replacing (and deleting) the old one.
func (s *Service) SetLogo(ctx context.Context, actor int64, filename string, r io.Reader) (domain.Company, error) {
	if s.files == nil {
		return domain.Company{}, errNoFiles
	}
	id, err := s.files.Save(ctx, filename, r, LogoMaxBytes, LogoTypes)
	if errors.Is(err, ports.ErrFileRejected) {
		return domain.Company{}, apperror.Validation([]validator.FieldError{{Field: "file", Message: strings.TrimPrefix(err.Error(), ports.ErrFileRejected.Error()+": ")}})
	}
	if err != nil {
		return domain.Company{}, err
	}
	var c domain.Company
	var old int64
	if err := s.repo.Update(ctx, companyKey, actor, &c, func() error {
		old, c.LogoFileID = c.LogoFileID, id
		return nil
	}); err != nil {
		_ = s.files.Delete(ctx, id)
		return domain.Company{}, err
	}
	if old != 0 {
		_ = s.files.Delete(ctx, old)
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "settings", Action: "settings.logo_updated", Target: "settings:company", Summary: "Mengganti logo perusahaan", Data: map[string]any{"file_id": id}})
	return c.Normalize(), nil
}

// RemoveLogo clears the logo and deletes its file.
func (s *Service) RemoveLogo(ctx context.Context, actor int64) (domain.Company, error) {
	if s.files == nil {
		return domain.Company{}, errNoFiles
	}
	var c domain.Company
	var old int64
	if err := s.repo.Update(ctx, companyKey, actor, &c, func() error {
		old, c.LogoFileID = c.LogoFileID, 0
		return nil
	}); err != nil {
		return domain.Company{}, err
	}
	if old != 0 {
		_ = s.files.Delete(ctx, old)
		s.audit.Record(ctx, kernel.AuditEntry{Module: "settings", Action: "settings.logo_removed", Target: "settings:company", Summary: "Menghapus logo perusahaan"})
	}
	return c.Normalize(), nil
}

// Logo opens the company logo, or contracts.ErrNoLogo.
func (s *Service) Logo(ctx context.Context) (io.ReadCloser, string, error) {
	if s.files == nil {
		return nil, "", contracts.ErrNoLogo
	}
	c, err := s.Company(ctx)
	if err != nil {
		return nil, "", err
	}
	if c.LogoFileID == 0 {
		return nil, "", contracts.ErrNoLogo
	}
	rc, kind, err := s.files.Open(ctx, c.LogoFileID)
	if errors.Is(err, ports.ErrFileNotFound) {
		return nil, "", contracts.ErrNoLogo
	}
	return rc, kind, err
}

// diff lists the fields that changed, before and after.
func diff(before, after domain.Company) map[string]any {
	out := map[string]any{}
	pairs := map[string][2]string{
		"name": {before.Name, after.Name}, "legal_name": {before.LegalName, after.LegalName}, "address": {before.Address, after.Address},
		"phone": {before.Phone, after.Phone}, "email": {before.Email, after.Email}, "tax_id": {before.TaxID, after.TaxID}, "timezone": {before.Timezone, after.Timezone},
	}
	for k, v := range pairs {
		if v[0] != v[1] {
			out[k] = map[string]any{"before": v[0], "after": v[1]}
		}
	}
	return out
}

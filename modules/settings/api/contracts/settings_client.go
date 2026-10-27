// Package contracts is what the settings module offers other modules (through their own ports
// and an adapter in the composition root).
package contracts

import (
	"context"
	"errors"
	"io"
)

// ErrNoLogo means no logo is set (or the file store is not wired).
var ErrNoLogo = errors.New("no logo")

// Company is who the app belongs to — for letterheads, invoices, emails.
type Company struct {
	Name      string
	LegalName string
	Address   string
	Phone     string
	Email     string
	TaxID     string // NPWP
	// Timezone is deprecated and unused: where a day starts follows kernel.Today — the scope's
	// zone (a branch's or company's, from an organization module), else APP_TIMEZONE.
	Timezone string
	HasLogo  bool
}

// SettingsClient reads the settings of the company ctx works in.
type SettingsClient interface {
	Company(ctx context.Context) (Company, error)
	// Logo opens the company logo (an image) with its content type, or ErrNoLogo.
	Logo(ctx context.Context) (io.ReadCloser, string, error)
}

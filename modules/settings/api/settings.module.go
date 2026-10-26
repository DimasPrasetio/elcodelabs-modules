// Package settings keeps app-wide settings: the company profile (name, address, NPWP, time
// zone, logo) other modules use on documents, and a settings page other modules add sections
// to. It offers contracts.SettingsClient and needs one optional port: a file store for the logo.
package settings

import (
	"context"
	"io"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/settings/application"
	"__GO_MODULE__/internal/modules/settings/contracts"
	"__GO_MODULE__/internal/modules/settings/infrastructure"
	"__GO_MODULE__/internal/modules/settings/ports"
	"__GO_MODULE__/internal/modules/settings/presentation"
)

// Module is the settings module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module. files is optional (nil): without it the logo cannot be uploaded.
func New(d *kernel.Deps, files ports.FileStore) (*Module, error) {
	d.Authz.Declare(kernel.Permission{Name: presentation.PermManage, Module: "settings", Description: "Mengubah pengaturan aplikasi dan profil perusahaan"})
	svc := application.NewService(infrastructure.NewRepository(d.Tx), files, d.Audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Auth, d.Authz)}, nil
}

// Routes mounts /api/v1/settings.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.SettingsClient { return client{m.svc} }

type client struct{ svc *application.Service }

func (c client) Company(ctx context.Context) (contracts.Company, error) {
	co, err := c.svc.Company(ctx)
	if err != nil {
		return contracts.Company{}, err
	}
	return contracts.Company{Name: co.Name, LegalName: co.LegalName, Address: co.Address, Phone: co.Phone, Email: co.Email,
		TaxID: co.TaxID, Timezone: co.Timezone, HasLogo: co.LogoFileID != 0}, nil
}

func (c client) Logo(ctx context.Context) (io.ReadCloser, string, error) { return c.svc.Logo(ctx) }

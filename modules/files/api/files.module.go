// Package files stores uploaded files for other modules: under DATA_DIR/files on disk, with a
// row each in the database. It offers contracts.FileClient (reached through each module's own
// port) and an administrator's page of every file. It imports no other module.
package files

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"__GO_MODULE__/internal/config"
	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/files/application"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/modules/files/infrastructure"
	"__GO_MODULE__/internal/modules/files/presentation"
)

// Module is the files module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module. FILES_MAX_BYTES caps any upload (default 10 MiB); DATA_DIR/files must
// be writable, or the API does not start.
func New(d *kernel.Deps) (*Module, error) {
	d.Authz.Declare(
		kernel.Permission{Name: presentation.PermRead, Module: "files", Description: "Melihat dan mengunduh semua berkas"},
		kernel.Permission{Name: presentation.PermManage, Module: "files", Description: "Menghapus berkas"},
	)
	maxBytes, err := strconv.ParseInt(config.Get("FILES_MAX_BYTES", strconv.Itoa(10<<20)), 10, 64)
	if err != nil || maxBytes <= 0 || maxBytes > 200<<20 {
		return nil, fmt.Errorf("files: FILES_MAX_BYTES must be a size in bytes up to 200 MiB (got %q)", config.Get("FILES_MAX_BYTES", ""))
	}
	disk := infrastructure.Disk{Root: filepath.Join(d.Config.DataDir, "files")}
	if err := disk.Check(); err != nil {
		return nil, err
	}
	svc := application.NewService(infrastructure.NewRepository(d.Tx), disk, d.Audit, d.Auth, maxBytes)
	d.Jobs.Every("files.purge", 24*time.Hour, svc.Purge)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz)}, nil
}

// Routes mounts /api/v1/files.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.FileClient { return client{m.svc} }

type client struct{ svc *application.Service }

func (c client) Save(ctx context.Context, u contracts.Upload) (contracts.File, error) {
	return c.svc.Save(ctx, u)
}
func (c client) Get(ctx context.Context, id int64) (contracts.File, error) { return c.svc.Get(ctx, id) }
func (c client) Delete(ctx context.Context, id int64) error                { return c.svc.Delete(ctx, id) }
func (c client) Lock(ctx context.Context, id int64) error                  { return c.svc.Lock(ctx, id) }
func (c client) Open(ctx context.Context, id int64) (contracts.File, io.ReadSeekCloser, error) {
	return c.svc.Open(ctx, id)
}

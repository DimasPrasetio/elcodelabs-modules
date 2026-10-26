// Package presentation is the files module's HTTP API: an administrator's view of every stored
// file. A module serves its own files through its own endpoints.
package presentation

import (
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/files/application"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

// Permissions this module declares (files.module.go) and guards with.
const (
	PermRead   = "files.read"
	PermManage = "files.manage"
)

type fileDTO struct {
	ID          int64     `json:"id"`
	Owner       string    `json:"owner"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"created_at"`
}

func toDTO(f contracts.File) fileDTO {
	return fileDTO{ID: f.ID, Owner: f.Owner, Name: f.Name, ContentType: f.ContentType, Size: f.Size, SHA256: f.SHA256, CreatedAt: f.CreatedAt}
}

// Handler serves /api/v1/files.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/files", h.authz.Require(PermRead, http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/files/{id}/download", h.authz.Require(PermRead, http.HandlerFunc(h.download)))
	mux.Handle("DELETE /api/v1/files/{id}", h.authz.Require(PermManage, http.HandlerFunc(h.delete)))
}

func notFound(err error) error {
	if errors.Is(err, contracts.ErrNotFound) {
		return apperror.NotFound("Berkas tidak ditemukan")
	}
	return err
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := pagination.FromRequest(r)
	q := r.URL.Query()
	files, total, err := h.svc.List(r.Context(), q.Get("owner"), strings.TrimSpace(q.Get("q")), p)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]fileDTO, len(files))
	for i, f := range files {
		out[i] = toDTO(f)
	}
	response.Paginated(w, "Berkas", out, pagination.NewMeta(p, total))
}

// download sends the file as an attachment: it is saved, never rendered in the app's origin.
func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	f, rc, err := h.svc.Open(r.Context(), id)
	if err != nil {
		response.Error(w, r, notFound(err))
		return
	}
	defer rc.Close()
	request.AllowTransfer(w, f.Size)
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", f.CreatedAt, rc)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.Error(w, r, notFound(err))
		return
	}
	response.OK(w, "Berkas dihapus", nil)
}

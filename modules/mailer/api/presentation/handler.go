// Package presentation is the mailer's HTTP API: the outbox, for whoever looks after email.
package presentation

import (
	"net/http"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/mailer/application"
	"__GO_MODULE__/internal/modules/mailer/domain"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

// Permissions this module declares (mailer.module.go) and guards with.
const (
	PermRead   = "mailer.read"
	PermManage = "mailer.manage"
)

type messageDTO struct {
	ID            int64      `json:"id"`
	To            string     `json:"to"`
	ToName        string     `json:"to_name"`
	Subject       string     `json:"subject"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	NextAttemptAt time.Time  `json:"next_attempt_at"`
	LastError     string     `json:"last_error"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        *time.Time `json:"sent_at"`
}

func toDTO(m domain.Message) messageDTO {
	d := messageDTO{ID: m.ID, To: m.To, ToName: m.ToName, Subject: m.Subject, Status: m.Status, Attempts: m.Attempts,
		NextAttemptAt: m.NextAttemptAt, LastError: m.LastError, CreatedAt: m.CreatedAt}
	if !m.SentAt.IsZero() {
		d.SentAt = &m.SentAt
	}
	return d
}

// Handler serves /api/v1/mailer.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

// Routes mounts the outbox endpoints.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/mailer/messages", h.authz.Require(PermRead, http.HandlerFunc(h.list)))
	mux.Handle("POST /api/v1/mailer/messages/{id}/retry", h.authz.Require(PermManage, http.HandlerFunc(h.retry)))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := pagination.FromRequest(r)
	q := r.URL.Query()
	msgs, total, err := h.svc.List(r.Context(), q.Get("status"), strings.TrimSpace(q.Get("q")), p)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]messageDTO, len(msgs))
	for i, m := range msgs {
		out[i] = toDTO(m)
	}
	response.Paginated(w, "Kotak keluar", out, pagination.NewMeta(p, total))
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Email dijadwalkan untuk dikirim ulang", nil)
}

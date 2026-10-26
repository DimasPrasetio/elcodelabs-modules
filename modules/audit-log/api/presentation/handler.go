// Package presentation is the audit trail's HTTP API — read-only: nothing changes or removes an
// entry except the retention job.
package presentation

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/audit-log/application"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/response"
)

// PermRead is the permission this module declares and guards with.
const PermRead = "audit-log.read"

// jakarta is where "a day" starts for the date filter.
var jakarta = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*3600)
}()

type entryDTO struct {
	ID         int64           `json:"id"`
	OccurredAt time.Time       `json:"occurred_at"`
	ActorID    int64           `json:"actor_id"`
	ActorName  string          `json:"actor_name"`
	Module     string          `json:"module"`
	Action     string          `json:"action"`
	Target     string          `json:"target"`
	Summary    string          `json:"summary"`
	Data       json.RawMessage `json:"data"`
	IP         string          `json:"ip"`
	RequestID  string          `json:"request_id"`
}

// Handler serves /api/v1/audit.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/audit", h.authz.Require(PermRead, http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/audit/modules", h.authz.Require(PermRead, http.HandlerFunc(h.modules)))
}

// day parses YYYY-MM-DD as the start of that day in Jakarta.
func day(v string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", v, jakarta)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := application.Filter{Module: q.Get("module"), Target: strings.TrimSpace(q.Get("target"))}
	if v := q.Get("actor"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.Error(w, r, apperror.BadRequest("Parameter actor tidak valid"))
			return
		}
		f.Actor = id
	}
	if v := q.Get("from"); v != "" {
		t, err := day(v)
		if err != nil {
			response.Error(w, r, apperror.BadRequest("Parameter from harus berformat YYYY-MM-DD"))
			return
		}
		f.From = t.UTC()
	}
	if v := q.Get("to"); v != "" {
		t, err := day(v)
		if err != nil {
			response.Error(w, r, apperror.BadRequest("Parameter to harus berformat YYYY-MM-DD"))
			return
		}
		f.To = t.AddDate(0, 0, 1).UTC() // the whole "to" day
	}
	p := pagination.FromRequest(r)
	entries, total, err := h.svc.List(r.Context(), f, p)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]entryDTO, len(entries))
	for i, e := range entries {
		out[i] = entryDTO{ID: e.ID, OccurredAt: e.OccurredAt, ActorID: e.ActorID, ActorName: e.ActorName, Module: e.Module, Action: e.Action,
			Target: e.Target, Summary: e.Summary, Data: e.Data, IP: e.IP, RequestID: e.RequestID}
		if len(out[i].Data) == 0 || string(out[i].Data) == "null" {
			out[i].Data = nil
		}
	}
	response.Paginated(w, "Log aktivitas", out, pagination.NewMeta(p, total))
}

func (h *Handler) modules(w http.ResponseWriter, r *http.Request) {
	mods, err := h.svc.Modules(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	if mods == nil {
		mods = []string{}
	}
	response.OK(w, "Modul", mods)
}

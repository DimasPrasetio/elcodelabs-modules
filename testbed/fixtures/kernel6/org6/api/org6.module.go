// Package org6 is a testbed fixture: an authenticator trusting X-Test-User and a scope
// resolver placing user 1 in companies 1 and 2, user 2 in company 1's branch 5.
package org6

import (
	"context"
	"net/http"
	"strconv"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/shared/response"
)

type Module struct{}

func New(d *kernel.Deps) *Module { return &Module{} }

func (m *Module) Routes(mux *http.ServeMux) {}

type uidKey struct{}

type authenticator struct{}

func (authenticator) Require(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.Header.Get("X-Test-User"), 10, 64)
		if err != nil || id <= 0 {
			response.Fail(w, http.StatusUnauthorized, "no test user", nil)
			return
		}
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), uidKey{}, id)))
	})
}

func (authenticator) UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(uidKey{}).(int64)
	return id, ok
}

func (m *Module) Authenticator() kernel.Authenticator { return authenticator{} }

type resolver struct{}

func (resolver) Resolve(_ context.Context, uid int64, requested string) (kernel.Scope, error) {
	switch {
	case uid == 1 && (requested == "" || requested == "1"):
		return kernel.Scope{CompanyID: 1}, nil
	case uid == 1 && requested == "2":
		return kernel.Scope{CompanyID: 2}, nil
	case uid == 2 && (requested == "" || requested == "1:5"):
		return kernel.Scope{CompanyID: 1, BranchID: 5}, nil
	}
	return kernel.Scope{}, kernel.ErrScopeDenied
}

func (resolver) Companies(context.Context) ([]kernel.Scope, error) {
	return []kernel.Scope{{CompanyID: 1}, {CompanyID: 2}}, nil
}

func (m *Module) Resolver() kernel.ScopeResolver { return resolver{} }

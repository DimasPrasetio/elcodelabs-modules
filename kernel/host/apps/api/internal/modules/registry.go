// Package modules is the composition root — the only package that imports modules. Modules
// never import each other: a consumer declares what it needs as ports in its own ports/
// package, and an adapter beside this file (adapter_<consumer>_<port>.go) satisfies that port
// with a provider's client. Build a provider before the modules wired to it. The modules that
// do authentication and authorization fill the kernel's slots here too: d.Auth.Provide(...),
// d.Authz.Provide(...).
//
// Tooling inserts lines between the elco:* markers. Keep the markers; add modules between
// them, never around them.
package modules

import (
	"net/http"

	"testbed-app/internal/kernel"
	// elco:imports:begin
	// elco:imports:end
)

// Register builds every module and mounts its routes. An error (a module missing a required
// setting) stops the API at startup.
func Register(mux *http.ServeMux, d *kernel.Deps) error {
	// elco:modules:begin
	// elco:modules:end
	return nil
}

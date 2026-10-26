// Package mailer sends the app's email: modules queue a message through its contract
// (MailClient, reached through their own port), and a kernel task (d.Tasks) delivers it with
// retries; the outbox table is the history.
// It imports no other module.
package mailer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"time"

	"__GO_MODULE__/internal/config"
	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/mailer/application"
	"__GO_MODULE__/internal/modules/mailer/contracts"
	"__GO_MODULE__/internal/modules/mailer/infrastructure"
	"__GO_MODULE__/internal/modules/mailer/presentation"
)

// Module is the mailer module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module. With SMTP_HOST set, mail goes to that server (MAIL_FROM required);
// without it, development writes each email to the log — and production refuses to start,
// because email that is silently dropped is worse than an API that does not come up.
func New(d *kernel.Deps) (*Module, error) {
	d.Authz.Declare(
		kernel.Permission{Name: presentation.PermRead, Module: "mailer", Description: "Melihat kotak keluar email"},
		kernel.Permission{Name: presentation.PermManage, Module: "mailer", Description: "Mengirim ulang email yang gagal"},
	)
	transport, from, err := transportFromEnv(d)
	if err != nil {
		return nil, fmt.Errorf("mailer: %w", err)
	}
	svc := application.NewService(infrastructure.NewRepository(d.Tx), transport, from, d.Audit, d.Tx, d.Tasks)
	svc.RegisterTasks()
	d.Jobs.Daily("mailer.prune", "03:30", svc.Prune)
	d.Jobs.Every("mailer.rescue", 10*time.Minute, svc.Rescue)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz)}, nil
}

func transportFromEnv(d *kernel.Deps) (application.Transport, application.Sender, error) {
	from := application.Sender{Name: config.Get("MAIL_FROM_NAME", d.Config.Name), AppName: d.Config.Name}
	host := config.Get("SMTP_HOST", "")
	if host == "" {
		if d.Config.IsProduction() {
			return nil, from, errors.New("SMTP_HOST is not set — production email would be dropped (set SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, MAIL_FROM)")
		}
		from.Address = config.Get("MAIL_FROM", "noreply@localhost")
		return infrastructure.Log{Logger: d.Log}, from, nil
	}
	addr, err := config.Require("MAIL_FROM")
	if err != nil {
		return nil, from, err
	}
	if a, err := mail.ParseAddress(addr); err != nil || a.Address != addr {
		return nil, from, fmt.Errorf("MAIL_FROM %q is not an email address", addr)
	}
	from.Address = addr
	port, err := strconv.Atoi(config.Get("SMTP_PORT", "587"))
	if err != nil || port <= 0 {
		return nil, from, fmt.Errorf("SMTP_PORT must be a port number (got %q)", config.Get("SMTP_PORT", ""))
	}
	mode := config.Get("SMTP_TLS", map[bool]string{true: "tls", false: "starttls"}[port == 465])
	switch mode {
	case "starttls", "tls":
	case "none":
		if d.Config.IsProduction() {
			return nil, from, errors.New("SMTP_TLS=none sends mail and the SMTP password in the clear — not in production")
		}
	default:
		return nil, from, fmt.Errorf("SMTP_TLS must be starttls, tls, or none (got %q)", mode)
	}
	return infrastructure.SMTP{Host: host, Port: port, Username: config.Get("SMTP_USERNAME", ""), Password: config.Get("SMTP_PASSWORD", ""), TLS: mode}, from, nil
}

// Routes mounts /api/v1/mailer.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.MailClient { return client{m.svc} }

type client struct{ svc *application.Service }

func (c client) Send(ctx context.Context, msg contracts.Message) (int64, error) {
	return c.svc.Send(ctx, msg)
}

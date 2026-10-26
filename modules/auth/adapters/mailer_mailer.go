package modules

import (
	"context"

	authports "__GO_MODULE__/internal/modules/auth/ports"
	mailercontracts "__GO_MODULE__/internal/modules/mailer/contracts"
)

var _ authports.Mailer = authMailer{}

// authMailer satisfies auth's Mailer port with the mailer module's client. Mapping only.
type authMailer struct {
	mail mailercontracts.MailClient
}

func (a authMailer) Send(ctx context.Context, to, toName, subject, text, html string) error {
	_, err := a.mail.Send(ctx, mailercontracts.Message{To: to, ToName: toName, Subject: subject, Text: text, HTML: html})
	return err
}

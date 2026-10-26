package application

import (
	"html"
	"strings"
)

// Layout wraps a message's inner HTML in the app's email layout: a plain, centred card that
// renders the same in webmail and desktop clients (tables and inline styles — what email HTML
// still needs). The colors are neutral on purpose: an email cannot follow the app's theme.
func Layout(appName, subject, inner string) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`)
	b.WriteString(html.EscapeString(subject))
	b.WriteString(`</title></head><body style="margin:0;padding:0;background:#f4f5f7;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#1f2937">`)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f5f7;padding:24px 12px"><tr><td align="center">`)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border:1px solid #e5e7eb;border-radius:8px">`)
	b.WriteString(`<tr><td style="padding:20px 28px;border-bottom:1px solid #e5e7eb;font-size:15px;font-weight:600">`)
	b.WriteString(html.EscapeString(appName))
	b.WriteString(`</td></tr><tr><td style="padding:24px 28px;font-size:15px;line-height:1.6">`)
	b.WriteString(inner)
	b.WriteString(`</td></tr></table>`)
	b.WriteString(`<p style="max-width:560px;margin:16px auto 0;font-size:12px;color:#6b7280;line-height:1.5">Email ini dikirim otomatis oleh `)
	b.WriteString(html.EscapeString(appName))
	b.WriteString(`. Jangan membalas email ini.</p></td></tr></table></body></html>`)
	return b.String()
}

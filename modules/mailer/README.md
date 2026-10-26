# mailer

Sends the app's email. Fullstack (Go + React), owns `mail_outbox`.

**How it works.** A module calls `MailClient.Send(ctx, Message{To, ToName, Subject, Text, HTML})`
through its own port (an adapter in the composition root — modules never import the mailer).
The message is validated (one address, no line breaks in headers, a plain-text body always) and
**stored**; the request does not wait for a mail server. A background job (`d.Jobs`, every 10 s)
claims due messages with `FOR UPDATE SKIP LOCKED`, sends them, and records the result. A failure
is retried after 1 min, 5 min, 30 min, 2 h, and 6 h; a permanent refusal (SMTP 5xx) or the sixth
failure marks it **failed**. A run claims 10 messages at a time, each sent within a minute, under
an 11-minute lease: a message stuck in *sending* (the app stopped mid-send) goes back to the queue
only when its lease ends. A delivery that cannot be recorded stops the run and is logged — it may
be sent again. A sent message's bodies are cleared (a reset link does not stay in the
database); its row — recipient, subject, times — is kept 30 days, failed ones 90.

`HTML` is the inner body — paragraphs, a link, a button. The mailer wraps it in a plain,
client-safe layout with the app's name; email cannot follow the app's theme, so the layout is
neutral. Write the message in the sending module (it knows what to say).

**Sending for real** needs a transactional provider over SMTP (587 STARTTLS, or 465 TLS): a VPS's
own port 25 is usually blocked and its IP distrusted. Put SPF, DKIM, and DMARC for the provider on
the `MAIL_FROM` domain, or mail lands in spam. Without `SMTP_HOST`, development writes each email
to the log (the text body included — a reset link can be copied from there); **production refuses
to start** without it, so email is never dropped silently. `SMTP_TLS=none` (local test servers)
is refused in production too.

**Kernel 5.** Uses `d.Jobs` (delivery runs in one container at a time, even during a deploy) and
`d.Audit` (a resend is recorded).

**Offers** `contracts.MailClient`. **Needs** nothing.

**Web:** `/mailer/outbox` — "Kotak keluar": search, status filter, failed messages with the
server's answer, and "Kirim ulang" (`mailer.manage`). Nav entry in "Pengaturan" (`mailer.read`).

**Not in this version:** attachments, inbound mail, bulk sending, per-provider HTTP APIs.

## Changes

- **0.1.0** — first version.

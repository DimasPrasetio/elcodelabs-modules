# mailer

Sends the app's email. Fullstack (Go + React), owns `mail_outbox`.

**How it works.** A module calls `MailClient.Send(ctx, Message{To, ToName, Subject, Text, HTML})`
through its own port (an adapter in the composition root — modules never import the mailer).
The message is validated (one address, no line breaks in headers, a plain-text body always) and
**stored** with its first delivery task (`d.Tasks`) — inside the caller's `d.Tx` when there is
one, so an email goes out only if what it tells about was committed; the request does not wait
for a mail server. The kernel runs each attempt once across containers (even during a deploy)
and reclaims one whose container died. A failure is retried after 1 min, 5 min, 30 min, 2 h, and
6 h; a permanent refusal (SMTP 5xx) or the sixth failure marks it **failed**. Delivery is at
least once: a crash after the server accepted a message but before it was recorded sends it
again, and so may a message an administrator resends after its task failed mid-delivery. Every
10 minutes a rescue job queues the next attempt of any message long due with no delivery under
way (left by 0.1's own worker, or by a delivery task that failed for good before recording the
outcome); one whose task still waits, or whose attempt has started, is untouched. A send cut short by
shutdown is not counted: that attempt is made again. A sent message's bodies are cleared (a reset link does not stay in the database); its
row — recipient, subject, times — is kept 30 days, failed ones 90 (a daily job).

`HTML` is the inner body — paragraphs, a link, a button. The mailer wraps it in a plain,
client-safe layout with the app's name; email cannot follow the app's theme, so the layout is
neutral. Write the message in the sending module (it knows what to say).

**Sending for real** needs a transactional provider over SMTP (587 STARTTLS, or 465 TLS): a VPS's
own port 25 is usually blocked and its IP distrusted. Put SPF, DKIM, and DMARC for the provider on
the `MAIL_FROM` domain, or mail lands in spam. Without `SMTP_HOST`, development writes each email
to the log (the text body included — a reset link can be copied from there); **production refuses
to start** without it, so email is never dropped silently. `SMTP_TLS=none` (local test servers)
is refused in production too.

**Kernel 6.** Uses `d.Tx`, `d.Tasks` (delivery), `d.Jobs` (the daily prune, the rescue every
10 minutes), and `d.Audit` (a resend is recorded).

**Offers** `contracts.MailClient`. **Needs** nothing.

**Web:** `/mailer/outbox` — "Kotak keluar": search, status filter, failed messages with the
server's answer, and "Kirim ulang" (`mailer.manage`). Nav entry in "Pengaturan" (`mailer.read`).

**Not in this version:** attachments, inbound mail, bulk sending, per-provider HTTP APIs.

## Changes

- **0.2.0** — needs kernel 6. Delivery is a kernel task per attempt instead of the module's own
  polling job and lease: queued in the sender's transaction, run once across containers,
  reclaimed after a crash. Same retry schedule and outbox page. Messages pending at the upgrade
  are picked up by the rescue job within 10 minutes.
- **0.1.0** — first version.

# audit-log

The audit trail. Fullstack (Go + React), owns `audit_log`.

**How it works.** It fills the kernel's `d.Audit` (`d.Audit.Provide(auditLogModule.Sink())` in
the composition root). Every module records its changes with `d.Audit.Record(ctx, AuditEntry{…})`
— none of them knows this module exists — and the kernel adds when, who (the signed-in user),
the client IP, and the request ID, which ties an entry to the log lines of the same request. The
table is **append-only**: no endpoint changes or deletes an entry; a daily job (`d.Jobs`) removes
entries older than `AUDIT_RETENTION_DAYS` (default 365, at least 30) in short batches.

A record is written right after the change, in the same request. A storage failure is logged and
never fails the request — the change already happened — and an over-long value is cut, not
refused: losing a record is worse than shortening one.

**What gets recorded** is each module's choice (see their READMEs): users (created, updated,
deactivated, profile), auth (sign-ins, failures, sign-outs, password changes and resets), rbac
(roles, permissions, members), settings, mailer (resends), files (deletions). Never secrets:
modules record that a password changed, never the password.

**API** `GET /api/v1/audit?module=&target=&actor=&from=&to=&page=&limit=` (dates `YYYY-MM-DD`, days
in Asia/Jakarta) and `GET /api/v1/audit/modules`, both `audit-log.read`.

**Port** `UserDirectory` (optional; ready-made adapter: `users`) names the actors; without it
they show as "Pengguna #12".

**Web:** `/audit` — "Log aktivitas": filter by module, exact target, and dates; a row opens its
details (actor, action, target, IP, request ID, the before/after data). Nav entry in "Pengaturan"
(`audit-log.read`).

**Not in this version:** tamper-evident (signed or hash-chained) records, export.

## Changes

- **0.2.0** — needs kernel 6. Scoped: each record keeps its kind (`change`, `access` to
  personal data, `export`, `security`), company, and branch (migration 002); the trail shows the
  active company's records, filterable by kind; records written inside the change's `d.Tx` commit
  with it; date filters count days in the branch's or the app's time zone.
- **0.1.0** — first version.

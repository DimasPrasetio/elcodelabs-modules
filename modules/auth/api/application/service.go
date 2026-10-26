// Package application holds the auth module's use cases: signing in, resolving a session, and
// ending one.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/auth/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// errInvalid is the one answer to every failed sign-in, so it never tells which part was wrong.
var errInvalid = apperror.Unauthorized("Email atau kata sandi salah")

// errSession is the one answer to every unusable session.
var errSession = apperror.Unauthorized("Sesi berakhir atau tidak valid")

// errTooMany answers a sign-in the limiter refused.
var errTooMany = apperror.New(http.StatusTooManyRequests, "Terlalu banyak percobaan masuk yang gagal. Coba lagi nanti.")

// dummyHash is compared against when the email is unknown, so an unknown email takes as long
// as a wrong password and response time does not reveal which accounts exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)

// ErrNoSession means no session row has that ID.
var ErrNoSession = errors.New("session not found")

// Sign-in limits per window: one person from one address, one address over any accounts
// (password spraying), and one account from anywhere (a slow distributed guess).
const (
	maxPerEmailAndIP = 5
	maxPerIP         = 20
	maxPerEmail      = 30
)

// SessionRecord is one sign-in, stored so it can be ended before its token expires.
type SessionRecord struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	Revoked   bool
	IP        string
	UserAgent string
}

// Sessions stores sign-ins.
type Sessions interface {
	Create(ctx context.Context, r SessionRecord) error
	// Get returns the session, or ErrNoSession.
	Get(ctx context.Context, id string) (SessionRecord, error)
	Revoke(ctx context.Context, id string, at time.Time) error
	DeleteExpired(ctx context.Context, before time.Time) error
}

// Session is a signed-in user and their token.
type Session struct {
	Token     string
	ExpiresAt time.Time
	User      ports.User
}

// Client is where a sign-in comes from, recorded with the session.
type Client struct {
	IP        string
	UserAgent string
}

// Auditor records sign-ins and password changes — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Deps is what the service is built from. Passwords and Mailer are optional ports (nil turns
// "change password" and "forgot password" off); AppURL is the public origin reset links point
// at, never taken from a request header. Limiter counts sign-ins; the password flows get their
// own limiter with the same settings, so flooding one flow never locks out another.
type Deps struct {
	Users     ports.UserStore
	Passwords ports.PasswordWriter
	Mailer    ports.Mailer
	Sessions  Sessions
	Resets    Resets
	Tokens    *Tokens
	Limiter   *Limiter
	Audit     Auditor
	Log       *slog.Logger
	AppURL    string
	AppName   string
}

// Service signs users in, resolves sessions, and changes passwords.
type Service struct {
	users     ports.UserStore
	passwords ports.PasswordWriter
	mailer    ports.Mailer
	sessions  Sessions
	resets    Resets
	tokens    *Tokens
	limiter   *Limiter
	flows     *Limiter
	audit     Auditor
	log       *slog.Logger
	appURL    string
	appName   string
	now       func() time.Time
	// later runs work the answer must not wait for (the email of "forgot password").
	later func(func())
}

func NewService(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{users: d.Users, passwords: d.Passwords, mailer: d.Mailer, sessions: d.Sessions, resets: d.Resets, tokens: d.Tokens,
		limiter: d.Limiter, flows: NewLimiter(d.Limiter.Window, d.Limiter.MaxKeys), audit: d.Audit, log: log,
		appURL: strings.TrimRight(d.AppURL, "/"), appName: d.AppName, now: time.Now, later: func(f func()) { go f() }}
}

func userTarget(id int64) string { return fmt.Sprintf("user:%d", id) }

// maskEmail keeps enough of what was typed to recognise it: "a***@example.com", or "***" when
// it is not an address at all.
func maskEmail(s string) string {
	at := strings.LastIndexByte(s, '@')
	if at < 1 || at == len(s)-1 || strings.ContainsAny(s, " \t") {
		return "***"
	}
	r := []rune(s[:at])
	return string(r[0]) + "***" + s[at:]
}

// Login checks email and password and opens a session.
func (s *Service) Login(ctx context.Context, email, password string, from Client) (Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var v validator.Validator
	v.Required("email", email)
	v.MaxLen("email", email, 254)
	v.Required("password", password)
	v.MaxLen("password", password, 72)
	if err := v.Err(); err != nil {
		return Session{}, err
	}
	ip := ClientKey(from.IP)
	personKey, ipKey, emailKey := "ep:"+email+"|"+ip, "ip:"+ip, "e:"+email
	if !s.limiter.Reserve(Rule{personKey, maxPerEmailAndIP}, Rule{ipKey, maxPerIP}, Rule{emailKey, maxPerEmail}) {
		return Session{}, errTooMany
	}

	// The session starts when the password is checked: a password change that lands while this
	// sign-in is still running ends it, as it ends every session from before the change.
	checked := s.now().UTC().Truncate(time.Microsecond)
	cred, err := s.users.FindCredentials(ctx, email)
	if errors.Is(err, ports.ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		// What was typed is not an account, and may be a password typed in the wrong field:
		// only a masked form is kept.
		s.audit.Record(ctx, kernel.AuditEntry{Module: "auth", Action: "auth.sign_in_failed", Summary: "Gagal masuk sebagai " + maskEmail(email) + " (tidak terdaftar)"})
		return Session{}, errInvalid
	}
	if err != nil {
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(password)) != nil {
		s.audit.Record(ctx, kernel.AuditEntry{Module: "auth", Action: "auth.sign_in_failed", Target: userTarget(cred.UserID), Summary: "Gagal masuk sebagai " + email})
		return Session{}, errInvalid
	}
	if !cred.IsActive {
		return Session{}, apperror.Forbidden("Akun ini dinonaktifkan")
	}
	// The person proved who they are: their own counters start over. The address's counter
	// stays, so one valid account cannot be used to keep guessing others' passwords.
	s.limiter.Forget(personKey, emailKey)

	user, err := s.users.FindUser(ctx, cred.UserID)
	if err != nil {
		return Session{}, err
	}
	if !checked.After(user.PasswordChangedAt) {
		return Session{}, errInvalid
	}
	session, err := s.open(ctx, user, from, checked)
	if err != nil {
		return Session{}, err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "auth", Action: "auth.signed_in", Actor: user.ID, Target: userTarget(user.ID), Summary: "Masuk"})
	return session, nil
}

// open starts a session for user on this device at now, at the microsecond precision the
// database stores. A session opened at or before the user's last password change is already
// ended (see Resolve).
func (s *Service) open(ctx context.Context, user ports.User, from Client, now time.Time) (Session, error) {
	id, err := newSessionID()
	if err != nil {
		return Session{}, err
	}
	token, exp, err := s.tokens.Issue(user.ID, id, now)
	if err != nil {
		return Session{}, err
	}
	ua := from.UserAgent
	if len(ua) > 255 {
		ua = ua[:255]
	}
	if err := s.sessions.Create(ctx, SessionRecord{ID: id, UserID: user.ID, CreatedAt: now, ExpiresAt: exp, IP: from.IP, UserAgent: ua}); err != nil {
		return Session{}, err
	}
	return Session{Token: token, ExpiresAt: exp, User: user}, nil
}

// Cleanup deletes expired sessions and reset tokens — the job the module schedules.
func (s *Service) Cleanup(ctx context.Context) error {
	now := s.now().UTC()
	if err := s.sessions.DeleteExpired(ctx, now); err != nil {
		return err
	}
	return s.resets.DeleteOld(ctx, now)
}

// Resolve returns the active user behind a token. A session ends before its token expires
// when the user signs out, is deactivated or deleted, or their password changes.
func (s *Service) Resolve(ctx context.Context, token string) (ports.User, error) {
	uid, sid, err := s.tokens.Verify(token)
	if err != nil {
		return ports.User{}, errSession
	}
	rec, err := s.sessions.Get(ctx, sid)
	if errors.Is(err, ErrNoSession) {
		return ports.User{}, errSession
	}
	if err != nil {
		return ports.User{}, err
	}
	if rec.Revoked || rec.UserID != uid || !rec.ExpiresAt.After(s.now()) {
		return ports.User{}, errSession
	}
	u, err := s.users.FindUser(ctx, uid)
	if errors.Is(err, ports.ErrUserNotFound) || (err == nil && !u.IsActive) {
		return ports.User{}, errSession
	}
	if err != nil {
		return ports.User{}, err
	}
	// Opened at or before the last password change: ended. (Microsecond timestamps on both
	// sides, so a change and a sign-in in the same second are still ordered.)
	if !u.PasswordChangedAt.IsZero() && !rec.CreatedAt.After(u.PasswordChangedAt) {
		_ = s.sessions.Revoke(ctx, sid, s.now().UTC())
		return ports.User{}, errSession
	}
	return u, nil
}

// Logout ends the token's session. An invalid or already ended token is not an error: the
// caller is signed out either way.
func (s *Service) Logout(ctx context.Context, token string) error {
	uid, sid, err := s.tokens.Verify(token)
	if err != nil {
		return nil
	}
	if err := s.sessions.Revoke(ctx, sid, s.now().UTC()); err != nil {
		return err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "auth", Action: "auth.signed_out", Actor: uid, Target: userTarget(uid), Summary: "Keluar"})
	return nil
}

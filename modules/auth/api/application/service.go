// Package application holds the auth module's use cases: signing in, resolving a session, and
// ending one.
package application

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

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

// Service signs users in and resolves sessions.
type Service struct {
	users    ports.UserStore
	sessions Sessions
	tokens   *Tokens
	limiter  *Limiter
	now      func() time.Time
}

func NewService(users ports.UserStore, sessions Sessions, tokens *Tokens, limiter *Limiter) *Service {
	return &Service{users: users, sessions: sessions, tokens: tokens, limiter: limiter, now: time.Now}
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
	personKey, ipKey, emailKey := "ep:"+email+"|"+from.IP, "ip:"+from.IP, "e:"+email
	if !s.limiter.Reserve(Rule{personKey, maxPerEmailAndIP}, Rule{ipKey, maxPerIP}, Rule{emailKey, maxPerEmail}) {
		return Session{}, errTooMany
	}

	cred, err := s.users.FindCredentials(ctx, email)
	if errors.Is(err, ports.ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return Session{}, errInvalid
	}
	if err != nil {
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(password)) != nil {
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
	now := s.now().UTC()
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
	// Housekeeping without a scheduler: each sign-in clears a batch of expired rows.
	_ = s.sessions.DeleteExpired(ctx, now)
	return Session{Token: token, ExpiresAt: exp, User: user}, nil
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
	_, sid, err := s.tokens.Verify(token)
	if err != nil {
		return nil
	}
	return s.sessions.Revoke(ctx, sid, s.now().UTC())
}

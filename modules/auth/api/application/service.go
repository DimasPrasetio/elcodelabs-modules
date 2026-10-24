// Package application holds the auth module's use cases: signing in and resolving a session.
package application

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"__GO_MODULE__/internal/modules/auth/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// errInvalid is the one answer to every failed sign-in, so it never tells which part was wrong.
var errInvalid = apperror.Unauthorized("Invalid email or password")

// dummyHash is compared against when the email is unknown, so an unknown email takes as long
// as a wrong password and response time does not reveal which accounts exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)

// Session is a signed-in user and their token.
type Session struct {
	Token     string
	ExpiresAt time.Time
	User      ports.User
}

// Service signs users in and resolves sessions.
type Service struct {
	users   ports.UserStore
	tokens  *Tokens
	limiter *Limiter
}

func NewService(users ports.UserStore, tokens *Tokens, limiter *Limiter) *Service {
	return &Service{users: users, tokens: tokens, limiter: limiter}
}

// Login checks email and password and opens a session.
func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var v validator.Validator
	v.Required("email", email)
	v.Required("password", password)
	if err := v.Err(); err != nil {
		return Session{}, err
	}
	if !s.limiter.Allow(email) {
		return Session{}, apperror.New(http.StatusTooManyRequests, "Too many failed sign-in attempts. Try again later.")
	}

	cred, err := s.users.FindCredentials(ctx, email)
	if errors.Is(err, ports.ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		s.limiter.Fail(email)
		return Session{}, errInvalid
	}
	if err != nil {
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(password)) != nil {
		s.limiter.Fail(email)
		return Session{}, errInvalid
	}
	if !cred.IsActive {
		return Session{}, apperror.Forbidden("This account is deactivated")
	}
	s.limiter.Reset(email)

	user, err := s.users.FindUser(ctx, cred.UserID)
	if err != nil {
		return Session{}, err
	}
	token, exp, err := s.tokens.Issue(user.ID)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, ExpiresAt: exp, User: user}, nil
}

// Resolve returns the active user behind a token. A deactivated or deleted user loses access
// on their next request, not when the token expires.
func (s *Service) Resolve(ctx context.Context, token string) (ports.User, error) {
	id, err := s.tokens.Verify(token)
	if err != nil {
		return ports.User{}, apperror.Unauthorized("Session expired or invalid")
	}
	u, err := s.users.FindUser(ctx, id)
	if errors.Is(err, ports.ErrUserNotFound) || (err == nil && !u.IsActive) {
		return ports.User{}, apperror.Unauthorized("Session expired or invalid")
	}
	return u, err
}

// Limiter counts failed sign-ins per email: after Max failures within Window, further
// attempts are refused until the window passes. In memory — one container per app.
type Limiter struct {
	Max    int
	Window time.Duration
	now    func() time.Time

	mu       sync.Mutex
	failures map[string][]time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{Max: max, Window: window, now: time.Now, failures: map[string][]time.Time{}}
}

func (l *Limiter) recent(key string) []time.Time {
	cutoff := l.now().Add(-l.Window)
	kept := l.failures[key][:0]
	for _, t := range l.failures[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = kept
	return kept
}

// Allow reports whether key may try again.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) < l.Max
}

// Fail records a failed attempt.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[key] = append(l.recent(key), l.now())
}

// Reset forgets key's failures after a successful sign-in.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

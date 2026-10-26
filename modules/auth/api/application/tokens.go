package application

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Tokens issues and verifies the session token: an HS256 JWT naming the user (sub), the
// session row (jti), and this app (aud) — so a token from another app sharing the secret by
// mistake is refused.
type Tokens struct {
	secret   []byte
	ttl      time.Duration
	audience string
	now      func() time.Time
}

func NewTokens(secret string, ttl time.Duration, audience string) *Tokens {
	return &Tokens{secret: []byte(secret), ttl: ttl, audience: audience, now: time.Now}
}

// TTL is how long an issued token stays valid.
func (t *Tokens) TTL() time.Duration { return t.ttl }

// Issue returns a token for userID's session sessionID, and when it expires.
func (t *Tokens) Issue(userID int64, sessionID string, now time.Time) (string, time.Time, error) {
	exp := now.Add(t.ttl)
	claims := jwt.RegisteredClaims{
		ID:        sessionID,
		Subject:   strconv.FormatInt(userID, 10),
		Audience:  jwt.ClaimStrings{t.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	return s, exp, err
}

// Verify returns the user ID and session ID in a valid, unexpired token for this app.
func (t *Tokens) Verify(token string) (int64, string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithAudience(t.audience),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return 0, "", err
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 || claims.ID == "" {
		return 0, "", errors.New("token has no valid subject or session")
	}
	return id, claims.ID, nil
}

// newSessionID is 128 random bits, hex-encoded.
func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/auth/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// ResetTTL is how long a "forgot password" link works.
const ResetTTL = 30 * time.Minute

// Limits per window for the password flows: they cost a bcrypt or an email each.
const (
	maxChangesPerUser = 5
	maxResetsPerIP    = 10
	maxResetsPerEmail = 3
	maxResetUsesPerIP = 20
	minPasswordLength = 8
	maxPasswordBytes  = 72
)

// ErrNoReset means no reset request has that token.
var ErrNoReset = errors.New("reset not found")

var (
	errUnavailable  = apperror.New(http.StatusNotImplemented, "Fitur ini belum tersedia di aplikasi ini")
	errResetInvalid = apperror.BadRequest("Tautan atur ulang kata sandi tidak valid atau sudah kedaluwarsa. Minta tautan baru.")
)

// ResetRecord is one "forgot password" request: for this user, sent to Email at CreatedAt.
type ResetRecord struct {
	ID        int64
	UserID    int64
	Email     string
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
}

// Resets stores reset requests — only a hash of each token.
type Resets interface {
	Create(ctx context.Context, rec ResetRecord, tokenHash, ip string) error
	// Find returns the request with that token hash, or ErrNoReset.
	Find(ctx context.Context, tokenHash string) (ResetRecord, error)
	// Use marks the request used; false when it already was (a second use loses the race).
	Use(ctx context.Context, id int64, at time.Time) (bool, error)
	// Release undoes Use(id, usedAt) when the new password could not be saved.
	Release(ctx context.Context, id int64, usedAt time.Time) error
	// VoidUser marks every unused request of the user used.
	VoidUser(ctx context.Context, userID int64, at time.Time) error
	DeleteOld(ctx context.Context, before time.Time) error
}

// Options are the password features this install offers — the sign-in page asks.
type Options struct {
	PasswordChange bool `json:"password_change"`
	PasswordReset  bool `json:"password_reset"`
}

func (s *Service) Options() Options {
	return Options{PasswordChange: s.passwords != nil, PasswordReset: s.passwords != nil && s.mailer != nil}
}

// checkNewPassword applies the length rules here too, so a reset token is not spent on a
// password the store would refuse.
func checkNewPassword(v *validator.Validator, field, password string) {
	v.Check(utf8.RuneCountInString(password) >= minPasswordLength, field, "minimal 8 karakter")
	v.Check(len(password) <= maxPasswordBytes, field, "maksimal 72 byte")
}

// setPassword hands the new password to the store; a refusal becomes a field error.
func (s *Service) setPassword(ctx context.Context, userID int64, field, password string) error {
	err := s.passwords.SetPassword(ctx, userID, password)
	if errors.Is(err, ports.ErrWeakPassword) {
		return apperror.Validation([]validator.FieldError{{Field: field, Message: strings.TrimPrefix(err.Error(), ports.ErrWeakPassword.Error()+": ")}})
	}
	return err
}

// ChangePassword replaces the signed-in user's password after checking the current one. Every
// other session ends (the store records the change); this device gets a fresh session, so the
// person stays signed in here.
func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next string, from Client) (Session, error) {
	if s.passwords == nil {
		return Session{}, errUnavailable
	}
	var v validator.Validator
	v.Required("current_password", current)
	checkNewPassword(&v, "new_password", next)
	v.Check(next == "" || next != current, "new_password", "harus berbeda dari kata sandi saat ini")
	if err := v.Err(); err != nil {
		return Session{}, err
	}
	key := fmt.Sprintf("cp:%d", userID)
	if !s.flows.Reserve(Rule{key, maxChangesPerUser}) {
		return Session{}, errTooMany
	}
	user, err := s.users.FindUser(ctx, userID)
	if err != nil {
		return Session{}, err
	}
	cred, err := s.users.FindCredentials(ctx, user.Email)
	if err != nil {
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(current)) != nil {
		return Session{}, apperror.Validation([]validator.FieldError{{Field: "current_password", Message: "salah"}})
	}
	if err := s.setPassword(ctx, userID, "new_password", next); err != nil {
		return Session{}, err
	}
	s.flows.Forget(key)
	// Read the change time back as the store recorded it: this device's new session starts after
	// it (valid), every older one before it (ended).
	if user, err = s.users.FindUser(ctx, userID); err != nil {
		return Session{}, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if changed := user.PasswordChangedAt.UTC(); !now.After(changed) {
		now = changed.Truncate(time.Microsecond).Add(time.Microsecond)
	}
	session, err := s.open(ctx, user, from, now)
	if err != nil {
		return Session{}, err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "auth", Action: "auth.password_changed", Target: userTarget(userID), Summary: "Mengganti kata sandi sendiri"})
	return session, nil
}

// RequestReset emails a reset link to email if it belongs to an active account. It answers the
// same either way — and also when the limits are reached or the email cannot be queued — so it
// never tells which addresses have accounts. The lookup and the email happen after the answer,
// so a known address does not take longer to answer than an unknown one either.
func (s *Service) RequestReset(ctx context.Context, email string, from Client) error {
	if s.passwords == nil || s.mailer == nil {
		return errUnavailable
	}
	email = strings.ToLower(strings.TrimSpace(email))
	var v validator.Validator
	v.Required("email", email)
	v.MaxLen("email", email, 254)
	if err := v.Err(); err != nil {
		return err
	}
	if !s.flows.Reserve(Rule{"fp-ip:" + ClientKey(from.IP), maxResetsPerIP}, Rule{"fp-e:" + email, maxResetsPerEmail}) {
		return nil
	}
	// Detached from the request (which ends with the answer) but keeping its values, so the
	// audit record still has the address and request ID.
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	s.later(func() {
		defer cancel()
		if err := s.sendReset(bg, email, from); err != nil {
			s.log.ErrorContext(bg, "auth: reset email not queued", "error", err)
		}
	})
	return nil
}

// sendReset creates the reset request and queues its email, when email is an active account.
func (s *Service) sendReset(ctx context.Context, email string, from Client) error {
	cred, err := s.users.FindCredentials(ctx, email)
	if errors.Is(err, ports.ErrUserNotFound) || (err == nil && !cred.IsActive) {
		return nil
	}
	if err != nil {
		return err
	}
	user, err := s.users.FindUser(ctx, cred.UserID)
	if err != nil {
		return err
	}
	token, hash, err := newResetToken()
	if err != nil {
		return err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if err := s.resets.VoidUser(ctx, user.ID, now); err != nil {
		return err
	}
	rec := ResetRecord{UserID: user.ID, Email: strings.ToLower(user.Email), CreatedAt: now, ExpiresAt: now.Add(ResetTTL)}
	if err := s.resets.Create(ctx, rec, hash, from.IP); err != nil {
		return err
	}
	// The token travels in the fragment: browsers never send it to a server, so it stays out of
	// access logs and Referer headers.
	link := s.appURL + "/reset-password#token=" + token
	subject, text, body := resetEmail(s.appName, user.Name, link)
	if err := s.mailer.Send(ctx, user.Email, user.Name, subject, text, body); err != nil {
		return err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "auth", Action: "auth.password_reset_requested", Actor: user.ID, Target: userTarget(user.ID), Summary: "Meminta tautan atur ulang kata sandi"})
	return nil
}

// ResetPassword sets a new password with a token from the email. The token works once, and only
// while the account still has the address it was sent to and the password it had then; every
// session of the account ends, and the person signs in with the new password.
func (s *Service) ResetPassword(ctx context.Context, token, next string, from Client) error {
	if s.passwords == nil {
		return errUnavailable
	}
	var v validator.Validator
	v.Required("token", token)
	checkNewPassword(&v, "new_password", next)
	if err := v.Err(); err != nil {
		return err
	}
	if !s.flows.Reserve(Rule{"rp-ip:" + ClientKey(from.IP), maxResetUsesPerIP}) {
		return errTooMany
	}
	rec, err := s.resets.Find(ctx, hashToken(token))
	if errors.Is(err, ErrNoReset) {
		return errResetInvalid
	}
	if err != nil {
		return err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if rec.Used || !rec.ExpiresAt.After(now) {
		return errResetInvalid
	}
	user, err := s.users.FindUser(ctx, rec.UserID)
	if errors.Is(err, ports.ErrUserNotFound) {
		return errResetInvalid
	}
	if err != nil {
		return err
	}
	if !user.IsActive || !strings.EqualFold(user.Email, rec.Email) || !rec.CreatedAt.After(user.PasswordChangedAt) {
		return errResetInvalid
	}
	used, err := s.resets.Use(ctx, rec.ID, now)
	if err != nil {
		return err
	}
	if !used {
		return errResetInvalid
	}
	if err := s.setPassword(ctx, rec.UserID, "new_password", next); err != nil {
		// Not saved: the link keeps working for another try.
		_ = s.resets.Release(context.WithoutCancel(ctx), rec.ID, now)
		return err
	}
	_ = s.resets.VoidUser(ctx, rec.UserID, now)
	s.audit.Record(ctx, kernel.AuditEntry{Module: "auth", Action: "auth.password_reset", Actor: rec.UserID, Target: userTarget(rec.UserID), Summary: "Mengatur ulang kata sandi lewat email"})
	return nil
}

// newResetToken returns a 256-bit token for the link and the hash that is stored.
func newResetToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// resetEmail is the message with the link, in plain text and HTML.
func resetEmail(appName, name, link string) (subject, text, body string) {
	subject = "Atur ulang kata sandi — " + appName
	text = fmt.Sprintf("Halo %s,\n\nAda permintaan untuk mengatur ulang kata sandi akun %s Anda. Buka tautan ini dalam 30 menit:\n\n%s\n\nJika Anda tidak memintanya, abaikan email ini — kata sandi Anda tidak berubah.\n", name, appName, link)
	e := html.EscapeString
	body = fmt.Sprintf(`<p>Halo %s,</p>
<p>Ada permintaan untuk mengatur ulang kata sandi akun %s Anda. Tautan ini berlaku 30 menit.</p>
<p style="margin:24px 0"><a href="%s" style="display:inline-block;background:#1f2a44;color:#ffffff;text-decoration:none;padding:10px 18px;border-radius:6px;font-weight:600">Atur ulang kata sandi</a></p>
<p style="font-size:13px;color:#6b7280">Jika tombol tidak berfungsi, salin tautan ini ke browser:<br><span style="word-break:break-all">%s</span></p>
<p style="font-size:13px;color:#6b7280">Jika Anda tidak memintanya, abaikan email ini — kata sandi Anda tidak berubah.</p>`, e(name), e(appName), e(link), e(link))
	return subject, text, body
}

// Package docstate is the life of a business document, shared by every module that posts one:
//
//	draft ──post──▶ posted ──cancel (reason)──▶ cancelled ──amend──▶ a new draft
//	  └──edit, delete
//
// A posted document is never changed: it is cancelled, and its correction is a new draft that
// points at it (AmendedFrom) and carries its number with a suffix (INV/2026/0007-1). The status
// lives in the document's own table; this kit only says which moves are allowed. Its errors are
// plain errors — the module maps ErrTransition and ErrStale to 409 and ErrReason to 422.
package docstate

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Status is where a document is in its life; it is stored as is in a VARCHAR(10) column.
type Status string

const (
	Draft     Status = "draft"
	Posted    Status = "posted"
	Cancelled Status = "cancelled"
)

// Valid reports whether s is one of the three statuses.
func (s Status) Valid() bool { return s == Draft || s == Posted || s == Cancelled }

// Action is what someone wants to do to a document.
type Action string

const (
	Edit   Action = "edit"   // change a draft
	Post   Action = "post"   // make it final: numbered, in the books
	Delete Action = "delete" // remove a draft nothing points at
	Cancel Action = "cancel" // void a posted document, with a reason
	Amend  Action = "amend"  // start the correction of a cancelled document, as a new draft
)

var (
	// ErrTransition: the action is not allowed in the document's status. errors.Is matches every
	// *TransitionError; its message says what to do instead.
	ErrTransition = errors.New("Aksi ini tidak berlaku untuk status dokumen")
	// ErrStale: the guarded UPDATE changed no row — someone else moved the document first.
	ErrStale = errors.New("Dokumen sudah diubah orang lain — muat ulang")
	// ErrReason: a cancellation without a usable reason.
	ErrReason = errors.New("Alasan pembatalan wajib diisi, 5 sampai 500 karakter")
)

// TransitionError is a refused action, with a message for the person who tried it.
type TransitionError struct {
	From   Status
	Action Action
	msg    string
}

func (e *TransitionError) Error() string        { return e.msg }
func (e *TransitionError) Is(target error) bool { return target == ErrTransition }

// next is the whole table: status → action → the status it leads to. Amend leaves the cancelled
// document as it is; the draft it starts is a new document.
var next = map[Status]map[Action]Status{
	Draft:     {Edit: Draft, Post: Posted, Delete: ""},
	Posted:    {Cancel: Cancelled},
	Cancelled: {Amend: Cancelled},
}

// Next is the status a document in from reaches through action ("" for Delete: it is gone), or a
// *TransitionError.
func Next(from Status, action Action) (Status, error) {
	to, ok := next[from][action]
	if !ok {
		return from, &TransitionError{From: from, Action: action, msg: refusal(from, action)}
	}
	return to, nil
}

// Check is Next without the destination: nil when action is allowed in from.
func Check(from Status, action Action) error {
	_, err := Next(from, action)
	return err
}

func refusal(from Status, action Action) string {
	switch {
	case !from.Valid():
		return "Status dokumen tidak dikenal"
	case from == Posted && action == Amend:
		return "Batalkan dokumen ini dulu, lalu buat revisinya"
	case from == Posted:
		return "Dokumen yang sudah diposting tidak dapat diubah; batalkan lalu buat revisinya"
	case from == Cancelled:
		return "Dokumen yang sudah dibatalkan tidak dapat diubah; buat revisinya bila perlu"
	case action == Cancel:
		return "Dokumen draft belum diposting — ubah atau hapus saja"
	case action == Amend:
		return "Hanya dokumen yang dibatalkan yang bisa direvisi"
	default:
		return "Aksi ini tidak berlaku untuk dokumen draft"
	}
}

// CancelReason is the reason trimmed, or ErrReason when it is shorter than 5 or longer than 500
// characters.
func CancelReason(reason string) (string, error) {
	r := strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(r); n < 5 || n > 500 {
		return "", ErrReason
	}
	return r, nil
}

// Guard turns the rows a guarded UPDATE changed into ErrStale when it changed none:
//
//	UPDATE inv_invoices SET status = 'posted', … WHERE id = ? AND company_id = ? AND status = 'draft'
func Guard(rowsAffected int64) error {
	if rowsAffected == 0 {
		return ErrStale
	}
	return nil
}

// Fields are the lifecycle columns a document embeds in its domain type. The columns:
//
//	status        VARCHAR(10) NOT NULL DEFAULT 'draft',
//	posted_at     DATETIME(6) NULL,  posted_by    BIGINT NULL,
//	cancelled_at  DATETIME(6) NULL,  cancelled_by BIGINT NULL,  cancel_reason VARCHAR(500) NULL,
//	amended_from  BIGINT NULL,       revision     INT NOT NULL DEFAULT 0,
//	UNIQUE KEY (company_id, amended_from)   -- a cancelled document is amended once
type Fields struct {
	Status       Status
	PostedAt     time.Time // zero until posted
	PostedBy     int64
	CancelledAt  time.Time // zero unless cancelled
	CancelledBy  int64
	CancelReason string
	AmendedFrom  int64 // the cancelled document this one corrects; 0 for an original
	Revision     int   // 0 for an original, 1, 2, … for its corrections
}

// AmendedNumber is base with the revision suffix: AmendedNumber("INV/2026/0007", 1) is
// "INV/2026/0007-1". Revision 0 is base itself.
func AmendedNumber(base string, revision int) string {
	if revision <= 0 {
		return base
	}
	return base + "-" + strconv.Itoa(revision)
}

// Amendment is the number and revision of the draft that corrects a cancelled document numbered
// number at revision: ("INV/0007", 0) → ("INV/0007-1", 1), ("INV/0007-1", 1) → ("INV/0007-2", 2)
// — always one suffix on the original number.
func Amendment(number string, revision int) (string, int) {
	base := number
	if revision > 0 {
		base = strings.TrimSuffix(number, "-"+strconv.Itoa(revision))
	}
	return AmendedNumber(base, revision+1), revision + 1
}

package docstate

import (
	"errors"
	"strings"
	"testing"
)

func TestTransitions(t *testing.T) {
	all := []Action{Edit, Post, Delete, Cancel, Amend}
	allowed := map[Status]map[Action]Status{
		Draft:     {Edit: Draft, Post: Posted, Delete: ""},
		Posted:    {Cancel: Cancelled},
		Cancelled: {Amend: Cancelled},
	}
	for _, from := range []Status{Draft, Posted, Cancelled, "", "void"} {
		for _, a := range all {
			to, err := Next(from, a)
			want, ok := allowed[from][a]
			switch {
			case ok && (err != nil || to != want):
				t.Errorf("%s --%s--> %q, %v; want %q", from, a, to, err, want)
			case !ok && !errors.Is(err, ErrTransition):
				t.Errorf("%s --%s--> %q, %v; want ErrTransition", from, a, to, err)
			case !ok && to != from:
				t.Errorf("refused %s --%s-- moved to %q", from, a, to)
			}
			if (Check(from, a) == nil) != ok {
				t.Errorf("Check(%s, %s) disagrees with Next", from, a)
			}
		}
	}
	if _, err := Next(Posted, Edit); err.Error() != "Dokumen yang sudah diposting tidak dapat diubah; batalkan lalu buat revisinya" {
		t.Errorf("posted edit: %q", err)
	}
	var te *TransitionError
	if _, err := Next(Cancelled, Post); !errors.As(err, &te) || te.From != Cancelled || te.Action != Post {
		t.Errorf("TransitionError = %#v", err)
	}
	if errors.Is(ErrStale, ErrTransition) || errors.Is(ErrReason, ErrTransition) {
		t.Error("ErrStale/ErrReason match ErrTransition")
	}
}

func TestCancelReason(t *testing.T) {
	for in, ok := range map[string]bool{"": false, "    ": false, "salah": true, " salah ": true, "abcd": false, "ééééé": true, strings.Repeat("x", 500): true, strings.Repeat("x", 501): false} {
		r, err := CancelReason(in)
		if ok != (err == nil) || (ok && r != strings.TrimSpace(in)) || (!ok && !errors.Is(err, ErrReason)) {
			t.Errorf("CancelReason(%q) = %q, %v", in, r, err)
		}
	}
}

func TestGuard(t *testing.T) {
	if !errors.Is(Guard(0), ErrStale) || Guard(1) != nil {
		t.Error("Guard")
	}
}

func TestAmendment(t *testing.T) {
	if got := AmendedNumber("INV/2026/10/0007", 1); got != "INV/2026/10/0007-1" {
		t.Errorf("AmendedNumber = %s", got)
	}
	if got := AmendedNumber("INV-7", 0); got != "INV-7" {
		t.Errorf("AmendedNumber rev 0 = %s", got)
	}
	cases := []struct {
		number string
		rev    int
		want   string
		next   int
	}{
		{"INV/0007", 0, "INV/0007-1", 1},
		{"INV/0007-1", 1, "INV/0007-2", 2},
		{"INV/0007-9", 9, "INV/0007-10", 10},
		{"INV-2026-0001", 0, "INV-2026-0001-1", 1}, // a dash in the pattern itself is kept
		{"INV-2026-0001-1", 1, "INV-2026-0001-2", 2},
	}
	for _, c := range cases {
		got, rev := Amendment(c.number, c.rev)
		if got != c.want || rev != c.next {
			t.Errorf("Amendment(%s, %d) = %s, %d; want %s, %d", c.number, c.rev, got, rev, c.want, c.next)
		}
	}
}

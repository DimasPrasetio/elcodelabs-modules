package domain

import (
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
)

var oct27 = kernel.Date{Year: 2026, Month: time.October, Day: 27}

func TestValidate(t *testing.T) {
	ok := []Format{
		{"INV/{YYYY}/{MM}/{SEQ:4}", Monthly, false},
		{"INV/{BRANCH}/{YY}{MM}/{SEQ:4}", Monthly, true},
		{"{BRANCH}/{SEQ:5}", Never, true},
		{"SO{YYYY}{SEQ:6}", Yearly, false},
		{"{SEQ:3}/{COMPANY}/{YYYY}", Yearly, false},
		{"{SEQ:3}/SJ/{MM}/{YYYY}", Yearly, false},
		{"INV-{YYYY}{SEQ:4}", Yearly, false},
		{"INV/{YYYY}-{SEQ:4}", Yearly, false},
		{"{SEQ:4}/X-1", Never, false},
		{"KW-{YY}{MM}{DD}-{SEQ:3}", Daily, false},
		{"{SEQ:1}", Never, false},
	}
	for _, f := range ok {
		if err := f.Validate(); err != nil {
			t.Errorf("%+v: %v", f, err)
		}
	}
	bad := map[string]Format{
		"no SEQ":               {"INV/{YYYY}", Yearly, false},
		"two SEQ":              {"{SEQ:2}-{SEQ:3}", Never, false},
		"yearly without year":  {"INV/{MM}/{SEQ:4}", Yearly, false},
		"monthly without MM":   {"INV/{YYYY}/{SEQ:4}", Monthly, false},
		"daily without DD":     {"INV/{YYYY}{MM}/{SEQ:4}", Daily, false},
		"per branch, no code":  {"INV/{YYYY}/{SEQ:4}", Yearly, true},
		"unknown token":        {"INV/{YEAR}/{SEQ:4}", Never, false},
		"unclosed":             {"INV/{SEQ:4", Never, false},
		"stray brace":          {"INV}/{SEQ:4}", Never, false},
		"SEQ width 0":          {"{SEQ:0}", Never, false},
		"SEQ width 13":         {"{SEQ:13}", Never, false},
		"bad reset":            {"{SEQ:4}", "weekly", false},
		"code glued after":     {"{BRANCH}1{SEQ:3}", Never, true},
		"code glued before":    {"X{BRANCH}/{SEQ:3}", Never, true},
		"code next to SEQ":     {"{SEQ:3}{BRANCH}", Never, true},
		"empty":                {"", Never, false},
		"too long":             {strings.Repeat("A", 60) + "{SEQ:1}", Never, false},
		"non-ASCII":            {"FAKTUR–{SEQ:3}", Never, false},
		"a dash after a code":  {"INV-{BRANCH}-{SEQ:1}", Never, true},
		"a dash before a code": {"INV-{BRANCH}/{SEQ:1}", Never, true},
		"ends with a code":     {"{SEQ:4}/{BRANCH}", Never, true},
		"renders too long":     {"{BRANCH}/{BRANCH}/{BRANCH}/{BRANCH}/{BRANCH}/{SEQ:12}", Never, true},
	}
	for name, f := range bad {
		if err := f.Validate(); err == nil {
			t.Errorf("%s (%q) accepted", name, f.Pattern)
		}
	}
}

func TestRenderAndPeriodKey(t *testing.T) {
	f := Format{"INV/{BRANCH}/{YYYY}/{MM}/{DD}/{SEQ:4}", Daily, true}
	if got := f.Render(oct27, 7, Codes{Branch: "BDG"}); got != "INV/BDG/2026/10/27/0007" {
		t.Errorf("Render = %s", got)
	}
	if got := (Format{"{COMPANY}-{YY}{SEQ:2}", Yearly, false}).Render(oct27, 12345, Codes{Company: "PT1"}); got != "PT1-2612345" {
		t.Errorf("SEQ past its padding = %s", got)
	}
	keys := map[Reset]string{Never: "", Yearly: "2026", Monthly: "2026-10", Daily: "2026-10-27"}
	for r, want := range keys {
		if got := PeriodKey(r, oct27); got != want {
			t.Errorf("PeriodKey(%s) = %q", r, got)
		}
	}
}

package taxid

import (
	"errors"
	"testing"
)

func TestNormalizeNPWP(t *testing.T) {
	for in, want := range map[string]string{
		"01.234.567.8-901.000": "0012345678901000", // the old 15 digits get a leading 0
		"012345678901000":      "0012345678901000",
		"3201234567890001":     "3201234567890001",
		" 0012 3456 7890 1000": "0012345678901000",
	} {
		if got, err := NormalizeNPWP(in); err != nil || got != want {
			t.Errorf("NormalizeNPWP(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "123", "01234567890100", "00123456789010001", "0123456789O1000", "01.234.567.8-901.00a"} {
		if _, err := NormalizeNPWP(in); !errors.Is(err, ErrFormat) {
			t.Errorf("NormalizeNPWP(%q) = %v; want ErrFormat", in, err)
		}
	}
}

func TestCheckNIK(t *testing.T) {
	for _, in := range []string{
		"3201011508900001", // 15 Aug 1990
		"3201015508900001", // a woman: day 15 + 40
		"3201012902000001", // 29 Feb 2000
		"3201-0115-0890-0001",
	} {
		if _, err := CheckNIK(in); err != nil {
			t.Errorf("CheckNIK(%q) = %v", in, err)
		}
	}
	for _, in := range []string{
		"320101150890000",  // 15 digits
		"0001011508900001", // no province
		"3201011508900000", // serial 0000
		"3201013208900001", // day 32
		"3201017208900001", // a woman's day 32
		"3201011513900001", // month 13
		"3201013102900001", // 31 Feb
		"3201011508900O01",
	} {
		if _, err := CheckNIK(in); !errors.Is(err, ErrFormat) {
			t.Errorf("CheckNIK(%q) = %v; want ErrFormat", in, err)
		}
	}
}

func TestCheckNITKU(t *testing.T) {
	if got, err := CheckNITKU("0012345678901000000001", "0012345678901000"); err != nil || got != "0012345678901000000001" {
		t.Errorf("a branch of the NPWP = %q, %v", got, err)
	}
	if _, err := CheckNITKU("0012345678901000000001", ""); err != nil {
		t.Errorf("without an NPWP to compare: %v", err)
	}
	for _, c := range [][2]string{
		{"001234567890100000000", ""},                  // 21 digits
		{"0099999999999999000000", "0012345678901000"}, // another NPWP's place
	} {
		if _, err := CheckNITKU(c[0], c[1]); !errors.Is(err, ErrFormat) {
			t.Errorf("CheckNITKU(%q, %q) = %v; want ErrFormat", c[0], c[1], err)
		}
	}
}

func TestCheckPassport(t *testing.T) {
	if got, err := CheckPassport(" e1234567 "); err != nil || got != "E1234567" {
		t.Errorf("CheckPassport = %q, %v", got, err)
	}
	for _, in := range []string{"A12", "A1234-567", "ABCDEFGHIJKLMNOPQRSTU"} {
		if _, err := CheckPassport(in); !errors.Is(err, ErrFormat) {
			t.Errorf("CheckPassport(%q) = %v; want ErrFormat", in, err)
		}
	}
}

func TestSuspectNPWP(t *testing.T) {
	// 01.302.442.7-…: Luhn over 013024427 holds.
	if SuspectNPWP("0013024427092000") {
		t.Error("a matching check digit is suspect")
	}
	if !SuspectNPWP("0013024420092000") {
		t.Error("a wrong check digit is not suspect")
	}
	if SuspectNPWP("3201011508900001") {
		t.Error("an NPWP that is a NIK has no check digit")
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{
		"3201011508900001": "3201********0001",
		"1234567890":       "1234**7890",
		"E1234567":         "******67",
		"12":               "**",
		"":                 "",
	} {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestKindValid(t *testing.T) {
	for _, k := range []Kind{NPWP, NIK, Passport, None} {
		if !k.Valid() {
			t.Errorf("%q not valid", k)
		}
	}
	if Kind("ktp").Valid() {
		t.Error("an unknown kind is valid")
	}
}

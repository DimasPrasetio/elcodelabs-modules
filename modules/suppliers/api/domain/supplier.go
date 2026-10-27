// Package domain holds the suppliers module's types and rules: what a supplier, its addresses,
// bank accounts, and contacts must be to be saved. No I/O.
package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"__GO_MODULE__/internal/shared/taxid"
	"__GO_MODULE__/internal/shared/validator"
)

// PartyType is what kind of party a supplier is — it decides which tax IDs fit.
type PartyType string

const (
	Company    PartyType = "company"
	Individual PartyType = "individual"
	Government PartyType = "government"
)

// Supplier is a party the company buys from.
type Supplier struct {
	ID          int64
	CompanyID   int64
	Code        string
	Name        string
	LegalName   string // the name on tax invoices; "" uses Name
	PartyType   PartyType
	IsForeign   bool
	Country     string // ISO 3166-1 alpha-2
	TaxIDKind   taxid.Kind
	TaxID       string // NPWP16 or NIK digits, a passport number in capitals, or ""
	IsPKP       bool
	Email       string
	Phone       string
	PaymentDays *int // nil: cash on delivery
	OnHold      bool
	HoldReason  string
	Notes       string
	IsActive    bool
	CreatedBy   int64
	CreatedAt   time.Time
	UpdatedBy   int64
	UpdatedAt   time.Time
}

// InvoiceName is the name a tax invoice prints.
func (c Supplier) InvoiceName() string {
	if c.LegalName != "" {
		return c.LegalName
	}
	return c.Name
}

// MaxPaymentDays bounds payment terms: a year.
const MaxPaymentDays = 365

var (
	codeRe    = regexp.MustCompile(`^[A-Z0-9][A-Z0-9./-]{0,29}$`)
	prefixRe  = regexp.MustCompile(`^[A-Z0-9./-]{0,10}$`)
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
	phoneRe   = regexp.MustCompile(`^[0-9+() .-]{0,30}$`)
)

// NormalizeCode is a code as stored: trimmed, in capitals.
func NormalizeCode(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// ValidCode reports whether a typed code may be stored.
func ValidCode(s string) bool { return codeRe.MatchString(s) }

// GeneratedCode is the code taken from a company's counter: prefix and n in five digits.
func GeneratedCode(prefix string, n int64) string { return fmt.Sprintf("%s%05d", prefix, n) }

// ValidPrefix reports whether a code prefix may be stored.
func ValidPrefix(s string) bool { return prefixRe.MatchString(s) }

// Clean trims and normalizes c, then records every field error in v. The tax ID is stored
// in its canonical form: a 15-digit NPWP becomes 16 digits, punctuation goes. It returns the
// warnings to show beside a valid supplier (a suspect NPWP check digit).
func (c *Supplier) Clean(v *validator.Validator) (warnings []string) {
	for _, f := range []*string{&c.Name, &c.LegalName, &c.Email, &c.Phone, &c.Notes, &c.TaxID, &c.Country} {
		*f = strings.TrimSpace(*f)
	}
	c.Email = strings.ToLower(c.Email)
	c.Country = strings.ToUpper(c.Country)
	if c.Country == "" {
		c.Country = "ID"
	}
	if c.TaxIDKind == "" {
		c.TaxIDKind = taxid.None
	}

	v.Required("name", c.Name)
	v.MaxLen("name", c.Name, 150)
	v.MaxLen("legal_name", c.LegalName, 200)
	v.Check(c.PartyType == Company || c.PartyType == Individual || c.PartyType == Government, "party_type", "pilih badan usaha, perorangan, atau instansi pemerintah")
	v.Check(countryRe.MatchString(c.Country), "country", "kode negara 2 huruf (ISO), mis. ID")
	v.Check(c.IsForeign == (c.Country != "ID"), "country", "pemasok luar negeri memakai negara selain ID, pemasok dalam negeri memakai ID")
	v.Email("email", c.Email)
	v.MaxLen("email", c.Email, 190)
	v.Check(phoneRe.MatchString(c.Phone), "phone", "hanya angka, spasi, dan + ( ) . -")
	v.MaxLen("notes", c.Notes, 2000)
	if c.PaymentDays != nil {
		v.Check(*c.PaymentDays >= 0 && *c.PaymentDays <= MaxPaymentDays, "payment_days", fmt.Sprintf("antara 0 dan %d hari", MaxPaymentDays))
	}
	return c.prepareTaxID(v)
}

func (c *Supplier) prepareTaxID(v *validator.Validator) (warnings []string) {
	switch c.TaxIDKind {
	case taxid.NPWP:
		n, err := taxid.NormalizeNPWP(c.TaxID)
		if err != nil {
			v.Check(false, "tax_id", "NPWP harus 16 digit (NPWP lama 15 digit diubah otomatis)")
			return nil
		}
		c.TaxID = n
		if taxid.SuspectNPWP(n) {
			warnings = append(warnings, "Digit pemeriksa NPWP tidak cocok — periksa kembali nomornya")
		}
	case taxid.NIK:
		v.Check(c.PartyType == Individual && !c.IsForeign, "tax_id_kind", "NIK hanya untuk perorangan dalam negeri")
		n, err := taxid.CheckNIK(c.TaxID)
		if err != nil {
			v.Check(false, "tax_id", "NIK harus 16 digit dengan tanggal lahir yang benar")
			return nil
		}
		c.TaxID = n
	case taxid.Passport:
		v.Check(c.IsForeign, "tax_id_kind", "paspor hanya untuk pemasok luar negeri")
		n, err := taxid.CheckPassport(c.TaxID)
		if err != nil {
			v.Check(false, "tax_id", "nomor paspor 5–20 huruf dan angka")
			return nil
		}
		c.TaxID = n
	case taxid.None:
		c.TaxID = ""
		v.Check(!c.IsPKP, "is_pkp", "PKP harus punya NPWP atau NIK")
	default:
		v.Check(false, "tax_id_kind", "pilih NPWP, NIK, paspor, atau tanpa identitas")
	}
	if c.TaxIDKind == taxid.Passport {
		v.Check(!c.IsPKP, "is_pkp", "PKP harus punya NPWP atau NIK")
	}
	return warnings
}

// TaxIDForNITKU is the NPWP a NITKU of this supplier must start with, or "" when it has none.
func (c Supplier) TaxIDForNITKU() string {
	if c.TaxIDKind == taxid.NPWP || c.TaxIDKind == taxid.NIK {
		return c.TaxID
	}
	return ""
}

// Address is a billing or shipping address of a supplier.
type Address struct {
	ID                int64
	CompanyID         int64
	SupplierID        int64
	Label             string
	Line              string
	City              string
	Province          string
	PostalCode        string
	Country           string
	NITKU             string // 22 digits, or ""
	ForBilling        bool
	ForShipping       bool
	IsDefaultBilling  bool
	IsDefaultShipping bool
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// OneLine is the address as a tax invoice prints it.
func (a Address) OneLine() string {
	parts := []string{a.Line}
	for _, p := range []string{a.City, a.Province, a.PostalCode} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

// Clean trims a and records every field error in v. npwp16 is the supplier's NPWP or NIK
// ("" when it has none): a NITKU must start with it.
func (a *Address) Clean(v *validator.Validator, npwp16 string) {
	for _, f := range []*string{&a.Label, &a.Line, &a.City, &a.Province, &a.PostalCode, &a.Country, &a.NITKU} {
		*f = strings.TrimSpace(*f)
	}
	a.Country = strings.ToUpper(a.Country)
	if a.Country == "" {
		a.Country = "ID"
	}
	v.Required("label", a.Label)
	v.MaxLen("label", a.Label, 60)
	v.Required("line", a.Line)
	v.MaxLen("line", a.Line, 255)
	v.MaxLen("city", a.City, 100)
	v.MaxLen("province", a.Province, 100)
	v.MaxLen("postal_code", a.PostalCode, 10)
	v.Check(countryRe.MatchString(a.Country), "country", "kode negara 2 huruf (ISO), mis. ID")
	v.Check(a.ForBilling || a.ForShipping, "for_billing", "pakai untuk penagihan, pengiriman, atau keduanya")
	v.Check(!a.IsDefaultBilling || a.ForBilling, "is_default_billing", "alamat default tagih harus dipakai untuk penagihan")
	v.Check(!a.IsDefaultShipping || a.ForShipping, "is_default_shipping", "alamat default kirim harus dipakai untuk pengiriman")
	if a.NITKU != "" {
		switch n, err := taxid.CheckNITKU(a.NITKU, npwp16); {
		case npwp16 == "":
			v.Check(false, "nitku", "NITKU hanya untuk pemasok ber-NPWP atau NIK")
		case err != nil:
			v.Check(false, "nitku", "22 angka: NPWP pemasok 16 digit + nomor urut tempat usaha 6 digit")
		default:
			a.NITKU = n
		}
	}
}

// BankAccount is where a supplier is paid.
type BankAccount struct {
	ID            int64
	CompanyID     int64
	SupplierID    int64
	BankName      string
	BankCode      string // a 3-digit Indonesian bank code, a SWIFT/BIC, or ""
	AccountNumber string
	AccountHolder string
	IsDefault     bool
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

var (
	bankCodeRe = regexp.MustCompile(`^([0-9]{3}|[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?)?$`)
	accountRe  = regexp.MustCompile(`^[0-9A-Z-]{4,34}$`)
)

// Clean trims b and records every field error in v. An account number keeps letters and dashes
// (IBANs abroad) but loses spaces and dots.
func (b *BankAccount) Clean(v *validator.Validator) {
	for _, f := range []*string{&b.BankName, &b.BankCode, &b.AccountNumber, &b.AccountHolder} {
		*f = strings.TrimSpace(*f)
	}
	b.BankCode = strings.ToUpper(b.BankCode)
	b.AccountNumber = strings.ToUpper(strings.NewReplacer(" ", "", ".", "").Replace(b.AccountNumber))
	v.Required("bank_name", b.BankName)
	v.MaxLen("bank_name", b.BankName, 100)
	v.Check(bankCodeRe.MatchString(b.BankCode), "bank_code", "kode bank 3 angka, atau SWIFT/BIC 8 atau 11 karakter")
	v.Check(accountRe.MatchString(b.AccountNumber), "account_number", "4–34 angka (atau huruf dan - untuk IBAN)")
	v.Required("account_holder", b.AccountHolder)
	v.MaxLen("account_holder", b.AccountHolder, 150)
}

// HolderDiffers reports whether an account is held by someone other than the supplier — worth
// a warning: a changed account is the usual way a payment is diverted.
func HolderDiffers(holder string, s Supplier) bool {
	norm := func(x string) string {
		x = strings.ToUpper(x)
		for _, p := range []string{"PT.", "PT ", "CV.", "CV ", "UD.", "UD ", "TBK", ",", "."} {
			x = strings.ReplaceAll(x, p, " ")
		}
		return strings.Join(strings.Fields(x), " ")
	}
	h := norm(holder)
	return h != norm(s.Name) && (s.LegalName == "" || h != norm(s.LegalName))
}

// Contact is a person at a supplier.
type Contact struct {
	ID         int64
	CompanyID  int64
	SupplierID int64
	Name       string
	Title      string
	Email      string
	Phone      string
	IsPrimary  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Clean trims c and records every field error in v.
func (c *Contact) Clean(v *validator.Validator) {
	for _, f := range []*string{&c.Name, &c.Title, &c.Email, &c.Phone} {
		*f = strings.TrimSpace(*f)
	}
	c.Email = strings.ToLower(c.Email)
	v.Required("name", c.Name)
	v.MaxLen("name", c.Name, 150)
	v.MaxLen("title", c.Title, 100)
	v.Email("email", c.Email)
	v.MaxLen("email", c.Email, 190)
	v.Check(phoneRe.MatchString(c.Phone), "phone", "hanya angka, spasi, dan + ( ) . -")
}

// ValidReason reports whether a hold reason may be stored: 3 to 255 characters.
func ValidReason(s string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n >= 3 && n <= 255
}

package application

import (
	"context"
	"errors"
	"strconv"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/suppliers/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/taxid"
	"__GO_MODULE__/internal/shared/validator"
)

func addressNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("Alamat tidak ditemukan")
	}
	return err
}

// SaveAddress adds an address (a.ID 0) or changes one. Marking it the default billing or
// shipping address takes the mark from the others.
func (s *Service) SaveAddress(ctx context.Context, supplierID int64, a domain.Address) (domain.Address, error) {
	co := company(ctx)
	now := s.clock.Now()
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.GetForUpdate(ctx, co, supplierID)
		if err != nil {
			return notFound(err)
		}
		var v validator.Validator
		a.Clean(&v, c.TaxIDForNITKU())
		if err := v.Err(); err != nil {
			return err
		}
		a.CompanyID, a.SupplierID, a.UpdatedAt = co, supplierID, now
		action := "address.updated"
		if a.ID == 0 {
			a.IsActive, a.CreatedAt, action = true, now, "address.added"
			if a.ID, err = s.repo.InsertAddress(ctx, a); err != nil {
				return err
			}
		} else {
			before, err := s.repo.Address(ctx, co, supplierID, a.ID)
			if err != nil {
				return addressNotFound(err)
			}
			if !before.IsActive && (a.IsDefaultBilling || a.IsDefaultShipping) {
				return apperror.Conflict("Alamat nonaktif tidak bisa menjadi alamat default — aktifkan dulu")
			}
			a.IsActive, a.CreatedAt = before.IsActive, before.CreatedAt
			if err := s.repo.UpdateAddress(ctx, a); err != nil {
				return err
			}
		}
		if a.IsDefaultBilling || a.IsDefaultShipping {
			if err := s.repo.ClearDefaults(ctx, co, supplierID, a.ID, a.IsDefaultBilling, a.IsDefaultShipping); err != nil {
				return err
			}
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "suppliers", Action: action, Target: target(supplierID),
			Summary: "Alamat " + a.Label + " pemasok " + c.Code, Data: map[string]any{"address_id": a.ID}})
		return nil
	})
	return a, err
}

// SetAddressActive deactivates or reactivates an address; a deactivated one loses its default
// marks. Documents that printed it keep their copy.
func (s *Service) SetAddressActive(ctx context.Context, supplierID, id int64, active bool) error {
	co := company(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.GetForUpdate(ctx, co, supplierID)
		if err != nil {
			return notFound(err)
		}
		a, err := s.repo.Address(ctx, co, supplierID, id)
		if err != nil {
			return addressNotFound(err)
		}
		if a.IsActive == active {
			return nil
		}
		if err := s.repo.SetAddressActive(ctx, co, supplierID, id, active, s.clock.Now()); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "suppliers", Action: "address.active_changed", Target: target(supplierID),
			Summary: "Alamat " + a.Label + " pemasok " + c.Code + " " + map[bool]string{true: "diaktifkan", false: "dinonaktifkan"}[active],
			Data:    map[string]any{"address_id": id, "active": active}})
		return nil
	})
}

// SaveContact adds a contact (c.ID 0) or changes one. Marking it primary takes the mark from
// the others.
func (s *Service) SaveContact(ctx context.Context, supplierID int64, ct domain.Contact) (domain.Contact, error) {
	co := company(ctx)
	now := s.clock.Now()
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := s.repo.GetForUpdate(ctx, co, supplierID); err != nil {
			return notFound(err)
		}
		var v validator.Validator
		ct.Clean(&v)
		if err := v.Err(); err != nil {
			return err
		}
		ct.CompanyID, ct.SupplierID, ct.UpdatedAt = co, supplierID, now
		if ct.ID == 0 {
			ct.CreatedAt = now
			id, err := s.repo.InsertContact(ctx, ct)
			if err != nil {
				return err
			}
			ct.ID = id
		} else {
			before, err := s.repo.Contact(ctx, co, supplierID, ct.ID)
			if err != nil {
				return contactNotFound(err)
			}
			ct.CreatedAt = before.CreatedAt
			if err := s.repo.UpdateContact(ctx, ct); err != nil {
				return err
			}
		}
		if ct.IsPrimary {
			return s.repo.ClearPrimaryContact(ctx, co, supplierID, ct.ID)
		}
		return nil
	})
	return ct, err
}

func contactNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("Kontak tidak ditemukan")
	}
	return err
}

// DeleteContact removes a contact: nothing else points at one.
func (s *Service) DeleteContact(ctx context.Context, supplierID, id int64) error {
	co := company(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := s.repo.GetForUpdate(ctx, co, supplierID); err != nil {
			return notFound(err)
		}
		return contactNotFound(s.repo.DeleteContact(ctx, co, supplierID, id))
	})
}

func bankNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("Rekening tidak ditemukan")
	}
	return err
}

func bankData(b domain.BankAccount) map[string]any {
	return map[string]any{"bank_name": b.BankName, "bank_code": b.BankCode, "account_number": taxid.Mask(b.AccountNumber), "account_holder": b.AccountHolder}
}

// SaveBankAccount adds an account (b.ID 0) or changes one: suppliers.sensitive across the
// company — where money goes is changed only by those who may see it — and a security record.
// It warns when the holder is not the supplier. Marking it default takes the mark from the others.
func (s *Service) SaveBankAccount(ctx context.Context, supplierID int64, b domain.BankAccount) (domain.BankAccount, []string, error) {
	if err := s.companyWide(ctx, PermSensitive, "Mengubah rekening pemasok butuh izin suppliers.sensitive di seluruh perusahaan"); err != nil {
		return b, nil, err
	}
	co := company(ctx)
	now := s.clock.Now()
	var warnings []string
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		sup, err := s.repo.GetForUpdate(ctx, co, supplierID)
		if err != nil {
			return notFound(err)
		}
		var v validator.Validator
		b.Clean(&v)
		if err := v.Err(); err != nil {
			return err
		}
		b.CompanyID, b.SupplierID, b.UpdatedAt = co, supplierID, now
		data := map[string]any{"after": bankData(b)}
		if b.ID == 0 {
			b.IsActive, b.CreatedAt = true, now
			if b.ID, err = s.repo.InsertBankAccount(ctx, b); err != nil {
				return err
			}
		} else {
			before, err := s.repo.BankAccount(ctx, co, supplierID, b.ID)
			if err != nil {
				return bankNotFound(err)
			}
			if !before.IsActive && b.IsDefault {
				return apperror.Conflict("Rekening nonaktif tidak bisa menjadi rekening utama — aktifkan dulu")
			}
			b.IsActive, b.CreatedAt = before.IsActive, before.CreatedAt
			data["before"] = bankData(before)
			if err := s.repo.UpdateBankAccount(ctx, b); err != nil {
				return err
			}
		}
		if b.IsDefault {
			if err := s.repo.ClearDefaultBankAccount(ctx, co, supplierID, b.ID); err != nil {
				return err
			}
		}
		if domain.HolderDiffers(b.AccountHolder, sup) {
			warnings = append(warnings, "Nama pemilik rekening berbeda dengan nama pemasok — pastikan rekening ini benar milik pemasok")
		}
		data["bank_account_id"] = b.ID
		s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "suppliers", Action: "bank_account.saved", Target: target(supplierID),
			Summary: "Rekening " + b.BankName + " pemasok " + sup.Code, Data: data})
		return nil
	})
	b.AccountNumber = taxid.Mask(b.AccountNumber)
	return b, warnings, err
}

// SetBankAccountActive deactivates or reactivates an account: suppliers.sensitive across the
// company, and a security record. A deactivated account loses its default mark.
func (s *Service) SetBankAccountActive(ctx context.Context, supplierID, id int64, active bool) error {
	if err := s.companyWide(ctx, PermSensitive, "Mengubah rekening pemasok butuh izin suppliers.sensitive di seluruh perusahaan"); err != nil {
		return err
	}
	co := company(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		sup, err := s.repo.GetForUpdate(ctx, co, supplierID)
		if err != nil {
			return notFound(err)
		}
		b, err := s.repo.BankAccount(ctx, co, supplierID, id)
		if err != nil {
			return bankNotFound(err)
		}
		if b.IsActive == active {
			return nil
		}
		if err := s.repo.SetBankAccountActive(ctx, co, supplierID, id, active, s.clock.Now()); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "suppliers", Action: "bank_account.active_changed", Target: target(supplierID),
			Summary: "Rekening " + b.BankName + " pemasok " + sup.Code + " " + map[bool]string{true: "diaktifkan", false: "dinonaktifkan"}[active],
			Data:    map[string]any{"bank_account_id": id, "active": active}})
		return nil
	})
}

// DefaultBankAccount is the account a payment goes to — the active default, else the first
// active one — whole, with an access record; ErrNotFound when there is none.
func (s *Service) DefaultBankAccount(ctx context.Context, supplierID int64) (domain.BankAccount, error) {
	co := company(ctx)
	if _, err := s.repo.Get(ctx, co, supplierID); err != nil {
		return domain.BankAccount{}, err
	}
	banks, err := s.repo.BankAccounts(ctx, co, supplierID)
	if err != nil {
		return domain.BankAccount{}, err
	}
	var pick domain.BankAccount
	for _, b := range banks {
		if b.IsActive && (b.IsDefault || pick.ID == 0) {
			pick = b
			if b.IsDefault {
				break
			}
		}
	}
	if pick.ID == 0 {
		return domain.BankAccount{}, ErrNotFound
	}
	s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditAccess, Module: "suppliers", Action: "bank_account.read", Target: target(supplierID),
		Summary: "Rekening pemasok dibaca untuk pembayaran", Data: map[string]any{"bank_account_id": pick.ID}})
	return pick, nil
}

// --- for other modules (contracts.SupplierClient) ---

// Get returns a supplier of the company, active or not; ErrNotFound when there is none.
func (s *Service) Get(ctx context.Context, id int64) (domain.Supplier, error) {
	return s.repo.Get(ctx, company(ctx), id)
}

// Search finds up to limit (1–50) active suppliers whose code, name, or tax ID matches q.
func (s *Service) Search(ctx context.Context, q string, limit int) ([]domain.Supplier, error) {
	limit = max(1, min(limit, 50))
	rows, err := s.repo.Search(ctx, company(ctx), q, limit)
	for i := range rows {
		rows[i] = Masked(rows[i])
	}
	return rows, err
}

// ErrInactive and ErrOnHold are what Usable refuses with.
var (
	ErrInactive = errors.New("suppliers: inactive")
	ErrOnHold   = errors.New("suppliers: on hold")
)

// Usable checks, holding the row shared until the caller's transaction ends, that a document
// may use the supplier now.
func (s *Service) Usable(ctx context.Context, id int64) error {
	c, err := s.repo.GetShared(ctx, company(ctx), id)
	switch {
	case err != nil:
		return err
	case !c.IsActive:
		return ErrInactive
	case c.OnHold:
		return ErrOnHold
	}
	return nil
}

// Snapshot is the supplier and its address addressID (0: the default billing address, else
// the first active billing one) as a document copies them.
func (s *Service) Snapshot(ctx context.Context, id, addressID int64) (domain.Supplier, domain.Address, error) {
	co := company(ctx)
	c, err := s.repo.Get(ctx, co, id)
	if err != nil {
		return domain.Supplier{}, domain.Address{}, err
	}
	if addressID != 0 {
		a, err := s.repo.Address(ctx, co, id, addressID)
		if err != nil {
			return c, domain.Address{}, errors.Join(err, errors.New("suppliers: address "+strconv.FormatInt(addressID, 10)))
		}
		return c, a, nil
	}
	addrs, err := s.repo.Addresses(ctx, co, id)
	if err != nil {
		return c, domain.Address{}, err
	}
	var pick domain.Address
	for _, a := range addrs {
		if !a.IsActive || !a.ForBilling {
			continue
		}
		if a.IsDefaultBilling {
			return c, a, nil
		}
		if pick.ID == 0 {
			pick = a
		}
	}
	return c, pick, nil
}

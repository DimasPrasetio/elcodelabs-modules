package application

import (
	"context"
	"errors"
	"strconv"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/customers/domain"
	"__GO_MODULE__/internal/shared/apperror"
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
func (s *Service) SaveAddress(ctx context.Context, customerID int64, a domain.Address) (domain.Address, error) {
	co := company(ctx)
	now := s.clock.Now()
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.GetForUpdate(ctx, co, customerID)
		if err != nil {
			return notFound(err)
		}
		var v validator.Validator
		a.Clean(&v, c.TaxIDForNITKU())
		if err := v.Err(); err != nil {
			return err
		}
		a.CompanyID, a.CustomerID, a.UpdatedAt = co, customerID, now
		action := "address.updated"
		if a.ID == 0 {
			a.IsActive, a.CreatedAt, action = true, now, "address.added"
			if a.ID, err = s.repo.InsertAddress(ctx, a); err != nil {
				return err
			}
		} else {
			before, err := s.repo.Address(ctx, co, customerID, a.ID)
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
			if err := s.repo.ClearDefaults(ctx, co, customerID, a.ID, a.IsDefaultBilling, a.IsDefaultShipping); err != nil {
				return err
			}
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "customers", Action: action, Target: target(customerID),
			Summary: "Alamat " + a.Label + " pelanggan " + c.Code, Data: map[string]any{"address_id": a.ID}})
		return nil
	})
	return a, err
}

// SetAddressActive deactivates or reactivates an address; a deactivated one loses its default
// marks. Documents that printed it keep their copy.
func (s *Service) SetAddressActive(ctx context.Context, customerID, id int64, active bool) error {
	co := company(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.GetForUpdate(ctx, co, customerID)
		if err != nil {
			return notFound(err)
		}
		a, err := s.repo.Address(ctx, co, customerID, id)
		if err != nil {
			return addressNotFound(err)
		}
		if a.IsActive == active {
			return nil
		}
		if err := s.repo.SetAddressActive(ctx, co, customerID, id, active, s.clock.Now()); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "customers", Action: "address.active_changed", Target: target(customerID),
			Summary: "Alamat " + a.Label + " pelanggan " + c.Code + " " + map[bool]string{true: "diaktifkan", false: "dinonaktifkan"}[active],
			Data:    map[string]any{"address_id": id, "active": active}})
		return nil
	})
}

// SaveContact adds a contact (c.ID 0) or changes one. Marking it primary takes the mark from
// the others.
func (s *Service) SaveContact(ctx context.Context, customerID int64, ct domain.Contact) (domain.Contact, error) {
	co := company(ctx)
	now := s.clock.Now()
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := s.repo.GetForUpdate(ctx, co, customerID); err != nil {
			return notFound(err)
		}
		var v validator.Validator
		ct.Clean(&v)
		if err := v.Err(); err != nil {
			return err
		}
		ct.CompanyID, ct.CustomerID, ct.UpdatedAt = co, customerID, now
		if ct.ID == 0 {
			ct.CreatedAt = now
			id, err := s.repo.InsertContact(ctx, ct)
			if err != nil {
				return err
			}
			ct.ID = id
		} else {
			before, err := s.repo.Contact(ctx, co, customerID, ct.ID)
			if err != nil {
				return contactNotFound(err)
			}
			ct.CreatedAt = before.CreatedAt
			if err := s.repo.UpdateContact(ctx, ct); err != nil {
				return err
			}
		}
		if ct.IsPrimary {
			return s.repo.ClearPrimaryContact(ctx, co, customerID, ct.ID)
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
func (s *Service) DeleteContact(ctx context.Context, customerID, id int64) error {
	co := company(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := s.repo.GetForUpdate(ctx, co, customerID); err != nil {
			return notFound(err)
		}
		return contactNotFound(s.repo.DeleteContact(ctx, co, customerID, id))
	})
}

// --- for other modules (contracts.CustomerClient) ---

// Get returns a customer of the company, active or not; ErrNotFound when there is none.
func (s *Service) Get(ctx context.Context, id int64) (domain.Customer, error) {
	return s.repo.Get(ctx, company(ctx), id)
}

// Search finds up to limit (1–50) active customers whose code, name, or tax ID matches q.
func (s *Service) Search(ctx context.Context, q string, limit int) ([]domain.Customer, error) {
	limit = max(1, min(limit, 50))
	rows, err := s.repo.Search(ctx, company(ctx), q, limit)
	for i := range rows {
		rows[i] = Masked(rows[i])
	}
	return rows, err
}

// ErrInactive and ErrOnHold are what Usable refuses with.
var (
	ErrInactive = errors.New("customers: inactive")
	ErrOnHold   = errors.New("customers: on hold")
)

// Usable checks, holding the row shared until the caller's transaction ends, that a document
// may use the customer now.
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

// Snapshot is the customer and its address addressID (0: the default billing address, else
// the first active billing one) as a document copies them.
func (s *Service) Snapshot(ctx context.Context, id, addressID int64) (domain.Customer, domain.Address, error) {
	co := company(ctx)
	c, err := s.repo.Get(ctx, co, id)
	if err != nil {
		return domain.Customer{}, domain.Address{}, err
	}
	if addressID != 0 {
		a, err := s.repo.Address(ctx, co, id, addressID)
		if err != nil {
			return c, domain.Address{}, errors.Join(err, errors.New("customers: address "+strconv.FormatInt(addressID, 10)))
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

// Package application is the ledger's use cases.
package application

import (
	"context"

	"__GO_MODULE__/internal/modules/ledger/contracts"
)

type Repository interface {
	Add(ctx context.Context, sku string, qty int) error
	Take(ctx context.Context, sku string, qty int) (bool, error)
	Note(ctx context.Context, body string) error
}

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) Add(ctx context.Context, sku string, qty int) error {
	return s.repo.Add(ctx, sku, qty)
}

func (s *Service) Take(ctx context.Context, sku string, qty int) error {
	ok, err := s.repo.Take(ctx, sku, qty)
	if err == nil && !ok {
		return contracts.ErrInsufficient
	}
	return err
}

func (s *Service) Note(ctx context.Context, body string) error { return s.repo.Note(ctx, body) }

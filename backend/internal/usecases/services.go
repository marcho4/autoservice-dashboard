package usecases

import (
	"context"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

// CatalogService lets the owner manage the services of the autoservice and the form the bot fills for each of them.
type CatalogService struct {
	autoservices *AutoserviceService
	services     ServiceRepository
}

func NewCatalogService(a *AutoserviceService, s ServiceRepository) *CatalogService {
	return &CatalogService{autoservices: a, services: s}
}

func (s *CatalogService) List(ctx context.Context, employeeID uuid.UUID) ([]domain.Service, error) {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	return s.services.List(ctx, a.ID, false)
}

func (s *CatalogService) Create(ctx context.Context, employeeID uuid.UUID, in domain.ServiceInput) (domain.Service, error) {
	in, err := in.Normalize()
	if err != nil {
		return domain.Service{}, err
	}
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return domain.Service{}, err
	}
	return s.services.Create(ctx, a.ID, in)
}

// CreateDefaults restores the starter catalog for an autoservice that has no services.
func (s *CatalogService) CreateDefaults(ctx context.Context, employeeID uuid.UUID) ([]domain.Service, error) {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if err := s.services.CreateDefaults(ctx, a.ID, domain.DefaultServices()); err != nil {
		return nil, err
	}
	return s.services.List(ctx, a.ID, false)
}

func (s *CatalogService) Update(ctx context.Context, employeeID, serviceID uuid.UUID, in domain.ServiceInput) (domain.Service, error) {
	in, err := in.Normalize()
	if err != nil {
		return domain.Service{}, err
	}
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return domain.Service{}, err
	}
	return s.services.Update(ctx, a.ID, serviceID, in)
}

func (s *CatalogService) Delete(ctx context.Context, employeeID, serviceID uuid.UUID) error {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return err
	}
	return s.services.Delete(ctx, a.ID, serviceID)
}

func (s *CatalogService) Reorder(ctx context.Context, employeeID uuid.UUID, ids []uuid.UUID) ([]domain.Service, error) {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if err := s.services.Reorder(ctx, a.ID, ids); err != nil {
		return nil, err
	}
	return s.services.List(ctx, a.ID, false)
}

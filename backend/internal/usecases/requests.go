package usecases

import (
	"context"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type DashboardService struct {
	autoservices *AutoserviceService
	clients      ClientRepository
	requests     RequestRepository
}

func NewDashboardService(a *AutoserviceService, c ClientRepository, r RequestRepository) *DashboardService {
	return &DashboardService{autoservices: a, clients: c, requests: r}
}

func (s *DashboardService) ListRequests(ctx context.Context, employeeID uuid.UUID, f domain.RequestFilter) ([]domain.RequestWithClient, int, error) {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return nil, 0, err
	}
	return s.requests.List(ctx, a.ID, normalizeFilter(f))
}

func (s *DashboardService) GetRequest(ctx context.Context, employeeID, requestID uuid.UUID) (domain.RequestWithClient, error) {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return domain.RequestWithClient{}, err
	}
	return s.requests.Get(ctx, a.ID, requestID)
}

func (s *DashboardService) ChangeStatus(ctx context.Context, employeeID, requestID uuid.UUID, to domain.RequestStatus) (domain.RequestWithClient, error) {
	current, err := s.GetRequest(ctx, employeeID, requestID)
	if err != nil {
		return domain.RequestWithClient{}, err
	}
	if !domain.CanTransition(current.Status, to, domain.ActorEmployee) {
		return domain.RequestWithClient{}, domain.ErrInvalidTransition
	}
	updated, err := s.requests.UpdateStatus(ctx, current.AutoserviceID, current.ID, current.Status, to)
	if err != nil {
		return domain.RequestWithClient{}, err
	}
	current.Request = updated
	return current, nil
}

func (s *DashboardService) ListClients(ctx context.Context, employeeID uuid.UUID) ([]domain.ClientSummary, error) {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	return s.clients.List(ctx, a.ID)
}

func (s *DashboardService) GetClient(ctx context.Context, employeeID, clientID uuid.UUID) (domain.Client, []domain.Request, error) {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return domain.Client{}, nil, err
	}
	c, err := s.clients.Get(ctx, a.ID, clientID)
	if err != nil {
		return domain.Client{}, nil, err
	}
	requests, err := s.requests.ListByClient(ctx, a.ID, c.ID)
	if err != nil {
		return domain.Client{}, nil, err
	}
	return c, requests, nil
}

const (
	defaultLimit = 50
	maxLimit     = 200
)

func normalizeFilter(f domain.RequestFilter) domain.RequestFilter {
	if f.Sort != domain.SortAsc {
		f.Sort = domain.SortDesc
	}
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	f.Limit = min(f.Limit, maxLimit)
	f.Offset = max(f.Offset, 0)
	return f
}

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

type RequestPage struct {
	Items []domain.RequestWithClient
	Total int
	// Counts covers all requests of the autoservice regardless of the filter, for the status tabs.
	Counts domain.RequestCounts
}

func (s *DashboardService) ListRequests(ctx context.Context, employeeID uuid.UUID, f domain.RequestFilter) (RequestPage, error) {
	a, err := s.autoservices.Get(ctx, employeeID)
	if err != nil {
		return RequestPage{}, err
	}
	items, total, err := s.requests.List(ctx, a.ID, normalizeFilter(f))
	if err != nil {
		return RequestPage{}, err
	}
	counts, err := s.requests.Counts(ctx, a.ID)
	return RequestPage{Items: items, Total: total, Counts: counts}, err
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

func (s *DashboardService) UpdateNote(ctx context.Context, employeeID, requestID uuid.UUID, note string) (domain.RequestWithClient, error) {
	note, err := domain.NormalizeNote(note)
	if err != nil {
		return domain.RequestWithClient{}, err
	}
	current, err := s.GetRequest(ctx, employeeID, requestID)
	if err != nil {
		return domain.RequestWithClient{}, err
	}
	updated, err := s.requests.UpdateNote(ctx, current.AutoserviceID, current.ID, note)
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

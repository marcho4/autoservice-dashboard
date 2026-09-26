package usecases

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type BotRunnerService struct {
	bots     BotRepository
	clients  ClientRepository
	requests RequestRepository
}

func NewBotRunnerService(b BotRepository, c ClientRepository, r RequestRepository) *BotRunnerService {
	return &BotRunnerService{bots: b, clients: c, requests: r}
}

func (s *BotRunnerService) ListBots(ctx context.Context) ([]domain.Bot, error) {
	return s.bots.List(ctx)
}

func (s *BotRunnerService) CreateRequest(ctx context.Context, botID int64, who domain.ClientIdentity, in domain.RequestInput) (domain.Request, error) {
	if err := who.Validate(); err != nil {
		return domain.Request{}, err
	}
	in, err := in.Normalize()
	if err != nil {
		return domain.Request{}, err
	}
	bot, err := s.bots.GetByTelegramID(ctx, botID)
	if err != nil {
		return domain.Request{}, err
	}
	client, err := s.clients.Upsert(ctx, bot.AutoserviceID, who, in.Phone)
	if err != nil {
		return domain.Request{}, err
	}
	return s.requests.Create(ctx, domain.Request{
		AutoserviceID: bot.AutoserviceID,
		ClientID:      client.ID,
		Status:        domain.StatusNew,
		CarBrand:      in.CarBrand,
		CarModel:      in.CarModel,
		Description:   in.Description,
		Phone:         in.Phone,
	})
}

func (s *BotRunnerService) ListClientRequests(ctx context.Context, botID, telegramID int64) ([]domain.Request, error) {
	bot, err := s.bots.GetByTelegramID(ctx, botID)
	if err != nil {
		return nil, err
	}
	client, err := s.clients.GetByTelegramID(ctx, bot.AutoserviceID, telegramID)
	if errors.Is(err, domain.ErrNotFound) {
		return []domain.Request{}, nil
	}
	if err != nil {
		return nil, err
	}
	return s.requests.ListByClient(ctx, client.AutoserviceID, client.ID)
}

func (s *BotRunnerService) GetClientRequest(ctx context.Context, botID, telegramID int64, requestID uuid.UUID) (domain.Request, error) {
	client, err := s.findClient(ctx, botID, telegramID)
	if err != nil {
		return domain.Request{}, err
	}
	r, err := s.requests.Get(ctx, client.AutoserviceID, requestID)
	if err != nil {
		return domain.Request{}, err
	}
	if r.ClientID != client.ID {
		return domain.Request{}, domain.ErrNotFound
	}
	return r.Request, nil
}

func (s *BotRunnerService) UpdateClientRequest(ctx context.Context, botID, telegramID int64, requestID uuid.UUID, in domain.RequestInput) (domain.Request, error) {
	in, err := in.Normalize()
	if err != nil {
		return domain.Request{}, err
	}
	current, err := s.GetClientRequest(ctx, botID, telegramID, requestID)
	if err != nil {
		return domain.Request{}, err
	}
	if !current.IsEditable() {
		return domain.Request{}, domain.ErrRequestNotEditable
	}
	return s.requests.UpdateContent(ctx, current.AutoserviceID, current.ClientID, current.ID, in)
}

func (s *BotRunnerService) CancelClientRequest(ctx context.Context, botID, telegramID int64, requestID uuid.UUID) (domain.Request, error) {
	current, err := s.GetClientRequest(ctx, botID, telegramID, requestID)
	if err != nil {
		return domain.Request{}, err
	}
	if !domain.CanTransition(current.Status, domain.StatusCancelled, domain.ActorClient) {
		return domain.Request{}, domain.ErrRequestNotEditable
	}
	return s.requests.UpdateStatus(ctx, current.AutoserviceID, current.ID, current.Status, domain.StatusCancelled)
}

func (s *BotRunnerService) findClient(ctx context.Context, botID, telegramID int64) (domain.Client, error) {
	bot, err := s.bots.GetByTelegramID(ctx, botID)
	if err != nil {
		return domain.Client{}, err
	}
	return s.clients.GetByTelegramID(ctx, bot.AutoserviceID, telegramID)
}

package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/VictoriaMetrics/metrics"
	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

// BotRunnerService is everything a bot may do on behalf of a Telegram client of one autoservice.
// The built-in Telegram bot calls it directly; the optional internal HTTP API exposes the same operations to external integrations.
type BotRunnerService struct {
	bots         BotRepository
	autoservices AutoserviceRepository
	services     ServiceRepository
	clients      ClientRepository
	requests     RequestRepository
}

func NewBotRunnerService(b BotRepository, a AutoserviceRepository, s ServiceRepository, c ClientRepository, r RequestRepository) *BotRunnerService {
	return &BotRunnerService{bots: b, autoservices: a, services: s, clients: c, requests: r}
}

func (s *BotRunnerService) ListBots(ctx context.Context) ([]domain.Bot, error) {
	return s.bots.List(ctx)
}

// ResolveBot maps a Telegram bot ID to the autoservice the bot is connected to.
func (s *BotRunnerService) ResolveBot(ctx context.Context, telegramBotID int64) (uuid.UUID, error) {
	bot, err := s.bots.GetByTelegramID(ctx, telegramBotID)
	return bot.AutoserviceID, err
}

func (s *BotRunnerService) SetBotStatus(ctx context.Context, bot domain.Bot, status domain.BotStatus, reason string) error {
	return s.bots.SetStatus(ctx, bot.AutoserviceID, bot.Token, status, reason)
}

func (s *BotRunnerService) Autoservice(ctx context.Context, autoserviceID uuid.UUID) (domain.Autoservice, error) {
	return s.autoservices.Get(ctx, autoserviceID)
}

func (s *BotRunnerService) ActiveServices(ctx context.Context, autoserviceID uuid.UUID) ([]domain.Service, error) {
	return s.services.List(ctx, autoserviceID, true)
}

func (s *BotRunnerService) FindClient(ctx context.Context, autoserviceID uuid.UUID, telegramID int64) (domain.Client, error) {
	return s.clients.GetByTelegramID(ctx, autoserviceID, telegramID)
}

func (s *BotRunnerService) CreateRequest(ctx context.Context, autoserviceID uuid.UUID, who domain.ClientIdentity, in domain.RequestInput) (domain.Request, error) {
	if err := who.Validate(); err != nil {
		return domain.Request{}, err
	}
	in, err := in.Normalize()
	if err != nil {
		return domain.Request{}, err
	}
	client, err := s.clients.Upsert(ctx, autoserviceID, who, in.Phone)
	if err != nil {
		return domain.Request{}, err
	}
	open, err := s.requests.CountByClientStatus(ctx, autoserviceID, client.ID, domain.StatusNew)
	if err != nil {
		return domain.Request{}, err
	}
	if open >= domain.MaxOpenRequestsPerClient {
		return domain.Request{}, domain.ErrTooManyRequests
	}
	request, err := s.requests.Create(ctx, domain.Request{
		AutoserviceID: autoserviceID,
		ClientID:      client.ID,
		Status:        domain.StatusNew,
		ServiceID:     in.ServiceID,
		ServiceName:   in.ServiceName,
		Answers:       in.Answers,
		Message:       in.Message,
		Phone:         in.Phone,
	})
	if err == nil {
		metrics.GetOrCreateCounter(fmt.Sprintf(`requests_created_total{autoservice_id=%q}`, request.AutoserviceID)).Inc()
	}
	return request, err
}

func (s *BotRunnerService) ListClientRequests(ctx context.Context, autoserviceID uuid.UUID, telegramID int64) ([]domain.Request, error) {
	client, err := s.clients.GetByTelegramID(ctx, autoserviceID, telegramID)
	if errors.Is(err, domain.ErrNotFound) {
		return []domain.Request{}, nil
	}
	if err != nil {
		return nil, err
	}
	return s.requests.ListByClient(ctx, client.AutoserviceID, client.ID)
}

func (s *BotRunnerService) GetClientRequest(ctx context.Context, autoserviceID uuid.UUID, telegramID int64, requestID uuid.UUID) (domain.Request, error) {
	client, err := s.clients.GetByTelegramID(ctx, autoserviceID, telegramID)
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

func (s *BotRunnerService) UpdateClientRequest(ctx context.Context, autoserviceID uuid.UUID, telegramID int64, requestID uuid.UUID, in domain.RequestInput) (domain.Request, error) {
	in, err := in.Normalize()
	if err != nil {
		return domain.Request{}, err
	}
	current, err := s.GetClientRequest(ctx, autoserviceID, telegramID, requestID)
	if err != nil {
		return domain.Request{}, err
	}
	if !current.IsEditable() {
		return domain.Request{}, domain.ErrRequestNotEditable
	}
	return s.requests.UpdateContent(ctx, current.AutoserviceID, current.ClientID, current.ID, in)
}

func (s *BotRunnerService) CancelClientRequest(ctx context.Context, autoserviceID uuid.UUID, telegramID int64, requestID uuid.UUID) (domain.Request, error) {
	current, err := s.GetClientRequest(ctx, autoserviceID, telegramID, requestID)
	if err != nil {
		return domain.Request{}, err
	}
	if !domain.CanTransition(current.Status, domain.StatusCancelled, domain.ActorClient) {
		return domain.Request{}, domain.ErrRequestNotEditable
	}
	return s.requests.UpdateStatus(ctx, current.AutoserviceID, current.ID, current.Status, domain.StatusCancelled)
}

// LinkRecipient subscribes a Telegram chat to new-request notifications if the code from the deep link is the current one.
func (s *BotRunnerService) LinkRecipient(ctx context.Context, autoserviceID uuid.UUID, code string, chatID int64, name string) error {
	if code == "" {
		return domain.ErrNotFound
	}
	return s.bots.AddRecipient(ctx, autoserviceID, code, domain.NewLinkCode(), chatID, name)
}

func (s *BotRunnerService) Recipients(ctx context.Context, autoserviceID uuid.UUID) ([]domain.NotifyRecipient, error) {
	return s.bots.ListRecipients(ctx, autoserviceID)
}

// RemoveRecipient drops a chat that can no longer be reached, e.g. the master blocked the bot.
func (s *BotRunnerService) RemoveRecipient(ctx context.Context, autoserviceID uuid.UUID, chatID int64) error {
	return s.bots.RemoveRecipient(ctx, autoserviceID, chatID)
}

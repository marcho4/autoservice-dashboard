package usecases

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type AutoserviceService struct {
	autoservices AutoserviceRepository
	bots         BotRepository
	telegram     TelegramAPI
}

func NewAutoserviceService(a AutoserviceRepository, b BotRepository, t TelegramAPI) *AutoserviceService {
	return &AutoserviceService{autoservices: a, bots: b, telegram: t}
}

func (s *AutoserviceService) Get(ctx context.Context, employeeID uuid.UUID) (domain.Autoservice, error) {
	a, err := s.autoservices.GetByOwner(ctx, employeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Autoservice{}, domain.ErrNoAutoservice
	}
	return a, err
}

func (s *AutoserviceService) Create(ctx context.Context, employeeID uuid.UUID, in domain.AutoserviceInput) (domain.Autoservice, error) {
	in, err := in.Normalize()
	if err != nil {
		return domain.Autoservice{}, err
	}
	return s.autoservices.Create(ctx, domain.Autoservice{
		OwnerID:     employeeID,
		Name:        in.Name,
		Address:     in.Address,
		Phone:       in.Phone,
		Description: in.Description,
	})
}

func (s *AutoserviceService) Update(ctx context.Context, employeeID uuid.UUID, in domain.AutoserviceInput) (domain.Autoservice, error) {
	in, err := in.Normalize()
	if err != nil {
		return domain.Autoservice{}, err
	}
	a, err := s.Get(ctx, employeeID)
	if err != nil {
		return domain.Autoservice{}, err
	}
	return s.autoservices.Update(ctx, a.ID, in)
}

func (s *AutoserviceService) Delete(ctx context.Context, employeeID uuid.UUID) error {
	a, err := s.Get(ctx, employeeID)
	if err != nil {
		return err
	}
	return s.autoservices.Delete(ctx, a.ID)
}

func (s *AutoserviceService) ConnectBot(ctx context.Context, employeeID uuid.UUID, token string) (domain.Bot, error) {
	token, err := domain.NormalizeBotToken(token)
	if err != nil {
		return domain.Bot{}, err
	}
	a, err := s.Get(ctx, employeeID)
	if err != nil {
		return domain.Bot{}, err
	}
	info, err := s.telegram.GetMe(ctx, token)
	if err != nil {
		return domain.Bot{}, err
	}
	return s.bots.Upsert(ctx, domain.Bot{
		AutoserviceID: a.ID,
		TelegramBotID: info.ID,
		Username:      info.Username,
		Token:         token,
	})
}

func (s *AutoserviceService) GetBot(ctx context.Context, employeeID uuid.UUID) (domain.Bot, error) {
	a, err := s.Get(ctx, employeeID)
	if err != nil {
		return domain.Bot{}, err
	}
	bot, err := s.bots.GetByAutoservice(ctx, a.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Bot{}, domain.ErrBotNotConnected
	}
	return bot, err
}

func (s *AutoserviceService) DisconnectBot(ctx context.Context, employeeID uuid.UUID) error {
	a, err := s.Get(ctx, employeeID)
	if err != nil {
		return err
	}
	err = s.bots.Delete(ctx, a.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrBotNotConnected
	}
	return err
}

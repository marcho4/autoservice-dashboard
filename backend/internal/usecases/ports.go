package usecases

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/pkg/jwtauth"
)

type EmployeeRepository interface {
	Create(ctx context.Context, e domain.Employee) (domain.Employee, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Employee, error)
	GetByEmail(ctx context.Context, email string) (domain.Employee, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, email, fullName string) (domain.Employee, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string) (domain.Employee, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type RevokedTokenRepository interface {
	Revoke(ctx context.Context, jti string, expiresAt time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

type AutoserviceRepository interface {
	Create(ctx context.Context, a domain.Autoservice) (domain.Autoservice, error)
	GetByOwner(ctx context.Context, ownerID uuid.UUID) (domain.Autoservice, error)
	Update(ctx context.Context, id uuid.UUID, in domain.AutoserviceInput) (domain.Autoservice, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type BotRepository interface {
	Upsert(ctx context.Context, b domain.Bot) (domain.Bot, error)
	GetByAutoservice(ctx context.Context, autoserviceID uuid.UUID) (domain.Bot, error)
	GetByTelegramID(ctx context.Context, telegramBotID int64) (domain.Bot, error)
	List(ctx context.Context) ([]domain.Bot, error)
	Delete(ctx context.Context, autoserviceID uuid.UUID) error
}

type ClientRepository interface {
	Upsert(ctx context.Context, autoserviceID uuid.UUID, identity domain.ClientIdentity, phone string) (domain.Client, error)
	Get(ctx context.Context, autoserviceID, id uuid.UUID) (domain.Client, error)
	GetByTelegramID(ctx context.Context, autoserviceID uuid.UUID, telegramID int64) (domain.Client, error)
	List(ctx context.Context, autoserviceID uuid.UUID) ([]domain.ClientSummary, error)
}

type RequestRepository interface {
	Create(ctx context.Context, r domain.Request) (domain.Request, error)
	Get(ctx context.Context, autoserviceID, id uuid.UUID) (domain.RequestWithClient, error)
	List(ctx context.Context, autoserviceID uuid.UUID, f domain.RequestFilter) ([]domain.RequestWithClient, int, error)
	ListByClient(ctx context.Context, autoserviceID, clientID uuid.UUID) ([]domain.Request, error)
	UpdateStatus(ctx context.Context, autoserviceID, id uuid.UUID, from, to domain.RequestStatus) (domain.Request, error)
	UpdateContent(ctx context.Context, autoserviceID, clientID, id uuid.UUID, in domain.RequestInput) (domain.Request, error)
}

type TelegramBotInfo struct {
	ID       int64
	Username string
}

type TelegramAPI interface {
	GetMe(ctx context.Context, token string) (TelegramBotInfo, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type TokenManager interface {
	Issue(subject string, version int) (string, jwtauth.Claims, error)
	Parse(token string) (jwtauth.Claims, error)
}

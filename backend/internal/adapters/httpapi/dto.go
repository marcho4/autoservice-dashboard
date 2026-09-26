package httpapi

import (
	"time"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
)

type listDTO[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

func mapAll[T, R any](items []T, convert func(T) R) []R {
	out := make([]R, len(items))
	for i, item := range items {
		out[i] = convert(item)
	}
	return out
}

type employeeDTO struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toEmployee(e domain.Employee) employeeDTO {
	return employeeDTO{
		ID:        e.ID,
		Email:     e.Email,
		FullName:  e.FullName,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

type sessionDTO struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresAt   time.Time   `json:"expires_at"`
	Employee    employeeDTO `json:"employee"`
}

func toSession(s usecases.Session) sessionDTO {
	return sessionDTO{
		AccessToken: s.Token,
		TokenType:   "Bearer",
		ExpiresAt:   s.ExpiresAt,
		Employee:    toEmployee(s.Employee),
	}
}

type autoserviceDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Phone       string    `json:"phone"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toAutoservice(a domain.Autoservice) autoserviceDTO {
	return autoserviceDTO{
		ID:          a.ID,
		Name:        a.Name,
		Address:     a.Address,
		Phone:       a.Phone,
		Description: a.Description,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}

type autoserviceInputDTO struct {
	Name        string `json:"name"`
	Address     string `json:"address"`
	Phone       string `json:"phone"`
	Description string `json:"description"`
}

func (d autoserviceInputDTO) toDomain() domain.AutoserviceInput {
	return domain.AutoserviceInput{
		Name:        d.Name,
		Address:     d.Address,
		Phone:       d.Phone,
		Description: d.Description,
	}
}

type botDTO struct {
	Username      string    `json:"username"`
	TelegramBotID int64     `json:"telegram_bot_id"`
	MaskedToken   string    `json:"masked_token"`
	ConnectedAt   time.Time `json:"connected_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toBot(b domain.Bot) botDTO {
	return botDTO{
		Username:      b.Username,
		TelegramBotID: b.TelegramBotID,
		MaskedToken:   domain.MaskBotToken(b.Token),
		ConnectedAt:   b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
	}
}

type runnerBotDTO struct {
	AutoserviceID uuid.UUID `json:"autoservice_id"`
	TelegramBotID int64     `json:"telegram_bot_id"`
	Username      string    `json:"username"`
	Token         string    `json:"token"`
}

func toRunnerBot(b domain.Bot) runnerBotDTO {
	return runnerBotDTO{
		AutoserviceID: b.AutoserviceID,
		TelegramBotID: b.TelegramBotID,
		Username:      b.Username,
		Token:         b.Token,
	}
}

type clientDTO struct {
	ID               uuid.UUID `json:"id"`
	TelegramID       int64     `json:"telegram_id"`
	TelegramUsername string    `json:"telegram_username"`
	Name             string    `json:"name"`
	Phone            string    `json:"phone"`
	CreatedAt        time.Time `json:"created_at"`
}

func toClient(c domain.Client) clientDTO {
	return clientDTO{
		ID:               c.ID,
		TelegramID:       c.TelegramID,
		TelegramUsername: c.TelegramUsername,
		Name:             c.Name,
		Phone:            c.Phone,
		CreatedAt:        c.CreatedAt,
	}
}

type clientSummaryDTO struct {
	clientDTO
	RequestsCount int        `json:"requests_count"`
	LastRequestAt *time.Time `json:"last_request_at"`
}

func toClientSummary(c domain.ClientSummary) clientSummaryDTO {
	return clientSummaryDTO{
		clientDTO:     toClient(c.Client),
		RequestsCount: c.RequestsCount,
		LastRequestAt: c.LastRequestAt,
	}
}

type clientDetailsDTO struct {
	clientDTO
	Requests []requestDTO `json:"requests"`
}

type clientIdentityDTO struct {
	TelegramID       int64  `json:"telegram_id"`
	TelegramUsername string `json:"telegram_username"`
	Name             string `json:"name"`
}

func (d clientIdentityDTO) toDomain() domain.ClientIdentity {
	return domain.ClientIdentity{
		TelegramID:       d.TelegramID,
		TelegramUsername: d.TelegramUsername,
		Name:             d.Name,
	}
}

type requestDTO struct {
	ID          uuid.UUID            `json:"id"`
	Status      domain.RequestStatus `json:"status"`
	CarBrand    string               `json:"car_brand"`
	CarModel    string               `json:"car_model"`
	Description string               `json:"description"`
	Phone       string               `json:"phone"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

func toRequest(r domain.Request) requestDTO {
	return requestDTO{
		ID:          r.ID,
		Status:      r.Status,
		CarBrand:    r.CarBrand,
		CarModel:    r.CarModel,
		Description: r.Description,
		Phone:       r.Phone,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type requestWithClientDTO struct {
	requestDTO
	Client clientDTO `json:"client"`
}

func toRequestWithClient(r domain.RequestWithClient) requestWithClientDTO {
	return requestWithClientDTO{requestDTO: toRequest(r.Request), Client: toClient(r.Client)}
}

type requestInputDTO struct {
	CarBrand    string `json:"car_brand"`
	CarModel    string `json:"car_model"`
	Description string `json:"description"`
	Phone       string `json:"phone"`
}

func (d requestInputDTO) toDomain() domain.RequestInput {
	return domain.RequestInput{
		CarBrand:    d.CarBrand,
		CarModel:    d.CarModel,
		Description: d.Description,
		Phone:       d.Phone,
	}
}

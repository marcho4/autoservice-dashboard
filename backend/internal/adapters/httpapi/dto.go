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

type recipientDTO struct {
	ChatID    int64     `json:"chat_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type botDTO struct {
	Username      string           `json:"username"`
	TelegramBotID int64            `json:"telegram_bot_id"`
	MaskedToken   string           `json:"masked_token"`
	Link          string           `json:"link"`
	Status        domain.BotStatus `json:"status"`
	StatusError   string           `json:"status_error"`
	StatusAt      time.Time        `json:"status_at"`
	// NotifyLink is the one-time deep link a master opens to get new requests in Telegram.
	NotifyLink  string         `json:"notify_link"`
	Recipients  []recipientDTO `json:"recipients"`
	ConnectedAt time.Time      `json:"connected_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func toBot(d usecases.BotDetails) botDTO {
	b := d.Bot
	return botDTO{
		Username:      b.Username,
		TelegramBotID: b.TelegramBotID,
		MaskedToken:   domain.MaskBotToken(b.Token),
		Link:          b.Link(),
		Status:        b.Status,
		StatusError:   b.StatusError,
		StatusAt:      b.StatusAt,
		NotifyLink:    b.MasterLink(),
		Recipients: mapAll(d.Recipients, func(r domain.NotifyRecipient) recipientDTO {
			return recipientDTO{ChatID: r.ChatID, Name: r.Name, CreatedAt: r.CreatedAt}
		}),
		ConnectedAt: b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
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

type fieldDTO struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Question string   `json:"question"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options"`
}

type serviceDTO struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsActive    bool       `json:"is_active"`
	Fields      []fieldDTO `json:"fields"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func toService(s domain.Service) serviceDTO {
	return serviceDTO{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		IsActive:    s.Active,
		Fields: mapAll(s.Fields, func(f domain.FormField) fieldDTO {
			options := f.Options
			if options == nil {
				options = []string{}
			}
			return fieldDTO{ID: f.ID, Label: f.Label, Question: f.Question, Type: string(f.Type), Required: f.Required, Options: options}
		}),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

type serviceInputDTO struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsActive    *bool      `json:"is_active"`
	Fields      []fieldDTO `json:"fields"`
}

func (d serviceInputDTO) toDomain() domain.ServiceInput {
	return domain.ServiceInput{
		Name:        d.Name,
		Description: d.Description,
		Active:      d.IsActive == nil || *d.IsActive,
		Fields: mapAll(d.Fields, func(f fieldDTO) domain.FormField {
			return domain.FormField{ID: f.ID, Label: f.Label, Question: f.Question, Type: domain.FieldType(f.Type), Required: f.Required, Options: f.Options}
		}),
	}
}

type answerDTO struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type requestDTO struct {
	ID          uuid.UUID            `json:"id"`
	Number      int                  `json:"number"`
	Status      domain.RequestStatus `json:"status"`
	ServiceID   *uuid.UUID           `json:"service_id"`
	ServiceName string               `json:"service_name"`
	Answers     []answerDTO          `json:"answers"`
	Message     string               `json:"message"`
	Phone       string               `json:"phone"`
	Note        string               `json:"note"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

func toRequest(r domain.Request) requestDTO {
	return requestDTO{
		ID:          r.ID,
		Number:      r.Number,
		Status:      r.Status,
		ServiceID:   r.ServiceID,
		ServiceName: r.ServiceName,
		Answers:     mapAll(r.Answers, func(a domain.Answer) answerDTO { return answerDTO(a) }),
		Message:     r.Message,
		Phone:       r.Phone,
		Note:        r.Note,
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

type requestPageDTO struct {
	Items  []requestWithClientDTO       `json:"items"`
	Total  int                          `json:"total"`
	Counts map[domain.RequestStatus]int `json:"counts"`
}

func toRequestPage(p usecases.RequestPage) requestPageDTO {
	counts := map[domain.RequestStatus]int{
		domain.StatusNew: 0, domain.StatusInProgress: 0, domain.StatusClosed: 0, domain.StatusRejected: 0, domain.StatusCancelled: 0,
	}
	for status, n := range p.Counts {
		counts[status] = n
	}
	return requestPageDTO{Items: mapAll(p.Items, toRequestWithClient), Total: p.Total, Counts: counts}
}

type requestInputDTO struct {
	ServiceID   *uuid.UUID  `json:"service_id"`
	ServiceName string      `json:"service_name"`
	Answers     []answerDTO `json:"answers"`
	Message     string      `json:"message"`
	Phone       string      `json:"phone"`
}

func (d requestInputDTO) toDomain() domain.RequestInput {
	return domain.RequestInput{
		ServiceID:   d.ServiceID,
		ServiceName: d.ServiceName,
		Answers:     mapAll(d.Answers, func(a answerDTO) domain.Answer { return domain.Answer(a) }),
		Message:     d.Message,
		Phone:       d.Phone,
	}
}

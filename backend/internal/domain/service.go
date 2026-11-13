package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type FieldType string

const (
	FieldText   FieldType = "text"
	FieldNumber FieldType = "number"
	FieldChoice FieldType = "choice"
)

const (
	MaxServices        = 30
	MaxServiceFields   = 15
	MaxFieldOptions    = 10
	MaxAnswerLength    = 500
	maxFieldLabel      = 60
	maxFieldQuestion   = 300
	maxFieldOption     = 40
	maxServiceName     = 60
	maxServiceDescribe = 500
)

// FormField is one question of a service form. It is stored as JSON inside the service row.
type FormField struct {
	ID       string    `json:"id"`
	Label    string    `json:"label"`
	Question string    `json:"question"`
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
	Options  []string  `json:"options,omitempty"`
}

// Prompt is the text the bot sends to ask for this field.
func (f FormField) Prompt() string {
	if f.Question != "" {
		return f.Question
	}
	return "Укажите: " + lowerFirst(f.Label)
}

// ParseAnswer validates a raw client (or LLM) answer and returns its normalized form.
func (f FormField) ParseAnswer(raw string) (string, bool) {
	raw = strings.Join(strings.Fields(raw), " ")
	if raw == "" {
		return "", false
	}
	switch f.Type {
	case FieldNumber:
		digits := strings.ReplaceAll(strings.ReplaceAll(raw, " ", ""), ",", ".")
		n, err := strconv.ParseFloat(digits, 64)
		if err != nil || n < 0 || n > 1e12 {
			return "", false
		}
		return strconv.FormatFloat(n, 'f', -1, 64), true
	case FieldChoice:
		for _, opt := range f.Options {
			if strings.EqualFold(opt, raw) {
				return opt, true
			}
		}
		if i, err := strconv.Atoi(raw); err == nil && i >= 1 && i <= len(f.Options) {
			return f.Options[i-1], true
		}
		return "", false
	default:
		if utf8.RuneCountInString(raw) > MaxAnswerLength {
			raw = string([]rune(raw)[:MaxAnswerLength])
		}
		return raw, true
	}
}

type Service struct {
	ID            uuid.UUID
	AutoserviceID uuid.UUID
	Name          string
	Description   string
	Active        bool
	Position      int
	Fields        []FormField
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (s Service) Field(id string) (FormField, bool) {
	for _, f := range s.Fields {
		if f.ID == id {
			return f, true
		}
	}
	return FormField{}, false
}

type ServiceInput struct {
	Name        string
	Description string
	Active      bool
	Fields      []FormField
}

var fieldIDRe = regexp.MustCompile(`^[a-z0-9]{4,16}$`)

func (in ServiceInput) Normalize() (ServiceInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if err := firstError(
		validateRequiredText("name", in.Name, maxServiceName),
		validateOptionalText("description", in.Description, maxServiceDescribe),
	); err != nil {
		return in, err
	}
	if len(in.Fields) > MaxServiceFields {
		return in, invalid("fields", fmt.Sprintf("must have at most %d items", MaxServiceFields))
	}
	fields := make([]FormField, 0, len(in.Fields))
	seen := map[string]bool{}
	for i, f := range in.Fields {
		f, err := normalizeField(f)
		if err != nil {
			return in, invalid(fmt.Sprintf("fields[%d]", i), err.Error())
		}
		if !fieldIDRe.MatchString(f.ID) || seen[f.ID] {
			f.ID = newFieldID()
		}
		seen[f.ID] = true
		fields = append(fields, f)
	}
	in.Fields = fields
	return in, nil
}

func normalizeField(f FormField) (FormField, error) {
	f.Label = strings.TrimSpace(f.Label)
	f.Question = strings.TrimSpace(f.Question)
	if f.Label == "" || utf8.RuneCountInString(f.Label) > maxFieldLabel {
		return f, fmt.Errorf("label must be 1-%d characters", maxFieldLabel)
	}
	if utf8.RuneCountInString(f.Question) > maxFieldQuestion {
		return f, fmt.Errorf("question must be at most %d characters", maxFieldQuestion)
	}
	switch f.Type {
	case FieldText, FieldNumber:
		f.Options = nil
	case FieldChoice:
		options := make([]string, 0, len(f.Options))
		seen := map[string]bool{}
		for _, opt := range f.Options {
			opt = strings.Join(strings.Fields(opt), " ")
			key := strings.ToLower(opt)
			if opt == "" || seen[key] {
				continue
			}
			if utf8.RuneCountInString(opt) > maxFieldOption {
				return f, fmt.Errorf("option must be at most %d characters", maxFieldOption)
			}
			seen[key] = true
			options = append(options, opt)
		}
		if len(options) < 2 || len(options) > MaxFieldOptions {
			return f, fmt.Errorf("choice must have 2-%d options", MaxFieldOptions)
		}
		f.Options = options
	default:
		return f, fmt.Errorf("type must be text, number or choice")
	}
	return f, nil
}

func newFieldID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return strings.ToLower(string(r)) + s[size:]
}

// DefaultServices is the starter catalog every new autoservice gets,
// so the bot can take requests right after the token is connected.
func DefaultServices() []ServiceInput {
	car := FormField{Label: "Автомобиль", Question: "Какой у вас автомобиль? Напишите марку, модель и год выпуска.", Type: FieldText, Required: true}
	when := FormField{Label: "Удобное время", Question: "Когда вам было бы удобно приехать? Точное время мастер согласует с вами по телефону.", Type: FieldText}
	services := []ServiceInput{
		{
			Name:        "Диагностика",
			Description: "Клиент описывает неисправность или симптом и не знает причину: стук, шум, вибрация, горит индикатор, машина не заводится.",
			Active:      true,
			Fields: []FormField{
				car,
				{Label: "Что случилось", Question: "Опишите, что беспокоит в автомобиле.", Type: FieldText, Required: true},
				when,
			},
		},
		{
			Name:        "Техобслуживание и замена масла",
			Description: "Плановое ТО, замена масла, фильтров, свечей, тормозной и охлаждающей жидкости, колодок.",
			Active:      true,
			Fields: []FormField{
				car,
				{Label: "Что нужно сделать", Question: "Что нужно сделать? Например: замена масла и фильтров, плановое ТО.", Type: FieldText, Required: true},
				when,
			},
		},
		{
			Name:        "Ремонт",
			Description: "Клиент знает, что нужно отремонтировать или заменить: подвеска, тормоза, двигатель, коробка передач, электрика, кондиционер.",
			Active:      true,
			Fields: []FormField{
				car,
				{Label: "Что нужно отремонтировать", Question: "Что нужно отремонтировать или заменить?", Type: FieldText, Required: true},
				when,
			},
		},
		{
			Name:        "Шиномонтаж",
			Description: "Сезонная смена шин, балансировка, ремонт проколов, правка дисков.",
			Active:      true,
			Fields: []FormField{
				car,
				{Label: "Вид работ", Question: "Что нужно сделать?", Type: FieldChoice, Required: true,
					Options: []string{"Сезонная смена шин", "Ремонт прокола", "Балансировка", "Другое"}},
				{Label: "Диаметр колёс", Question: "Какой диаметр колёс? Например, R16.", Type: FieldText},
				when,
			},
		},
		{
			Name:        "Вопрос мастеру",
			Description: "Всё, что не подходит под остальные услуги: вопросы о ценах, сроках, наличии запчастей, графике работы.",
			Active:      true,
			Fields: []FormField{
				{Label: "Вопрос", Question: "Опишите ваш вопрос — мастер свяжется с вами и ответит.", Type: FieldText, Required: true},
			},
		},
	}
	// Normalize gives every field its ID.
	for i, svc := range services {
		services[i], _ = svc.Normalize()
	}
	return services
}

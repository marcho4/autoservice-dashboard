// Package bot is the built-in Telegram bot of an autoservice.
//
// A client describes the problem in free form; one LLM call picks the service and pre-fills its form.
// Everything after that is a plain state machine: ask the missing fields, ask the phone, show the summary, submit on confirmation.
package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

// Backend is what the bot needs from the application.
type Backend interface {
	Autoservice(ctx context.Context, autoserviceID uuid.UUID) (domain.Autoservice, error)
	ActiveServices(ctx context.Context, autoserviceID uuid.UUID) ([]domain.Service, error)
	FindClient(ctx context.Context, autoserviceID uuid.UUID, telegramID int64) (domain.Client, error)
	CreateRequest(ctx context.Context, autoserviceID uuid.UUID, who domain.ClientIdentity, in domain.RequestInput) (domain.Request, error)
	ListClientRequests(ctx context.Context, autoserviceID uuid.UUID, telegramID int64) ([]domain.Request, error)
	CancelClientRequest(ctx context.Context, autoserviceID uuid.UUID, telegramID int64, requestID uuid.UUID) (domain.Request, error)
	LinkRecipient(ctx context.Context, autoserviceID uuid.UUID, code string, chatID int64, name string) error
	Recipients(ctx context.Context, autoserviceID uuid.UUID) ([]domain.NotifyRecipient, error)
}

// StateStore persists the dialog state of a chat as an opaque document.
// A state left untouched for a day is not loaded any more, and DeleteExpired removes it.
type StateStore interface {
	Load(ctx context.Context, autoserviceID uuid.UUID, chatID int64) ([]byte, error)
	Save(ctx context.Context, autoserviceID uuid.UUID, chatID int64, state []byte) error
	DeleteExpired(ctx context.Context) error
}

type ExtractInput struct {
	Autoservice string
	Services    []domain.Service
	Message     string
}

// Extraction is the LLM's reading of a free-form message. It is untrusted: the engine validates every value.
type Extraction struct {
	ServiceID uuid.UUID
	Values    map[string]string // form field ID -> raw value
	Phone     string
}

// Extractor picks a service for a free-form client message and pulls out the form fields the client already mentioned.
type Extractor interface {
	Extract(ctx context.Context, in ExtractInput) (Extraction, error)
}

type Incoming struct {
	ChatID int64
	From   domain.ClientIdentity
	// Exactly one of Text, Contact and Callback is set; none of them means an unsupported message such as a photo.
	Text     string
	Contact  string
	Callback string
	// MessageText is the text of the message whose inline button was pressed.
	MessageText string
}

type Button struct {
	Text string
	Data string
	URL  string
}

type ReplyButton struct {
	Text           string
	RequestContact bool
}

// Outgoing is a message to send; Text is Telegram HTML.
type Outgoing struct {
	ChatID         int64
	Text           string
	Buttons        [][]Button
	Keyboard       [][]ReplyButton
	RemoveKeyboard bool
}

type Result struct {
	Messages []Outgoing
	// The rest applies to a pressed inline button: a toast, and what to do with the message that carried the button.
	Toast        string
	EditText     string
	ClearButtons bool
}

type Engine struct {
	backend   Backend
	state     StateStore
	extractor Extractor
	appURL    string
	log       *slog.Logger
	now       func() time.Time

	// LLM budget of each autoservice. In memory is enough: only the leader runner handles updates.
	llmMu    sync.Mutex
	llmUsage map[uuid.UUID]llmUsage
}

type llmUsage struct {
	since time.Time
	calls int
}

// NewEngine builds the dialog engine. extractor may be nil: the client then picks the service with buttons.
// appURL, when it is an https URL, adds an "open in CRM" button to master notifications.
func NewEngine(backend Backend, state StateStore, extractor Extractor, appURL string, log *slog.Logger) *Engine {
	return &Engine{backend: backend, state: state, extractor: extractor, appURL: strings.TrimRight(appURL, "/"), log: log, now: time.Now,
		llmUsage: map[uuid.UUID]llmUsage{}}
}

type step string

const (
	stepIdle    step = ""
	stepService step = "service" // waiting for a service: a button or another free-form message
	stepField   step = "field"   // waiting for the answer to the Current form field
	stepPhone   step = "phone"
	stepConfirm step = "confirm" // the summary is shown
	stepEdit    step = "edit"    // the "what to change" menu is shown
)

type conversation struct {
	Step      step              `json:"step,omitempty"`
	DraftID   uuid.UUID         `json:"draft_id"` // the future request ID: binds the summary buttons, makes the submit idempotent
	ServiceID uuid.UUID         `json:"service_id"`
	Message   string            `json:"message,omitempty"`
	Values    map[string]string `json:"values,omitempty"`
	Skipped   map[string]bool   `json:"skipped,omitempty"`
	Phone     string            `json:"phone,omitempty"`
	Current   string            `json:"current,omitempty"`
	// LLM budget of the chat; it survives resets so that restarting the dialog does not refill it.
	LLMCalls int   `json:"llm_calls,omitempty"`
	LLMSince int64 `json:"llm_since,omitempty"`
}

const (
	maxMessageLength       = 2000
	llmWindow              = time.Hour
	llmCallsPerChat        = 20
	llmCallsPerAutoservice = 300
	maxListedOrders        = 5

	// Telegram rejects messages over 4096 characters. Long answers are shortened in the summary and in the
	// master notification so that even a full form of maximum-length answers fits; the CRM keeps them whole.
	shownAnswerLength  = 150
	shownMessageLength = 300
)

const (
	cbService   = "svc"
	cbOption    = "opt"
	cbSkip      = "skip"
	cbConfirm   = "confirm"
	cbEdit      = "edit"
	cbEditField = "ef"
	cbBack      = "back"
	cbCancel    = "cancel"
	cbCancelReq = "rcancel"
	cbOther     = "other" // another service instead of the one the LLM picked

	editPhone   = "phone"
	editService = "service"
)

// Handle processes one update of a chat and returns what to send back. Updates of one chat must be handled sequentially.
func (e *Engine) Handle(ctx context.Context, autoserviceID uuid.UUID, in Incoming) (Result, error) {
	var st conversation
	raw, err := e.state.Load(ctx, autoserviceID, in.ChatID)
	if err != nil {
		return Result{}, fmt.Errorf("load conversation: %w", err)
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &st); err != nil {
			e.log.Warn("dropping unreadable conversation state", "chat_id", in.ChatID, "error", err)
			st = conversation{}
		}
	}

	t := &turn{e: e, ctx: ctx, autoserviceID: autoserviceID, in: in, st: st}
	if err := t.dispatch(); err != nil {
		return Result{}, err
	}

	raw, _ = json.Marshal(t.st)
	if err := e.state.Save(ctx, autoserviceID, in.ChatID, raw); err != nil {
		return Result{}, fmt.Errorf("save conversation: %w", err)
	}
	return t.res, nil
}

// turn is the handling of a single update.
type turn struct {
	e             *Engine
	ctx           context.Context
	autoserviceID uuid.UUID
	in            Incoming
	st            conversation
	res           Result
}

func (t *turn) say(text string) *Outgoing {
	t.res.Messages = append(t.res.Messages, Outgoing{ChatID: t.in.ChatID, Text: text})
	return &t.res.Messages[len(t.res.Messages)-1]
}

func (t *turn) reset() {
	t.st = conversation{LLMCalls: t.st.LLMCalls, LLMSince: t.st.LLMSince}
}

func (t *turn) dispatch() error {
	switch {
	case t.in.Callback != "":
		return t.onCallback()
	case strings.HasPrefix(t.in.Text, "/"):
		return t.onCommand()
	case t.in.Contact != "":
		if t.st.Step == stepPhone {
			return t.answerPhone(t.in.Contact)
		}
		t.say("Спасибо! Номер понадобится при оформлении заявки. Напишите, чем могу помочь.").RemoveKeyboard = true
		return nil
	case strings.TrimSpace(t.in.Text) != "":
		return t.onText(strings.TrimSpace(t.in.Text))
	default:
		return t.onUnsupported()
	}
}

// onUnsupported answers a photo, a voice message and the like, and repeats what the bot is waiting for.
func (t *turn) onUnsupported() error {
	const text = "Голосовые и фото я пока не принимаю — напишите, пожалуйста, текстом."
	switch t.st.Step {
	case stepField:
		svc, ok, err := t.service()
		if !ok {
			return err
		}
		if f, ok := svc.Field(t.st.Current); ok {
			t.askField(f, text)
			return nil
		}
		return t.advance(svc, text)
	case stepPhone:
		return t.askPhone(text)
	}
	t.say(text)
	return nil
}

func (t *turn) onCommand() error {
	cmd, payload, _ := strings.Cut(t.in.Text, " ")
	cmd, _, _ = strings.Cut(cmd, "@")
	switch cmd {
	case "/start":
		if code, ok := domain.MasterLinkCode(strings.TrimSpace(payload)); ok {
			return t.linkMaster(code)
		}
		t.reset()
		t.say("Здравствуйте! Чем могу помочь?").RemoveKeyboard = true
	case "/cancel":
		if t.st.Step == stepIdle {
			t.say("Сейчас нет незавершённой заявки. Напишите, чем могу помочь.").RemoveKeyboard = true
			return nil
		}
		t.cancelDraft()
	case "/my":
		return t.listRequests()
	default:
		t.say("Напишите своими словами, что нужно сделать с автомобилем, — я оформлю заявку и передам её мастеру.\n\n" +
			"/my — ваши заявки\n/cancel — отменить заполнение заявки")
	}
	return nil
}

func (t *turn) cancelDraft() {
	t.reset()
	t.say("Хорошо, заявку не отправляю. Если понадобится помощь — просто напишите.").RemoveKeyboard = true
}

func (t *turn) linkMaster(code string) error {
	name := t.in.From.Name
	if name == "" && t.in.From.TelegramUsername != "" {
		name = "@" + t.in.From.TelegramUsername
	}
	err := t.e.backend.LinkRecipient(t.ctx, t.autoserviceID, code, t.in.ChatID, name)
	if errors.Is(err, domain.ErrNotFound) {
		t.say("Эта ссылка уже использована или устарела. Откройте раздел «Автосервис и бот» в личном кабинете и перейдите по новой ссылке.")
		return nil
	}
	if err != nil {
		return err
	}
	a, err := t.e.backend.Autoservice(t.ctx, t.autoserviceID)
	if err != nil {
		return err
	}
	t.say(fmt.Sprintf("Готово! Сюда будут приходить новые заявки автосервиса «%s».", esc(a.Name)))
	return nil
}

func (t *turn) onText(text string) error {
	if (t.st.Step == stepField || t.st.Step == stepPhone) && isCancel(text) {
		t.cancelDraft()
		return nil
	}
	switch t.st.Step {
	case stepField:
		return t.answerField(text)
	case stepPhone:
		return t.answerPhone(text)
	case stepConfirm:
		if isYes(text) {
			return t.submit()
		}
		return t.showSummary("Чтобы отправить заявку, нажмите «Отправить». Если нужно что-то поправить — «Изменить».")
	case stepEdit:
		return t.showEditMenu()
	default:
		if reply, ok := smallTalk[shortWord(text)]; ok {
			t.say(reply)
			return nil
		}
		return t.classify(text)
	}
}

// classify is the only place the LLM is used: it turns a free-form message into a service and pre-filled form fields.
func (t *turn) classify(text string) error {
	services, err := t.e.backend.ActiveServices(t.ctx, t.autoserviceID)
	if err != nil {
		return err
	}
	if len(services) == 0 {
		return t.noServices()
	}
	// Later messages add to the first one instead of replacing it: the first usually describes the problem best.
	if t.st.Message != "" {
		text = t.st.Message + "\n" + text
	}
	t.st.Message = truncate(text, maxMessageLength)

	var ext Extraction
	understood := false
	if t.e.extractor != nil && t.spendLLMCall() {
		a, err := t.e.backend.Autoservice(t.ctx, t.autoserviceID)
		if err != nil {
			return err
		}
		ext, err = t.e.extractor.Extract(t.ctx, ExtractInput{Autoservice: a.Name, Services: services, Message: t.st.Message})
		if err != nil {
			// The dialog must go on without the LLM: the client picks the service by hand.
			t.e.log.Warn("llm extraction failed", "autoservice_id", t.autoserviceID, "error", err)
		} else {
			understood = true
		}
	}
	for _, svc := range services {
		if understood && svc.ID == ext.ServiceID {
			if err := t.startForm(svc, ext); err != nil {
				return err
			}
			// The LLM may have guessed wrong: the way out is right under the first question.
			if t.st.Step == stepField && len(services) > 1 {
				msg := &t.res.Messages[len(t.res.Messages)-1]
				msg.Buttons = append(msg.Buttons, []Button{{Text: "Другая услуга", Data: cbOther + ":" + t.st.DraftID.String()}})
			}
			return nil
		}
	}
	if !understood && len(services) == 1 {
		return t.startForm(services[0], Extraction{})
	}
	t.askService(services, "Подскажите, какая услуга вам нужна? Выберите из списка или опишите задачу подробнее.")
	return nil
}

func (t *turn) spendLLMCall() bool {
	now := t.e.now()
	if now.Sub(time.Unix(t.st.LLMSince, 0)) > llmWindow {
		t.st.LLMSince, t.st.LLMCalls = now.Unix(), 0
	}
	if t.st.LLMCalls >= llmCallsPerChat || !t.e.spendAutoserviceLLMCall(t.autoserviceID, now) {
		return false
	}
	t.st.LLMCalls++
	return true
}

func (e *Engine) spendAutoserviceLLMCall(autoserviceID uuid.UUID, now time.Time) bool {
	e.llmMu.Lock()
	defer e.llmMu.Unlock()
	u := e.llmUsage[autoserviceID]
	if now.Sub(u.since) > llmWindow {
		u = llmUsage{since: now}
	}
	if u.calls >= llmCallsPerAutoservice {
		return false
	}
	u.calls++
	e.llmUsage[autoserviceID] = u
	return true
}

func (t *turn) noServices() error {
	t.reset()
	a, err := t.e.backend.Autoservice(t.ctx, t.autoserviceID)
	if err != nil {
		return err
	}
	text := "К сожалению, приём заявок через бота сейчас не настроен."
	if a.Phone != "" {
		text += " Позвоните нам: " + esc(a.Phone)
	}
	t.say(text)
	return nil
}

func (t *turn) askService(services []domain.Service, text string) {
	t.st.Step = stepService
	msg := t.say(text)
	for _, svc := range services {
		msg.Buttons = append(msg.Buttons, []Button{{Text: svc.Name, Data: cbService + ":" + svc.ID.String()}})
	}
}

func (t *turn) startForm(svc domain.Service, ext Extraction) error {
	t.st.DraftID = uuid.New()
	t.st.ServiceID = svc.ID
	t.st.Values = map[string]string{}
	t.st.Skipped = nil
	t.st.Current = ""
	for id, raw := range ext.Values {
		if f, ok := svc.Field(id); ok {
			if v, ok := f.ParseAnswer(raw); ok {
				t.st.Values[id] = v
			}
		}
	}
	if t.st.Phone == "" && ext.Phone != "" {
		if phone, err := domain.NormalizePhone(ext.Phone); err == nil {
			t.st.Phone = phone
		}
	}
	return t.advance(svc, fmt.Sprintf("Оформляю заявку: <b>%s</b>.", esc(svc.Name)))
}

// service returns the service of the draft, or false if it was deleted or switched off meanwhile.
func (t *turn) service() (domain.Service, bool, error) {
	services, err := t.e.backend.ActiveServices(t.ctx, t.autoserviceID)
	if err != nil {
		return domain.Service{}, false, err
	}
	for _, svc := range services {
		if svc.ID == t.st.ServiceID {
			return svc, true, nil
		}
	}
	if len(services) == 0 {
		return domain.Service{}, false, t.noServices()
	}
	t.askService(services, "К сожалению, эта услуга больше недоступна. Выберите, пожалуйста, другую:")
	return domain.Service{}, false, nil
}

func (t *turn) nextMissing(svc domain.Service) (domain.FormField, bool) {
	for _, f := range svc.Fields {
		if t.st.Values[f.ID] == "" && (f.Required || !t.st.Skipped[f.ID]) {
			return f, true
		}
	}
	return domain.FormField{}, false
}

// advance moves the dialog to the next thing the request still lacks: a form field, the phone, or the confirmation.
func (t *turn) advance(svc domain.Service, intro string) error {
	if f, ok := t.nextMissing(svc); ok {
		t.askField(f, intro)
		return nil
	}
	if t.st.Phone == "" {
		return t.askPhone(intro)
	}
	return t.showSummaryOf(svc, intro)
}

func (t *turn) askField(f domain.FormField, intro string) {
	t.st.Step = stepField
	t.st.Current = f.ID
	msg := t.say(join(intro, esc(f.Prompt())))
	for i, opt := range f.Options {
		msg.Buttons = append(msg.Buttons, []Button{{Text: opt, Data: cbOption + ":" + f.ID + ":" + strconv.Itoa(i)}})
	}
	if !f.Required {
		msg.Buttons = append(msg.Buttons, []Button{{Text: "Пропустить", Data: cbSkip + ":" + f.ID}})
	}
}

func (t *turn) answerField(text string) error {
	svc, ok, err := t.service()
	if !ok {
		return err
	}
	f, ok := svc.Field(t.st.Current)
	if !ok {
		return t.advance(svc, "")
	}
	value, ok := f.ParseAnswer(text)
	if !ok {
		hint := "Не получилось разобрать ответ. Попробуйте ещё раз."
		switch f.Type {
		case domain.FieldNumber:
			hint = "Здесь нужно число, например 120000."
		case domain.FieldChoice:
			hint = "Выберите, пожалуйста, один из вариантов."
		}
		t.askField(f, hint)
		return nil
	}
	t.setValue(f, value)
	return t.advance(svc, "")
}

func (t *turn) setValue(f domain.FormField, value string) {
	if t.st.Values == nil {
		t.st.Values = map[string]string{}
	}
	t.st.Values[f.ID] = value
	delete(t.st.Skipped, f.ID)
}

func (t *turn) askPhone(intro string) error {
	t.st.Step = stepPhone
	msg := t.say(join(intro, "Оставьте номер телефона — мастер перезвонит вам. Нажмите кнопку ниже или напишите номер."))
	client, err := t.e.backend.FindClient(t.ctx, t.autoserviceID, t.in.From.TelegramID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if client.Phone != "" {
		msg.Keyboard = append(msg.Keyboard, []ReplyButton{{Text: domain.FormatPhone(client.Phone)}})
	}
	msg.Keyboard = append(msg.Keyboard, []ReplyButton{{Text: "📱 Отправить мой номер", RequestContact: true}})
	return nil
}

func (t *turn) answerPhone(raw string) error {
	phone, err := domain.NormalizePhone(raw)
	if err != nil {
		t.say("Не похоже на номер телефона. Напишите его в формате +7 900 123-45-67 или нажмите «Отправить мой номер».")
		return nil
	}
	t.st.Phone = phone
	t.say("Записал номер " + esc(domain.FormatPhone(phone)) + ".").RemoveKeyboard = true
	svc, ok, err := t.service()
	if !ok {
		return err
	}
	return t.advance(svc, "")
}

func (t *turn) answers(svc domain.Service) []domain.Answer {
	var answers []domain.Answer
	for _, f := range svc.Fields {
		if v := t.st.Values[f.ID]; v != "" {
			answers = append(answers, domain.Answer{Label: f.Label, Value: v})
		}
	}
	return answers
}

func (t *turn) showSummary(intro string) error {
	svc, ok, err := t.service()
	if !ok {
		return err
	}
	return t.advance(svc, intro)
}

func (t *turn) showSummaryOf(svc domain.Service, intro string) error {
	t.st.Step = stepConfirm
	t.st.Current = ""
	var b strings.Builder
	b.WriteString("<b>Проверьте заявку</b>\n\n")
	fmt.Fprintf(&b, "Услуга: <b>%s</b>\n", esc(svc.Name))
	for _, a := range t.answers(svc) {
		fmt.Fprintf(&b, "%s: %s\n", esc(a.Label), esc(truncate(a.Value, shownAnswerLength)))
	}
	fmt.Fprintf(&b, "Телефон: %s\n\nВсё верно?", esc(domain.FormatPhone(t.st.Phone)))
	draft := ":" + t.st.DraftID.String()
	t.say(join(intro, b.String())).Buttons = [][]Button{
		{{Text: "✅ Отправить", Data: cbConfirm + draft}},
		{{Text: "✏️ Изменить", Data: cbEdit + draft}, {Text: "❌ Отменить", Data: cbCancel + draft}},
	}
	return nil
}

func (t *turn) showEditMenu() error {
	svc, ok, err := t.service()
	if !ok {
		return err
	}
	t.st.Step = stepEdit
	msg := t.say("Что изменить?")
	for _, f := range svc.Fields {
		msg.Buttons = append(msg.Buttons, []Button{{Text: f.Label, Data: cbEditField + ":" + f.ID}})
	}
	msg.Buttons = append(msg.Buttons,
		[]Button{{Text: "Телефон", Data: cbEditField + ":" + editPhone}},
		[]Button{{Text: "Услугу", Data: cbEditField + ":" + editService}},
		[]Button{{Text: "← Назад", Data: cbBack}},
	)
	return nil
}

func (t *turn) submit() error {
	svc, ok, err := t.service()
	if !ok {
		return err
	}
	// The owner may have added a required field while the client was filling the form.
	if _, missing := t.nextMissing(svc); missing || t.st.Phone == "" {
		return t.advance(svc, "Осталось уточнить ещё кое-что.")
	}
	req, err := t.e.backend.CreateRequest(t.ctx, t.autoserviceID, t.in.From, domain.RequestInput{
		DraftID:     t.st.DraftID,
		ServiceID:   &svc.ID,
		ServiceName: svc.Name,
		Answers:     t.answers(svc),
		Message:     t.st.Message,
		Phone:       t.st.Phone,
	})
	if errors.Is(err, domain.ErrTooManyRequests) {
		// The draft stays: the client can send it with the same button once the limit allows.
		t.res.ClearButtons = false
		t.say(fmt.Sprintf("Вы уже оставили %d заявок за сутки. Мастер свяжется с вами по ним; новую можно будет отправить позже.",
			domain.MaxRequestsPerClientPerDay))
		return nil
	}
	if err != nil {
		return err
	}
	t.reset()
	t.say(fmt.Sprintf("✅ Заявка №%d принята!\n\nМастер свяжется с вами по номеру %s, как только освободится.\n\n"+
		"Ваши заявки — /my. Чтобы оставить ещё одну, просто напишите, что нужно.",
		req.Number, esc(domain.FormatPhone(req.Phone))))
	t.notifyMasters(newRequestNotice(req, t.in.From), req)
	return nil
}

// notifyMasters queues a message to every subscribed master. A failure here must not fail the client's request.
func (t *turn) notifyMasters(text string, req domain.Request) {
	recipients, err := t.e.backend.Recipients(t.ctx, t.autoserviceID)
	if err != nil {
		t.e.log.Error("list notification recipients", "autoservice_id", t.autoserviceID, "error", err)
		return
	}
	for _, r := range recipients {
		msg := Outgoing{ChatID: r.ChatID, Text: text}
		if strings.HasPrefix(t.e.appURL, "https://") {
			msg.Buttons = [][]Button{{{Text: "Открыть в CRM", URL: t.e.appURL + "/requests/" + req.ID.String()}}}
		}
		t.res.Messages = append(t.res.Messages, msg)
	}
}

func newRequestNotice(req domain.Request, who domain.ClientIdentity) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🆕 <b>Новая заявка №%d</b>\n\n", req.Number)
	fmt.Fprintf(&b, "Услуга: <b>%s</b>\n", esc(req.ServiceName))
	for _, a := range req.Answers {
		fmt.Fprintf(&b, "%s: %s\n", esc(a.Label), esc(truncate(a.Value, shownAnswerLength)))
	}
	client := who.Name
	if who.TelegramUsername != "" {
		client = strings.TrimSpace(client + " @" + who.TelegramUsername)
	}
	if client != "" {
		fmt.Fprintf(&b, "\nКлиент: %s", esc(client))
	}
	fmt.Fprintf(&b, "\nТелефон: %s", esc(req.Phone))
	if req.Message != "" {
		fmt.Fprintf(&b, "\n\n<i>Сообщение клиента:</i>\n%s", esc(truncate(req.Message, shownMessageLength)))
	}
	return b.String()
}

var statusForClient = map[domain.RequestStatus]string{
	domain.StatusNew:        "ждёт мастера",
	domain.StatusInProgress: "в работе",
	domain.StatusClosed:     "закрыта",
	domain.StatusRejected:   "отклонена",
	domain.StatusCancelled:  "отменена",
}

func (t *turn) listRequests() error {
	requests, err := t.e.backend.ListClientRequests(t.ctx, t.autoserviceID, t.in.From.TelegramID)
	if err != nil {
		return err
	}
	if len(requests) == 0 {
		t.say("У вас пока нет заявок. Напишите, чем могу помочь.")
		return nil
	}
	requests = requests[:min(len(requests), maxListedOrders)]
	var b strings.Builder
	b.WriteString("<b>Ваши заявки</b>\n")
	var buttons [][]Button
	for _, r := range requests {
		fmt.Fprintf(&b, "\n№%d · %s — %s", r.Number, esc(r.ServiceName), statusForClient[r.Status])
		if domain.CanTransition(r.Status, domain.StatusCancelled, domain.ActorClient) {
			buttons = append(buttons, []Button{{Text: fmt.Sprintf("Отменить №%d", r.Number), Data: cbCancelReq + ":" + r.ID.String()}})
		}
	}
	t.say(b.String()).Buttons = buttons
	return nil
}

func (t *turn) cancelRequest(arg string) error {
	id, err := uuid.Parse(arg)
	if err != nil {
		t.res.Toast = staleButton
		return nil
	}
	req, err := t.e.backend.CancelClientRequest(t.ctx, t.autoserviceID, t.in.From.TelegramID, id)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		t.res.Toast = "Заявка не найдена"
	case errors.Is(err, domain.ErrRequestNotEditable), errors.Is(err, domain.ErrInvalidTransition):
		t.res.Toast = "Мастер уже взял заявку в работу — отменить её можно по телефону"
		t.res.ClearButtons = true
	case err != nil:
		return err
	default:
		t.res.ClearButtons = true
		t.say(fmt.Sprintf("Заявка №%d отменена.", req.Number))
		t.notifyMasters(fmt.Sprintf("❌ Клиент отменил заявку №%d (%s).", req.Number, esc(req.ServiceName)), req)
	}
	return nil
}

const staleButton = "Эта кнопка уже неактуальна"

// picked records the pressed option in the message that carried the buttons, so the chat history stays readable.
func (t *turn) picked(label string) {
	t.res.ClearButtons = true
	if t.in.MessageText != "" {
		t.res.EditText = esc(t.in.MessageText) + "\n\n— " + esc(label)
	}
}

// onCallback handles an inline button. Buttons of earlier steps stay in the chat, so every action checks the current step.
func (t *turn) onCallback() error {
	action, arg, _ := strings.Cut(t.in.Callback, ":")
	if action == cbCancelReq {
		return t.cancelRequest(arg)
	}
	valid := map[string][]step{
		cbService:   {stepIdle, stepService},
		cbOption:    {stepField},
		cbSkip:      {stepField},
		cbConfirm:   {stepConfirm},
		cbEdit:      {stepConfirm},
		cbCancel:    {stepConfirm, stepEdit},
		cbEditField: {stepEdit},
		cbBack:      {stepEdit},
		cbOther:     {stepField, stepPhone, stepConfirm},
	}
	// Buttons of a question belong to its field, buttons of the summary to the draft.
	fresh := true
	switch action {
	case cbOption, cbSkip:
		field, _, _ := strings.Cut(arg, ":")
		fresh = field == t.st.Current
	case cbConfirm, cbEdit, cbCancel, cbOther:
		fresh = arg == t.st.DraftID.String()
	}
	if !fresh || !containsStep(valid[action], t.st.Step) {
		t.res.Toast = staleButton
		return nil
	}

	switch action {
	case cbService:
		services, err := t.e.backend.ActiveServices(t.ctx, t.autoserviceID)
		if err != nil {
			return err
		}
		for _, svc := range services {
			if svc.ID.String() == arg {
				t.picked(svc.Name)
				return t.startForm(svc, Extraction{})
			}
		}
		t.res.Toast = "Эта услуга больше недоступна"
		return nil

	case cbOption, cbSkip:
		svc, ok, err := t.service()
		if !ok {
			t.res.ClearButtons = true
			return err
		}
		f, ok := svc.Field(t.st.Current)
		if !ok {
			t.res.ClearButtons = true
			return t.advance(svc, "")
		}
		if action == cbSkip {
			if f.Required {
				t.res.Toast = staleButton
				return nil
			}
			delete(t.st.Values, f.ID)
			if t.st.Skipped == nil {
				t.st.Skipped = map[string]bool{}
			}
			t.st.Skipped[f.ID] = true
			t.picked("пропущено")
			return t.advance(svc, "")
		}
		_, option, _ := strings.Cut(arg, ":")
		i, err := strconv.Atoi(option)
		if err != nil || i < 0 || i >= len(f.Options) {
			t.res.Toast = staleButton
			return nil
		}
		t.setValue(f, f.Options[i])
		t.picked(f.Options[i])
		return t.advance(svc, "")

	case cbConfirm:
		t.res.ClearButtons = true
		return t.submit()

	case cbEdit:
		t.res.ClearButtons = true
		return t.showEditMenu()

	case cbBack:
		t.res.ClearButtons = true
		return t.showSummary("")

	case cbCancel:
		t.res.ClearButtons = true
		t.cancelDraft()
		return nil

	case cbOther:
		t.res.ClearButtons = true
		services, err := t.e.backend.ActiveServices(t.ctx, t.autoserviceID)
		if err != nil {
			return err
		}
		t.askService(services, "Выберите услугу:")
		return nil

	case cbEditField:
		t.res.ClearButtons = true
		svc, ok, err := t.service()
		if !ok {
			return err
		}
		switch arg {
		case editPhone:
			return t.askPhone("")
		case editService:
			services, err := t.e.backend.ActiveServices(t.ctx, t.autoserviceID)
			if err != nil {
				return err
			}
			t.askService(services, "Выберите услугу:")
			return nil
		}
		f, ok := svc.Field(arg)
		if !ok {
			return t.showSummaryOf(svc, "")
		}
		t.askField(f, "")
		return nil
	}
	return nil
}

func containsStep(steps []step, s step) bool {
	for _, x := range steps {
		if x == s {
			return true
		}
	}
	return false
}

// shortWord normalizes a one-word reply such as "Спасибо!" for matching against fixed lists.
func shortWord(text string) string { return strings.Trim(strings.ToLower(text), " .,!)") }

func isYes(text string) bool {
	switch shortWord(text) {
	case "да", "ага", "отправить", "отправляй", "отправляйте", "верно", "всё верно", "все верно", "ок", "ok", "подтверждаю", "+":
		return true
	}
	return false
}

func isCancel(text string) bool {
	switch shortWord(text) {
	case "отмена", "отменить", "стоп", "передумал", "передумала", "не надо":
		return true
	}
	return false
}

const (
	thanksReply   = "Пожалуйста! Если понадобится помощь — напишите."
	greetingReply = "Здравствуйте! Напишите, что нужно сделать с машиной."
)

// smallTalk answers courtesy messages without the LLM, which would only offer the list of services.
var smallTalk = map[string]string{
	"спасибо": thanksReply, "спс": thanksReply, "благодарю": thanksReply, "ок": thanksReply,
	"хорошо": thanksReply, "понял": thanksReply, "поняла": thanksReply, "👍": thanksReply,
	"привет": greetingReply, "здравствуйте": greetingReply, "добрый день": greetingReply, "добрый вечер": greetingReply,
	"доброе утро": greetingReply, "здрасьте": greetingReply, "hi": greetingReply, "hello": greetingReply,
}

func esc(s string) string { return html.EscapeString(s) }

func join(intro, text string) string {
	if intro == "" {
		return text
	}
	return intro + "\n\n" + text
}

func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	return string([]rune(s)[:maxRunes]) + "…"
}

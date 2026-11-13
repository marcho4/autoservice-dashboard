package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

// memBackend is an in-memory application behind the bot.
type memBackend struct {
	autoservice domain.Autoservice
	services    []domain.Service
	clients     map[int64]domain.Client
	requests    []domain.Request
	recipients  []domain.NotifyRecipient
	linkCode    string
}

func (b *memBackend) Autoservice(context.Context, uuid.UUID) (domain.Autoservice, error) {
	return b.autoservice, nil
}

func (b *memBackend) ActiveServices(context.Context, uuid.UUID) ([]domain.Service, error) {
	var active []domain.Service
	for _, s := range b.services {
		if s.Active {
			active = append(active, s)
		}
	}
	return active, nil
}

func (b *memBackend) FindClient(_ context.Context, _ uuid.UUID, telegramID int64) (domain.Client, error) {
	c, ok := b.clients[telegramID]
	if !ok {
		return domain.Client{}, domain.ErrNotFound
	}
	return c, nil
}

func (b *memBackend) CreateRequest(_ context.Context, _ uuid.UUID, who domain.ClientIdentity, in domain.RequestInput) (domain.Request, error) {
	in, err := in.Normalize()
	if err != nil {
		return domain.Request{}, err
	}
	recent := 0
	for _, r := range b.requests {
		if r.ID == in.DraftID {
			return r, nil
		}
		if time.Since(r.CreatedAt) < 24*time.Hour {
			recent++
		}
	}
	if recent >= domain.MaxRequestsPerClientPerDay {
		return domain.Request{}, domain.ErrTooManyRequests
	}
	b.clients[who.TelegramID] = domain.Client{TelegramID: who.TelegramID, Name: who.Name, Phone: in.Phone}
	r := domain.Request{
		ID: in.DraftID, Number: len(b.requests) + 1, Status: domain.StatusNew, CreatedAt: time.Now(),
		ServiceID: in.ServiceID, ServiceName: in.ServiceName, Answers: in.Answers, Message: in.Message, Phone: in.Phone,
	}
	b.requests = append(b.requests, r)
	return r, nil
}

func (b *memBackend) ListClientRequests(context.Context, uuid.UUID, int64) ([]domain.Request, error) {
	return b.requests, nil
}

func (b *memBackend) CancelClientRequest(_ context.Context, _ uuid.UUID, _ int64, id uuid.UUID) (domain.Request, error) {
	for i, r := range b.requests {
		if r.ID != id {
			continue
		}
		if r.Status != domain.StatusNew {
			return domain.Request{}, domain.ErrRequestNotEditable
		}
		b.requests[i].Status = domain.StatusCancelled
		return b.requests[i], nil
	}
	return domain.Request{}, domain.ErrNotFound
}

func (b *memBackend) LinkRecipient(_ context.Context, _ uuid.UUID, code string, chatID int64, name string) error {
	if code == "" || code != b.linkCode {
		return domain.ErrNotFound
	}
	b.linkCode = "rotated"
	b.recipients = append(b.recipients, domain.NotifyRecipient{ChatID: chatID, Name: name})
	return nil
}

func (b *memBackend) Recipients(context.Context, uuid.UUID) ([]domain.NotifyRecipient, error) {
	return b.recipients, nil
}

type memState map[int64][]byte

func (s memState) Load(_ context.Context, _ uuid.UUID, chatID int64) ([]byte, error) {
	return s[chatID], nil
}

func (s memState) Save(_ context.Context, _ uuid.UUID, chatID int64, state []byte) error {
	s[chatID] = state
	return nil
}

func (memState) DeleteExpired(context.Context) error { return nil }

type extractorFunc func(in ExtractInput) (Extraction, error)

func (f extractorFunc) Extract(_ context.Context, in ExtractInput) (Extraction, error) { return f(in) }

var (
	oilService = domain.Service{
		ID: uuid.New(), Name: "Замена масла", Active: true,
		Fields: []domain.FormField{
			{ID: "car", Label: "Автомобиль", Question: "Какой у вас автомобиль?", Type: domain.FieldText, Required: true},
			{ID: "km", Label: "Пробег", Question: "Какой пробег?", Type: domain.FieldNumber, Required: true},
			{ID: "when", Label: "Удобное время", Question: "Когда вам удобно приехать?", Type: domain.FieldText},
		},
	}
	tireService = domain.Service{
		ID: uuid.New(), Name: "Шиномонтаж", Active: true,
		Fields: []domain.FormField{
			{ID: "work", Label: "Вид работ", Question: "Что нужно сделать?", Type: domain.FieldChoice, Required: true,
				Options: []string{"Смена шин", "Балансировка"}},
		},
	}
)

const (
	clientChat = int64(42)
	masterChat = int64(7)
)

// dialog drives the engine as one Telegram user and keeps the last reply for assertions.
type dialog struct {
	t       *testing.T
	engine  *Engine
	backend *memBackend
	from    domain.ClientIdentity
	last    Result
	llm     int
}

func newDialog(t *testing.T, extract func(in ExtractInput) (Extraction, error)) *dialog {
	d := &dialog{t: t, from: domain.ClientIdentity{TelegramID: clientChat, TelegramUsername: "petya", Name: "Пётр"}}
	d.backend = &memBackend{
		autoservice: domain.Autoservice{Name: "Гараж №1", Phone: "+7 900 000-00-00"},
		services:    []domain.Service{oilService, tireService},
		clients:     map[int64]domain.Client{},
		linkCode:    "secret",
	}
	var extractor Extractor
	if extract != nil {
		extractor = extractorFunc(func(in ExtractInput) (Extraction, error) {
			d.llm++
			return extract(in)
		})
	}
	d.engine = NewEngine(d.backend, memState{}, extractor, "https://crm.example.com", slog.New(slog.NewTextHandler(io.Discard, nil)))
	return d
}

func (d *dialog) send(in Incoming) *dialog {
	d.t.Helper()
	in.ChatID, in.From = d.from.TelegramID, d.from
	res, err := d.engine.Handle(context.Background(), uuid.Nil, in)
	if err != nil {
		d.t.Fatalf("handle %+v: %v", in, err)
	}
	d.last = res
	return d
}

func (d *dialog) text(s string) *dialog { d.t.Helper(); return d.send(Incoming{Text: s}) }

// click presses the inline button with the given label on the last bot message.
func (d *dialog) click(label string) *dialog {
	d.t.Helper()
	for _, m := range d.last.Messages {
		for _, row := range m.Buttons {
			for _, b := range row {
				if b.Text == label {
					return d.send(Incoming{Callback: b.Data, MessageText: m.Text})
				}
			}
		}
	}
	d.t.Fatalf("no button %q in %+v", label, d.last.Messages)
	return d
}

// replies returns the texts sent to the client's own chat.
func (d *dialog) replies() string {
	var texts []string
	for _, m := range d.last.Messages {
		if m.ChatID == d.from.TelegramID {
			texts = append(texts, m.Text)
		}
	}
	return strings.Join(texts, "\n---\n")
}

func (d *dialog) expect(parts ...string) *dialog {
	d.t.Helper()
	for _, part := range parts {
		if !strings.Contains(d.replies(), part) {
			d.t.Fatalf("reply must contain %q, got:\n%s", part, d.replies())
		}
	}
	return d
}

func (d *dialog) buttons() []string {
	var labels []string
	for _, m := range d.last.Messages {
		for _, row := range m.Buttons {
			for _, b := range row {
				labels = append(labels, b.Text)
			}
		}
		for _, row := range m.Keyboard {
			for _, b := range row {
				labels = append(labels, b.Text)
			}
		}
	}
	return labels
}

func (d *dialog) expectButtons(want ...string) *dialog {
	d.t.Helper()
	if got := d.buttons(); strings.Join(got, "|") != strings.Join(want, "|") {
		d.t.Fatalf("buttons = %q, want %q", got, want)
	}
	return d
}

func TestFreeFormRequestIsPrefilledByLLM(t *testing.T) {
	d := newDialog(t, func(in ExtractInput) (Extraction, error) {
		if in.Autoservice != "Гараж №1" || len(in.Services) != 2 {
			t.Errorf("unexpected extraction input: %+v", in)
		}
		return Extraction{ServiceID: oilService.ID, Values: map[string]string{"car": "Toyota Camry 2015"}}, nil
	})
	d.backend.recipients = []domain.NotifyRecipient{{ChatID: masterChat, Name: "Мастер"}}

	d.text("/start").expect("Здравствуйте! Чем могу помочь?")
	// The car is already known from the message, so the bot goes straight to the first missing field.
	d.text("Хочу поменять масло на камри 2015 года").expect("Оформляю заявку: <b>Замена масла</b>", "Какой пробег?")
	d.text("сто двадцать тысяч").expect("нужно число", "Какой пробег?")
	d.text("120 000").expect("Когда вам удобно приехать?").expectButtons("Пропустить")
	d.click("Пропустить").expect("Оставьте номер телефона").expectButtons("📱 Отправить мой номер")
	if !strings.HasSuffix(d.last.EditText, "— пропущено") {
		t.Errorf("the skipped question must be marked in the chat: %q", d.last.EditText)
	}
	d.text("позвоните мне").expect("Не похоже на номер телефона")
	d.send(Incoming{Contact: "79001112233"}).expect(
		"Записал номер +7 900 111-22-33", "Проверьте заявку", "Услуга: <b>Замена масла</b>",
		"Автомобиль: Toyota Camry 2015", "Пробег: 120000", "Телефон: +7 900 111-22-33")
	if strings.Contains(d.replies(), "Удобное время") {
		t.Errorf("skipped field must not be in the summary:\n%s", d.replies())
	}
	d.expectButtons("✅ Отправить", "✏️ Изменить", "❌ Отменить")

	if len(d.backend.requests) != 0 {
		t.Fatal("request must not be created before the client confirms")
	}
	d.click("✅ Отправить").expect("Заявка №1 принята", "+7 900 111-22-33")

	if d.llm != 1 {
		t.Errorf("LLM must be called once per request, got %d calls", d.llm)
	}
	if len(d.backend.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(d.backend.requests))
	}
	r := d.backend.requests[0]
	wantAnswers := []domain.Answer{{Label: "Автомобиль", Value: "Toyota Camry 2015"}, {Label: "Пробег", Value: "120000"}}
	if *r.ServiceID != oilService.ID || r.ServiceName != "Замена масла" || r.Phone != "+79001112233" ||
		r.Message != "Хочу поменять масло на камри 2015 года" || len(r.Answers) != 2 || r.Answers[0] != wantAnswers[0] || r.Answers[1] != wantAnswers[1] {
		t.Errorf("unexpected request: %+v", r)
	}

	// The master gets the whole request with a link to the CRM.
	var notice *Outgoing
	for i, m := range d.last.Messages {
		if m.ChatID == masterChat {
			notice = &d.last.Messages[i]
		}
	}
	if notice == nil {
		t.Fatal("master was not notified")
	}
	for _, part := range []string{"Новая заявка №1", "Замена масла", "Toyota Camry 2015", "Пётр @petya", "+79001112233", "Хочу поменять масло"} {
		if !strings.Contains(notice.Text, part) {
			t.Errorf("notification must contain %q:\n%s", part, notice.Text)
		}
	}
	if len(notice.Buttons) != 1 || notice.Buttons[0][0].URL != "https://crm.example.com/requests/"+r.ID.String() {
		t.Errorf("notification must link to the request: %+v", notice.Buttons)
	}

	// The dialog is over: a late press on the old button does nothing.
	d.send(Incoming{Callback: cbConfirm})
	if d.last.Toast != staleButton || len(d.backend.requests) != 1 {
		t.Errorf("stale confirm must be ignored: %+v", d.last)
	}
}

func TestUnknownIntentFallsBackToServiceButtons(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) { return Extraction{}, nil })

	d.text("посмотрите машину").expect("какая услуга вам нужна").expectButtons("Замена масла", "Шиномонтаж")
	// The client may also describe the task again instead of pressing a button.
	d.text("ну мне бы машину посмотреть").expectButtons("Замена масла", "Шиномонтаж")
	if d.llm != 2 {
		t.Errorf("each free-form message before the service is known goes to the LLM, got %d calls", d.llm)
	}
	d.click("Шиномонтаж").expect("Оформляю заявку: <b>Шиномонтаж</b>", "Что нужно сделать?").
		expectButtons("Смена шин", "Балансировка")
	d.text("покраска").expect("один из вариантов")
	d.click("Балансировка").expect("Оставьте номер телефона")
	if !strings.HasSuffix(d.last.EditText, "— Балансировка") {
		t.Errorf("the chosen option must be written into the question: %q", d.last.EditText)
	}
	d.text("8 900 111-22-33").expect("Вид работ: Балансировка")
	d.text("да").expect("Заявка №1 принята")
	if d.llm != 2 {
		t.Errorf("no LLM calls after the service is chosen, got %d", d.llm)
	}
}

func TestLLMFailureDoesNotBlockTheClient(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) { return Extraction{}, errors.New("llm is down") })
	d.text("нужно поменять масло").expect("какая услуга вам нужна").expectButtons("Замена масла", "Шиномонтаж")
	d.click("Замена масла").expect("Какой у вас автомобиль?")
}

func TestWithoutLLMSingleServiceStartsRightAway(t *testing.T) {
	d := newDialog(t, nil)
	d.backend.services = []domain.Service{tireService}
	d.text("здравствуйте, нужна помощь").expect("Оформляю заявку: <b>Шиномонтаж</b>", "Что нужно сделать?")
}

func TestLLMOutputIsValidated(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) {
		return Extraction{
			ServiceID: oilService.ID,
			Values:    map[string]string{"car": " BMW  X5 ", "km": "много", "ghost": "x", "work": "Балансировка"},
			Phone:     "not a phone",
		}, nil
	})
	// A non-numeric mileage, an unknown field and a bad phone are dropped, so the bot asks for them itself.
	d.text("масло на бмв х5").expect("Какой пробег?")
	d.text("90000").click("Пропустить").expect("Оставьте номер телефона")
	d.text("+7 900 111 22 33").expect("Автомобиль: BMW X5", "Пробег: 90000")
}

func TestPhoneFromMessageAndKnownClient(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) {
		return Extraction{ServiceID: tireService.ID, Values: map[string]string{"work": "смена шин"}, Phone: "8 (900) 111-22-33"}, nil
	})
	// Everything is in the first message: the bot only asks to confirm.
	d.text("переобуться, мой номер 8 (900) 111-22-33").expect("Проверьте заявку", "Вид работ: Смена шин", "+7 900 111-22-33")
	d.click("✅ Отправить").expect("Заявка №1 принята")

	// Next time the saved phone is offered as a one-tap answer.
	d.engine.extractor = extractorFunc(func(ExtractInput) (Extraction, error) {
		return Extraction{ServiceID: tireService.ID, Values: map[string]string{"work": "Балансировка"}}, nil
	})
	d.text("ещё балансировку").expect("Оставьте номер телефона").expectButtons("+7 900 111-22-33", "📱 Отправить мой номер")
	d.text("+7 900 111-22-33").expect("Проверьте заявку")
}

func TestEditBeforeSubmit(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) {
		return Extraction{ServiceID: oilService.ID, Values: map[string]string{"car": "Lada Vesta", "km": "50000", "when": "завтра"}, Phone: "+79001112233"}, nil
	})
	d.text("масло, веста, 50 тыс, завтра, +79001112233").expect("Проверьте заявку", "Удобное время: завтра")

	d.click("✏️ Изменить").expect("Что изменить?").
		expectButtons("Автомобиль", "Пробег", "Удобное время", "Телефон", "Услугу", "← Назад")
	d.click("← Назад").expect("Проверьте заявку")

	d.click("✏️ Изменить").click("Пробег").expect("Какой пробег?")
	d.text("51000").expect("Проверьте заявку", "Пробег: 51000", "Автомобиль: Lada Vesta")

	d.click("✏️ Изменить").click("Удобное время").click("Пропустить")
	if strings.Contains(d.replies(), "Удобное время") {
		t.Errorf("a cleared optional field must leave the summary:\n%s", d.replies())
	}

	d.click("✏️ Изменить").click("Телефон").expect("Оставьте номер телефона")
	d.text("+7 911 000-00-00").expect("Телефон: +7 911 000-00-00")

	// Changing the service starts its form from scratch but keeps the phone.
	d.click("✏️ Изменить").click("Услугу").expectButtons("Замена масла", "Шиномонтаж")
	d.click("Шиномонтаж").expect("Что нужно сделать?")
	d.click("Смена шин").expect("Услуга: <b>Шиномонтаж</b>", "Телефон: +7 911 000-00-00")

	d.click("❌ Отменить").expect("заявку не отправляю")
	if len(d.backend.requests) != 0 {
		t.Fatal("cancelled draft must not create a request")
	}
	d.send(Incoming{Callback: cbConfirm})
	if d.last.Toast != staleButton {
		t.Errorf("confirm after cancel must be stale: %+v", d.last)
	}
}

func TestCommandsAndUnsupportedInput(t *testing.T) {
	d := newDialog(t, nil)
	d.text("/cancel").expect("нет незавершённой заявки")
	d.text("/my").expect("пока нет заявок")
	d.text("/help").expect("/my", "/cancel")
	d.send(Incoming{}).expect("Голосовые и фото я пока не принимаю")

	d.text("нужна помощь").click("Замена масла").expect("Какой у вас автомобиль?")
	d.text("/cancel").expect("заявку не отправляю")
	d.text("/start").expect("Здравствуйте! Чем могу помочь?")
}

func TestServiceRemovedWhileFilling(t *testing.T) {
	d := newDialog(t, nil)
	d.text("нужна помощь").click("Замена масла").expect("Какой у вас автомобиль?")
	d.backend.services = []domain.Service{tireService}
	d.text("Kia Rio").expect("услуга больше недоступна").expectButtons("Шиномонтаж")

	d.backend.services = nil
	d.click("Шиномонтаж")
	if d.last.Toast == "" {
		t.Errorf("a removed service must be reported: %+v", d.last)
	}
	d.text("а теперь?").expect("приём заявок через бота сейчас не настроен", "+7 900 000-00-00")
}

func TestRequiredFieldAddedBeforeConfirm(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) {
		return Extraction{ServiceID: tireService.ID, Values: map[string]string{"work": "Смена шин"}, Phone: "+79001112233"}, nil
	})
	d.text("переобуться").expect("Проверьте заявку")
	withSize := tireService
	withSize.Fields = append([]domain.FormField{}, tireService.Fields...)
	withSize.Fields = append(withSize.Fields, domain.FormField{ID: "size", Label: "Диаметр", Question: "Какой диаметр колёс?", Type: domain.FieldText, Required: true})
	d.backend.services = []domain.Service{withSize}

	d.click("✅ Отправить").expect("Осталось уточнить", "Какой диаметр колёс?")
	d.text("R16").expect("Диаметр: R16")
	d.click("✅ Отправить").expect("Заявка №1 принята")
}

func TestMyRequestsAndCancel(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) {
		return Extraction{ServiceID: tireService.ID, Values: map[string]string{"work": "Смена шин"}, Phone: "+79001112233"}, nil
	})
	d.backend.recipients = []domain.NotifyRecipient{{ChatID: masterChat}}
	d.text("переобуться").click("✅ Отправить")

	d.text("/my").expect("№1 · Шиномонтаж — ждёт мастера").expectButtons("Отменить №1")
	d.click("Отменить №1").expect("Заявка №1 отменена")
	if d.backend.requests[0].Status != domain.StatusCancelled {
		t.Fatal("request must be cancelled")
	}
	if n := len(d.last.Messages); n != 2 || d.last.Messages[1].ChatID != masterChat {
		t.Errorf("master must be told about the cancellation: %+v", d.last.Messages)
	}
	d.text("/my").expect("отменена").expectButtons()
}

func TestMasterLink(t *testing.T) {
	d := newDialog(t, nil)
	d.text("/start master_wrong").expect("ссылка уже использована или устарела")
	d.text("/start master_secret").expect("Сюда будут приходить новые заявки автосервиса «Гараж №1»")
	if len(d.backend.recipients) != 1 || d.backend.recipients[0].ChatID != clientChat || d.backend.recipients[0].Name != "Пётр" {
		t.Fatalf("recipient not linked: %+v", d.backend.recipients)
	}
	d.text("/start master_secret").expect("ссылка уже использована или устарела")
}

func TestLLMBudgetPerChat(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) { return Extraction{}, nil })
	for range llmCallsPerChat + 5 {
		d.text("ерунда какая-то").expectButtons("Замена масла", "Шиномонтаж")
	}
	if d.llm != llmCallsPerChat {
		t.Errorf("LLM calls must be capped at %d per chat, got %d", llmCallsPerChat, d.llm)
	}
}

func TestClientInputIsEscaped(t *testing.T) {
	d := newDialog(t, nil)
	d.text("нужна помощь").click("Замена масла")
	d.text("<b>Жигули</b> & co").text("1").click("Пропустить").text("+79001112233").
		expect("Автомобиль: &lt;b&gt;Жигули&lt;/b&gt; &amp; co")
}

func TestLongestFormFitsTelegramMessage(t *testing.T) {
	const telegramLimit = 4096
	long := domain.Service{ID: uuid.New(), Name: strings.Repeat("У", 60), Active: true}
	for i := range domain.MaxServiceFields {
		long.Fields = append(long.Fields, domain.FormField{
			ID: "f" + strings.Repeat("0", 3) + string(rune('a'+i)), Label: strings.Repeat("Я", 60), Type: domain.FieldText, Required: true,
		})
	}
	d := newDialog(t, nil)
	d.backend.services = []domain.Service{long}
	d.backend.recipients = []domain.NotifyRecipient{{ChatID: masterChat}}

	d.text(strings.Repeat("щ", maxMessageLength+100))
	for range long.Fields {
		d.text(strings.Repeat("ж", domain.MaxAnswerLength+100))
	}
	d.text("+79001112233").expect("Проверьте заявку")
	summary := d.last.Messages
	d.click("✅ Отправить").expect("принята")
	for _, m := range append(summary, d.last.Messages...) {
		if n := len([]rune(m.Text)); n > telegramLimit {
			t.Errorf("message of %d characters does not fit into Telegram", n)
		}
	}
	// Nothing is lost: the CRM gets the answers in full.
	if got := len([]rune(d.backend.requests[0].Answers[0].Value)); got != domain.MaxAnswerLength {
		t.Errorf("stored answer has %d characters, want %d", got, domain.MaxAnswerLength)
	}
}

func TestButtonsOfEarlierQuestionsAndDraftsAreStale(t *testing.T) {
	car := domain.Service{ID: uuid.New(), Name: "Подбор", Active: true, Fields: []domain.FormField{
		{ID: "fuel", Label: "Топливо", Question: "Какое топливо?", Type: domain.FieldChoice, Required: true, Options: []string{"Бензин", "Дизель"}},
		{ID: "gear", Label: "КПП", Question: "Какая коробка?", Type: domain.FieldChoice, Required: true, Options: []string{"Механика", "Автомат"}},
	}}
	d := newDialog(t, nil)
	d.backend.services = []domain.Service{car}

	fuelQuestion := d.text("нужен подбор").expect("Какое топливо?").last
	d.text("Бензин").expect("Какая коробка?")
	d.last = fuelQuestion
	d.click("Дизель")
	if d.last.Toast != staleButton {
		t.Errorf("an option of an answered question must be stale: %+v", d.last)
	}
	d.text("Автомат").text("+79001112233").expect("Топливо: Бензин", "КПП: Автомат")
	for _, label := range d.buttons() {
		if data := d.callbackData(label); len(data) > 64 {
			t.Errorf("button %q has %d bytes of data, Telegram allows 64", label, len(data))
		}
	}

	oldSummary := d.last
	d.click("❌ Отменить")
	d.text("нужен подбор").text("Дизель").text("Механика").text("+79001112233").expect("Проверьте заявку")
	d.last = oldSummary
	d.click("✅ Отправить")
	if d.last.Toast != staleButton || len(d.backend.requests) != 0 {
		t.Errorf("the summary of a cancelled draft must not send the new one: %+v, %d requests", d.last, len(d.backend.requests))
	}
}

// callbackData returns the data of the inline button with the given label on the last bot message.
func (d *dialog) callbackData(label string) string {
	for _, m := range d.last.Messages {
		for _, row := range m.Buttons {
			for _, b := range row {
				if b.Text == label {
					return b.Data
				}
			}
		}
	}
	return ""
}

func TestClientCanLeaveAWrongService(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) { return Extraction{ServiceID: oilService.ID}, nil })
	d.text("что-то стучит").expect("Оформляю заявку: <b>Замена масла</b>").expectButtons("Другая услуга")
	d.click("Другая услуга").expectButtons("Замена масла", "Шиномонтаж")
	// A service the client picked by hand needs no way out.
	d.click("Шиномонтаж").expect("Что нужно сделать?").expectButtons("Смена шин", "Балансировка")
	d.text("Передумал!").expect("заявку не отправляю")
	if len(d.backend.requests) != 0 {
		t.Fatal("a cancelled draft must not create a request")
	}
}

func TestCourtesyMessagesAreAnsweredWithoutLLM(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) {
		return Extraction{ServiceID: tireService.ID, Values: map[string]string{"work": "Смена шин"}, Phone: "+79001112233"}, nil
	})
	d.text("Добрый день!").expect("Здравствуйте! Напишите, что нужно сделать с машиной.")
	d.text("переобуться").click("✅ Отправить").expect("принята")
	d.text("Спасибо)").expect("Пожалуйста! Если понадобится помощь — напишите.")
	if d.llm != 1 {
		t.Errorf("only the request itself goes to the LLM, got %d calls", d.llm)
	}
	if got := d.backend.requests[0].Message; got != "переобуться" {
		t.Errorf("a greeting must not become the client's message, got %q", got)
	}
	d.text("/start")
	if d.replies() != "Здравствуйте! Чем могу помочь?" {
		t.Errorf("/start answer is fixed by the customer, got %q", d.replies())
	}
}

func TestFollowUpMessagesAddToTheFirst(t *testing.T) {
	d := newDialog(t, func(in ExtractInput) (Extraction, error) {
		if !strings.Contains(in.Message, "шиномонтаж") {
			return Extraction{}, nil
		}
		return Extraction{ServiceID: tireService.ID, Values: map[string]string{"work": "Смена шин"}, Phone: "+79001112233"}, nil
	})
	d.text("колесо спускает за ночь").expect("какая услуга вам нужна")
	d.text("шиномонтаж").click("✅ Отправить").expect("принята")
	if got := d.backend.requests[0].Message; got != "колесо спускает за ночь\nшиномонтаж" {
		t.Errorf("the master must see every free-form message of the client, got %q", got)
	}
}

func TestDailyRequestLimitKeepsTheDraft(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) {
		return Extraction{ServiceID: tireService.ID, Values: map[string]string{"work": "Смена шин"}, Phone: "+79001112233"}, nil
	})
	for range domain.MaxRequestsPerClientPerDay {
		d.text("переобуться").click("✅ Отправить").expect("принята")
	}
	summary := d.text("переобуться").last
	d.click("✅ Отправить").expect("Вы уже оставили 5 заявок за сутки. Мастер свяжется с вами по ним; новую можно будет отправить позже.")

	// A day later the client sends the same draft with the same button.
	for i := range d.backend.requests {
		d.backend.requests[i].CreatedAt = time.Now().Add(-25 * time.Hour)
	}
	d.last = summary
	d.click("✅ Отправить").expect("Заявка №6 принята")
}

func TestLLMBudgetPerAutoservice(t *testing.T) {
	d := newDialog(t, func(ExtractInput) (Extraction, error) { return Extraction{}, nil })
	for i := range llmCallsPerAutoservice + 1 {
		// Every message comes from a new chat, so the per-chat budget never runs out.
		d.from.TelegramID = int64(1000 + i)
		d.text("ерунда какая-то").expectButtons("Замена масла", "Шиномонтаж")
	}
	if d.llm != llmCallsPerAutoservice {
		t.Errorf("LLM calls must be capped at %d per autoservice, got %d", llmCallsPerAutoservice, d.llm)
	}
}

func TestUnsupportedMessageRepeatsTheQuestion(t *testing.T) {
	d := newDialog(t, nil)
	d.text("нужна помощь").click("Шиномонтаж")
	d.send(Incoming{}).expect("Голосовые и фото я пока не принимаю", "Что нужно сделать?").expectButtons("Смена шин", "Балансировка")
	d.click("Смена шин")
	d.send(Incoming{}).expect("Голосовые и фото я пока не принимаю", "Оставьте номер телефона").expectButtons("📱 Отправить мой номер")
}

package app_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/app"
	"github.com/marcho4/autoservice-dashboard/backend/internal/bot"
	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/postgres"
	"github.com/marcho4/autoservice-dashboard/backend/internal/testkit"
	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
)

// startRunner runs the real bot runner against the fake Telegram; the returned function stops it and waits.
func startRunner(a *api) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.NewBotRunner(a.cfg, a.pool, slog.New(slog.NewJSONHandler(io.Discard, nil))).Run(ctx)
	}()
	stopped := false
	stop = func() {
		if !stopped {
			stopped = true
			cancel()
			<-done
		}
	}
	a.t.Cleanup(stop)
	return stop
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// chat is one Telegram user talking to a bot.
type chat struct {
	a     *api
	token string
	user  testkit.TelegramUser
	last  testkit.SentMessage
}

// next waits for the bot's next message in this chat and checks what it says.
func (c *chat) next(parts ...string) *chat {
	c.a.t.Helper()
	m, err := c.a.tg.NextMessage(c.token, c.user.ID, 10*time.Second)
	if err != nil {
		c.a.t.Fatal(err)
	}
	for _, part := range parts {
		if !strings.Contains(m.Text, part) {
			c.a.t.Fatalf("bot message must contain %q, got:\n%s", part, m.Text)
		}
	}
	c.last = m
	return c
}

func (c *chat) say(text string) *chat { c.a.tg.SendText(c.token, c.user, text); return c }

func (c *chat) click(label string) *chat {
	c.a.t.Helper()
	data, ok := c.last.Button(label)
	if !ok {
		c.a.t.Fatalf("no button %q on message %q (buttons: %q)", label, c.last.Text, c.last.ButtonTexts())
	}
	c.a.tg.Click(c.token, c.user, c.last, data)
	return c
}

func TestBotEndToEnd(t *testing.T) {
	a := setup(t)

	// Onboarding as the owner sees it: register, paste the token — the bot starts by itself.
	r := a.do("POST", "/api/v1/auth/register", "", map[string]string{
		"email": "owner@example.com", "password": "password123", "full_name": "Иван", "autoservice_name": "Гараж №1"})
	expect(t, r, 201, "register")
	tok := r.body["access_token"].(string)

	stop := startRunner(a)
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tok, map[string]string{"token": tokenA}), 200, "connect bot")
	botStatus := func() string {
		b := a.do("GET", "/api/v1/autoservice/bot", tok, nil)
		return fmt.Sprint(b.body["status"], ":", b.body["status_error"])
	}
	eventually(t, "bot is running", func() bool { return botStatus() == "running:" })

	// The fake LLM answers with field IDs of the real catalog, the way a model reading the prompt would.
	oil := items(a.do("GET", "/api/v1/services", tok, nil))[1]
	if oil["name"] != "Техобслуживание и замена масла" {
		t.Fatalf("unexpected catalog: %v", oil)
	}
	fieldID := map[string]string{}
	for _, f := range oil["fields"].([]any) {
		f := f.(map[string]any)
		fieldID[f["label"].(string)] = f["id"].(string)
	}
	// The model also names a field the form does not have: it must be dropped, not stored.
	a.llm.SetReply(func(system, user string) (int, string) {
		if !strings.Contains(user, "масло") {
			return http.StatusOK, `{"service": null, "fields": {}, "phone": null}`
		}
		return http.StatusOK, fmt.Sprintf(`{"service": "2", "fields": {%q: "Toyota Camry 2015", "mileage": "120000"}, "phone": null}`,
			fieldID["Автомобиль"])
	})

	// A master subscribes to notifications with the one-time link from the dashboard.
	master := &chat{a: a, token: tokenA, user: testkit.TelegramUser{ID: 7, FirstName: "Мастер"}}
	notifyLink := a.do("GET", "/api/v1/autoservice/bot", tok, nil).body["notify_link"].(string)
	payload := notifyLink[strings.Index(notifyLink, "start=")+len("start="):]
	master.say("/start " + payload).next("Сюда будут приходить новые заявки автосервиса «Гараж №1»")
	master.say("/start " + payload).next("уже использована или устарела")
	b := a.do("GET", "/api/v1/autoservice/bot", tok, nil)
	recipients := b.body["recipients"].([]any)
	if len(recipients) != 1 || recipients[0].(map[string]any)["name"] != "Мастер" || b.body["notify_link"] == notifyLink {
		t.Fatalf("master must be subscribed and the link rotated: %v", b.body)
	}

	// The client's side of the dialog.
	client := &chat{a: a, token: tokenA, user: testkit.TelegramUser{ID: 42, FirstName: "Пётр", Username: "petya"}}
	client.say("/start").next("Здравствуйте! Чем могу помочь?")
	client.say("Добрый день").next("Здравствуйте! Напишите, что нужно сделать с машиной.")
	client.say("мне бы машину посмотреть").next("какая услуга вам нужна")
	if got := client.last.ButtonTexts(); len(got) != 5 || got[0] != "Диагностика" {
		t.Fatalf("service buttons: %q", got)
	}
	a.tg.SendPhoto(tokenA, client.user)
	client.next("Голосовые и фото я пока не принимаю")

	client.say("Хочу поменять масло на камри 2015").
		next("Оформляю заявку: <b>Техобслуживание и замена масла</b>", "Что нужно сделать?")
	calls := a.llm.Calls()
	if last := calls[len(calls)-1]; !strings.Contains(last.System, "Гараж №1") || !strings.Contains(last.System, fieldID["Автомобиль"]) {
		t.Fatalf("the prompt must describe the autoservice and its forms:\n%s", last.System)
	}

	// The dialog state is in the database: a restart of the runner in the middle loses nothing.
	stop()
	client.say("Замена масла и фильтров")
	startRunner(a)
	client.next("Когда вам было бы удобно приехать? Точное время мастер согласует с вами по телефону.").
		click("Пропустить").next("Оставьте номер телефона")
	if kb := client.last.ReplyMarkup.Keyboard; len(kb) != 1 || !kb[0][0].RequestContact {
		t.Fatalf("phone step must offer the contact button: %+v", client.last.ReplyMarkup)
	}
	a.tg.SendContact(tokenA, client.user, "79001112233")
	client.next("Записал номер +7 900 111-22-33")
	client.next("Проверьте заявку", "Автомобиль: Toyota Camry 2015",
		"Что нужно сделать: Замена масла и фильтров", "Телефон: +7 900 111-22-33")
	if strings.Contains(client.last.Text, "120000") {
		t.Fatalf("a value for a field the form does not have must be dropped:\n%s", client.last.Text)
	}

	if total := a.do("GET", "/api/v1/requests", tok, nil).body["total"].(float64); total != 0 {
		t.Fatal("nothing reaches the CRM before the client confirms")
	}
	summary := client.last
	client.click("✅ Отправить").next("Заявка №1 принята")
	eventually(t, "confirm buttons are removed", func() bool {
		return len(a.tg.Message(tokenA, summary.MessageID).ReplyMarkup.InlineKeyboard) == 0
	})
	if n := len(a.llm.Calls()); n != 2 {
		t.Fatalf("LLM is used only to recognize free-form messages, got %d calls", n)
	}

	// The request is in the CRM with the client and the answers; the master got it in Telegram.
	list := a.do("GET", "/api/v1/requests", tok, nil)
	expect(t, list, 200, "list requests")
	if list.body["total"].(float64) != 1 {
		t.Fatalf("expected one request: %v", list.body)
	}
	req := items(list)[0]
	answers := fmt.Sprint(req["answers"])
	if req["number"].(float64) != 1 || req["status"] != "new" || req["service_name"] != "Техобслуживание и замена масла" ||
		req["service_id"] != oil["id"] || req["phone"] != "+79001112233" ||
		req["message"] != "мне бы машину посмотреть\nХочу поменять масло на камри 2015" ||
		!strings.Contains(answers, "Toyota Camry 2015") || !strings.Contains(answers, "Замена масла и фильтров") || strings.Contains(answers, "Удобное время") {
		t.Fatalf("unexpected request: %v", req)
	}
	if c := req["client"].(map[string]any); c["telegram_username"] != "petya" || c["name"] != "Пётр" || c["phone"] != "+79001112233" {
		t.Fatalf("unexpected client: %v", c)
	}
	master.next("Новая заявка №1", "Техобслуживание и замена масла", "Toyota Camry 2015", "Пётр @petya", "+79001112233")

	// The client follows the request from the bot; once the master took it, it cannot be cancelled there.
	client.say("/my").next("№1 · Техобслуживание и замена масла — ждёт мастера")
	expect(t, a.do("POST", "/api/v1/requests/"+req["id"].(string)+"/take", tok, nil), 200, "take")
	client.click("Отменить №1")
	eventually(t, "late cancel is refused", func() bool {
		toasts := a.tg.Toasts(tokenA)
		return len(toasts) > 0 && strings.Contains(toasts[len(toasts)-1], "уже взял заявку в работу")
	})
	client.say("/my").next("в работе")

	// Deleting the service keeps the request readable.
	expect(t, a.do("DELETE", "/api/v1/services/"+oil["id"].(string), tok, nil), 204, "delete service")
	got := a.do("GET", "/api/v1/requests/"+req["id"].(string), tok, nil)
	if got.body["service_id"] != nil || got.body["service_name"] != "Техобслуживание и замена масла" || len(got.body["answers"].([]any)) != 2 {
		t.Fatalf("request must survive its service: %v", got.body)
	}

	// The owner replaces the bot: the runner switches to the new token without a restart.
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tok, map[string]string{"token": tokenB}), 200, "replace bot")
	eventually(t, "new bot is polled and the old one is not", func() bool {
		return a.tg.Polling(tokenB) == 1 && a.tg.Polling(tokenA) == 0 && botStatus() == "running:"
	})
	if rest := a.do("GET", "/api/v1/autoservice/bot", tok, nil).body["recipients"].([]any); len(rest) != 0 {
		t.Fatalf("masters of the old bot must not be carried over: %v", rest)
	}
	clientB := &chat{a: a, token: tokenB, user: client.user}
	clientB.say("/start").next("Здравствуйте! Чем могу помочь?")

	// A revoked token shows up in the dashboard instead of failing silently.
	a.tg.RemoveBot(tokenB)
	eventually(t, "revoked token is reported", func() bool { return botStatus() == "error:unauthorized" })

	expect(t, a.do("DELETE", "/api/v1/autoservice/bot", tok, nil), 204, "disconnect bot")
}

func TestOnlyOneRunnerPollsTelegram(t *testing.T) {
	a := setup(t)
	tok := register(a, "owner@example.com")
	expect(t, a.do("POST", "/api/v1/autoservice", tok, map[string]string{"name": "Гараж"}), 201, "create autoservice")
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tok, map[string]string{"token": tokenA}), 200, "connect bot")

	stopFirst := startRunner(a)
	eventually(t, "first runner polls", func() bool { return a.tg.Polling(tokenA) == 1 })
	startRunner(a)
	// Two replicas are up, but Telegram must see a single consumer per bot.
	for range 20 {
		time.Sleep(25 * time.Millisecond)
		if n := a.tg.Polling(tokenA); n > 1 {
			t.Fatalf("%d runners poll the same bot", n)
		}
	}

	// When the leader goes away, the standby takes over.
	stopFirst()
	eventually(t, "second runner takes over", func() bool { return a.tg.Polling(tokenA) == 1 })
	client := &chat{a: a, token: tokenA, user: testkit.TelegramUser{ID: 42, FirstName: "Пётр"}}
	client.say("/start").next("Здравствуйте! Чем могу помочь?")
}

func TestBotIsReportedRunningBeforeFirstPollReturns(t *testing.T) {
	a := setup(t)
	// Production timing: an idle long poll returns only after tens of seconds.
	a.cfg.Runner.PollTimeout = 30 * time.Second
	tok := register(a, "owner@example.com")
	expect(t, a.do("POST", "/api/v1/autoservice", tok, map[string]string{"name": "Гараж"}), 201, "create autoservice")
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tok, map[string]string{"token": tokenA}), 200, "connect bot")

	startRunner(a)
	start := time.Now()
	eventually(t, "bot is running", func() bool {
		return a.do("GET", "/api/v1/autoservice/bot", tok, nil).body["status"] == "running"
	})
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("the owner waited %s to see the bot running", took)
	}
}

// flakyState loses the next save, like a database blip right after the request was created.
type flakyState struct {
	*postgres.ConversationRepo
	fail bool
}

func (s *flakyState) Save(ctx context.Context, autoserviceID uuid.UUID, chatID int64, state []byte) error {
	if s.fail {
		s.fail = false
		return errors.New("database blip")
	}
	return s.ConversationRepo.Save(ctx, autoserviceID, chatID, state)
}

func TestSubmitAfterLostStateDoesNotDuplicateTheRequest(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	r := a.do("POST", "/api/v1/auth/register", "", map[string]string{
		"email": "owner@example.com", "password": "password123", "full_name": "Иван", "autoservice_name": "Гараж №1"})
	expect(t, r, 201, "register")
	tok := r.body["access_token"].(string)
	autoserviceID := uuid.MustParse(a.do("GET", "/api/v1/autoservice", tok, nil).body["id"].(string))

	service := usecases.NewBotRunnerService(postgres.NewBotRepo(a.pool), postgres.NewAutoserviceRepo(a.pool),
		postgres.NewServiceRepo(a.pool), postgres.NewClientRepo(a.pool), postgres.NewRequestRepo(a.pool))
	state := &flakyState{ConversationRepo: postgres.NewConversationRepo(a.pool)}
	engine := bot.NewEngine(service, state, nil, "", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	who := domain.ClientIdentity{TelegramID: 42, Name: "Пётр"}
	var last bot.Result
	send := func(in bot.Incoming) error {
		in.ChatID, in.From = who.TelegramID, who
		res, err := engine.Handle(ctx, autoserviceID, in)
		if err == nil {
			last = res
		}
		return err
	}
	button := func(label string) bot.Incoming {
		for _, m := range last.Messages {
			for _, row := range m.Buttons {
				for _, b := range row {
					if b.Text == label {
						return bot.Incoming{Callback: b.Data, MessageText: m.Text}
					}
				}
			}
		}
		t.Fatalf("no button %q in %+v", label, last.Messages)
		return bot.Incoming{}
	}
	if err := send(bot.Incoming{Text: "сколько стоит ТО?"}); err != nil {
		t.Fatal(err)
	}
	if err := send(button("Вопрос мастеру")); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Сколько стоит ТО на Kia Rio?", "+79001112233"} {
		if err := send(bot.Incoming{Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	confirm := button("✅ Отправить")

	state.fail = true
	if err := send(confirm); err == nil {
		t.Fatal("the lost save must be reported")
	}
	if err := send(confirm); err != nil {
		t.Fatal(err)
	}
	if text := last.Messages[0].Text; !strings.Contains(text, "Заявка №1 принята") {
		t.Fatalf("the repeated submit must confirm the request made the first time, got %q", text)
	}
	list := a.do("GET", "/api/v1/requests", tok, nil)
	if total := list.body["total"].(float64); total != 1 {
		t.Fatalf("one draft must make one request, got %v", total)
	}

	// The repeated submit did not use up a number, and nobody else can claim the draft.
	next, err := service.CreateRequest(ctx, autoserviceID, who, domain.RequestInput{ServiceName: "Ремонт", Phone: "+79001112233"})
	if err != nil || next.Number != 2 {
		t.Fatalf("the next request must get number 2, got %d, %v", next.Number, err)
	}
	draftID := uuid.MustParse(items(list)[0]["id"].(string))
	other := domain.ClientIdentity{TelegramID: 77}
	if _, err := service.CreateRequest(ctx, autoserviceID, other, domain.RequestInput{DraftID: draftID, ServiceName: "Ремонт", Phone: "+79001112233"}); err == nil {
		t.Fatal("a draft ID of another client's request must be an error")
	}
}

func TestAbandonedDialogStartsOver(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	tok := register(a, "owner@example.com")
	expect(t, a.do("POST", "/api/v1/autoservice", tok, map[string]string{"name": "Гараж"}), 201, "create autoservice")
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tok, map[string]string{"token": tokenA}), 200, "connect bot")
	startRunner(a)

	client := &chat{a: a, token: tokenA, user: testkit.TelegramUser{ID: 42, FirstName: "Пётр"}}
	client.say("нужен ремонт").next("какая услуга вам нужна").click("Ремонт").next("Какой у вас автомобиль?")
	age := func() {
		if _, err := a.pool.Exec(ctx, `UPDATE conversations SET updated_at = now() - interval '25 hours'`); err != nil {
			t.Fatal(err)
		}
	}
	age()
	// A day later the message is a new request, not the answer to the forgotten question.
	client.say("Kia Rio").next("какая услуга вам нужна")

	age()
	if err := postgres.NewConversationRepo(a.pool).DeleteExpired(ctx); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := a.pool.QueryRow(ctx, `SELECT count(*) FROM conversations`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("expired dialogs must be deleted, %d left (%v)", left, err)
	}
}

func TestShutdownCutsLongHandlersAndDropsQueuedUpdates(t *testing.T) {
	a := setup(t)
	a.cfg.Runner.HandleGrace = time.Second
	tok := register(a, "owner@example.com")
	expect(t, a.do("POST", "/api/v1/autoservice", tok, map[string]string{"name": "Гараж"}), 201, "create autoservice")
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tok, map[string]string{"token": tokenA}), 200, "connect bot")
	// The LLM hangs for one client and answers the other once the runner is stopping.
	hang, answer := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(hang) })
	a.llm.SetReply(func(_, user string) (int, string) {
		select {
		case <-hang:
		case <-answer:
			if strings.Contains(user, "висит") {
				<-hang
			}
		}
		return http.StatusOK, `{"service": null}`
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.NewBotRunner(a.cfg, a.pool, slog.New(slog.NewJSONHandler(io.Discard, nil))).Run(ctx)
	}()
	t.Cleanup(func() { cancel(); <-done })

	stuck := &chat{a: a, token: tokenA, user: testkit.TelegramUser{ID: 42, FirstName: "Пётр"}}
	busy := &chat{a: a, token: tokenA, user: testkit.TelegramUser{ID: 43, FirstName: "Анна"}}
	stuck.say("висит")
	busy.say("нужен ремонт")
	eventually(t, "both messages are with the LLM", func() bool { return len(a.llm.Calls()) == 2 })
	busy.say("/start")
	eventually(t, "the runner took the queued message", func() bool { return a.tg.Pending(tokenA) == 0 })

	start := time.Now()
	cancel()
	close(answer)
	<-done
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("the runner took %s to stop, the grace period is %s", took, a.cfg.Runner.HandleGrace)
	}
	var replies []string
	for _, m := range a.tg.Sent(tokenA, busy.user.ID) {
		replies = append(replies, m.Text)
	}
	if got := strings.Join(replies, "\n---\n"); !strings.Contains(got, "какая услуга вам нужна") || strings.Contains(got, "Чем могу помочь") {
		t.Fatalf("the update being handled must be finished and the queued one dropped, the bot sent:\n%s", got)
	}
}

func TestBotProfileDescribesTheAutoservice(t *testing.T) {
	a := setup(t)
	tok := register(a, "owner@example.com")
	expect(t, a.do("POST", "/api/v1/autoservice", tok, map[string]string{
		"name": "Гараж №1", "address": "Москва, Лесная 5", "phone": "+7 900 000-00-00"}), 201, "create autoservice")
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tok, map[string]string{"token": tokenA}), 200, "connect bot")
	startRunner(a)

	eventually(t, "bot description is set", func() bool { d, _ := a.tg.Description(tokenA); return d != "" })
	description, short := a.tg.Description(tokenA)
	want := "Бот автосервиса «Гараж №1». Опишите, что нужно сделать с машиной, — я передам мастеру, и он вам перезвонит.\n" +
		"Адрес: Москва, Лесная 5. Телефон: +7 900 000-00-00."
	if description != want {
		t.Errorf("description = %q, want %q", description, want)
	}
	if short != "Заявки в автосервис «Гараж №1»" {
		t.Errorf("short description = %q", short)
	}
}

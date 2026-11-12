package app_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/marcho4/autoservice-dashboard/backend/internal/app"
	"github.com/marcho4/autoservice-dashboard/backend/internal/testkit"
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
	a.llm.SetReply(func(system, user string) (int, string) {
		if !strings.Contains(user, "масло") {
			return http.StatusOK, `{"service": null, "fields": {}, "phone": null}`
		}
		return http.StatusOK, fmt.Sprintf(`{"service": "2", "fields": {%q: "Toyota Camry 2015", %q: "120000"}, "phone": null}`,
			fieldID["Автомобиль"], fieldID["Пробег, км"])
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
	client.say("Добрый день").next("какая услуга вам нужна")
	if got := client.last.ButtonTexts(); len(got) != 5 || got[0] != "Диагностика" {
		t.Fatalf("service buttons: %q", got)
	}
	a.tg.SendPhoto(tokenA, client.user)
	client.next("только текст")

	client.say("Хочу поменять масло на камри 2015, пробег 120 тысяч").
		next("Оформляю заявку: <b>Техобслуживание и замена масла</b>", "Что нужно сделать?")
	calls := a.llm.Calls()
	if last := calls[len(calls)-1]; !strings.Contains(last.System, "Гараж №1") || !strings.Contains(last.System, fieldID["Автомобиль"]) {
		t.Fatalf("the prompt must describe the autoservice and its forms:\n%s", last.System)
	}

	// The dialog state is in the database: a restart of the runner in the middle loses nothing.
	stop()
	client.say("Замена масла и фильтров")
	startRunner(a)
	client.next("Когда вам удобно приехать?").click("Пропустить").next("Оставьте номер телефона")
	if kb := client.last.ReplyMarkup.Keyboard; len(kb) != 1 || !kb[0][0].RequestContact {
		t.Fatalf("phone step must offer the contact button: %+v", client.last.ReplyMarkup)
	}
	a.tg.SendContact(tokenA, client.user, "79001112233")
	client.next("Записал номер +7 900 111-22-33")
	client.next("Проверьте заявку", "Автомобиль: Toyota Camry 2015", "Пробег, км: 120000",
		"Что нужно сделать: Замена масла и фильтров", "Телефон: +7 900 111-22-33")

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
		req["message"] != "Хочу поменять масло на камри 2015, пробег 120 тысяч" ||
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
	if got.body["service_id"] != nil || got.body["service_name"] != "Техобслуживание и замена масла" || len(got.body["answers"].([]any)) != 3 {
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

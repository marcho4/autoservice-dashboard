package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/app"
	"github.com/marcho4/autoservice-dashboard/backend/internal/bot"
	"github.com/marcho4/autoservice-dashboard/backend/internal/config"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/postgres"
	"github.com/marcho4/autoservice-dashboard/backend/internal/testkit"
)

const apiKey = "test-bot-runner-api-key"

var (
	tokenA = "111111:" + strings.Repeat("a", 35)
	tokenB = "222222:" + strings.Repeat("b", 35)
	tokenC = "333333:" + strings.Repeat("c", 35)
)

type api struct {
	t    *testing.T
	srv  *httptest.Server
	tg   *testkit.Telegram
	llm  *testkit.LLM
	cfg  config.Config
	pool *pgxpool.Pool
}

type resp struct {
	status int
	body   map[string]any
}

func (a *api) do(method, path, token string, body any, headers ...string) resp {
	a.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, a.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	out := resp{status: res.StatusCode}
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func (a *api) runner(method, path string, body any) resp {
	a.t.Helper()
	return a.do(method, "/internal/v1"+path, "", body, "X-API-Key", apiKey)
}

func expect(t *testing.T, r resp, status int, what string) {
	t.Helper()
	if r.status != status {
		t.Fatalf("%s: status %d, want %d, body %v", what, r.status, status, r.body)
	}
}

func errCode(r resp) string {
	e, _ := r.body["error"].(map[string]any)
	s, _ := e["code"].(string)
	return s
}

func setup(t *testing.T) *api {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TEST_DATABASE_URL to run end-to-end tests; its public schema will be dropped")
	}
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if err := postgres.Migrate(ctx, pool, log); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, pool, log); err != nil {
		t.Fatal(err)
	}

	tg := testkit.NewTelegram()
	t.Cleanup(tg.Close)
	tg.AddBot(tokenA, "service_a_bot")
	tg.AddBot(tokenB, "service_b_bot")
	tg.AddBot(tokenC, "service_c_bot")
	llm := testkit.NewLLM()
	t.Cleanup(llm.Close)
	cfg := config.Config{
		JWTSecret:      strings.Repeat("s", 32),
		JWTTTL:         time.Hour,
		BotAPIKey:      apiKey,
		TelegramAPIURL: tg.URL(),
		LLMBaseURL:     llm.URL(),
		LLMModel:       "test-model",
		Runner:         bot.RunnerConfig{ReconcileInterval: 30 * time.Millisecond, PollTimeout: time.Second},
	}
	router, err := app.NewRouter(cfg, pool, log)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return &api{t: t, srv: srv, tg: tg, llm: llm, cfg: cfg, pool: pool}
}

func register(a *api, email string) string {
	a.t.Helper()
	r := a.do("POST", "/api/v1/auth/register", "", map[string]string{"email": email, "password": "password123", "full_name": "Мастер " + email})
	expect(a.t, r, 201, "register "+email)
	return r.body["access_token"].(string)
}

func TestEmployeeAccountFlow(t *testing.T) {
	a := setup(t)

	tok := register(a, "Ivan@Example.com")
	expect(t, a.do("POST", "/api/v1/auth/register", "", map[string]string{"email": "ivan@example.com", "password": "password123", "full_name": "x"}), 409, "duplicate email")
	expect(t, a.do("POST", "/api/v1/auth/register", "", map[string]string{"email": "bad", "password": "password123", "full_name": "x"}), 400, "bad email")
	expect(t, a.do("POST", "/api/v1/auth/register", "", map[string]string{"email": "p@example.com", "password": "short", "full_name": "x"}), 400, "short password")

	expect(t, a.do("POST", "/api/v1/auth/login", "", map[string]string{"email": "ivan@example.com", "password": "wrong-pass"}), 401, "wrong password")
	login := a.do("POST", "/api/v1/auth/login", "", map[string]string{"email": "ivan@example.com", "password": "password123"})
	expect(t, login, 200, "login")
	tok2 := login.body["access_token"].(string)

	me := a.do("GET", "/api/v1/me", tok, nil)
	expect(t, me, 200, "me")
	if me.body["email"] != "ivan@example.com" {
		t.Fatalf("email not normalized: %v", me.body)
	}
	if _, leaked := me.body["password_hash"]; leaked {
		t.Fatal("password hash leaked")
	}
	expect(t, a.do("GET", "/api/v1/me", "", nil), 401, "no token")
	expect(t, a.do("GET", "/api/v1/me", "garbage", nil), 401, "bad token")

	upd := a.do("PATCH", "/api/v1/me", tok, map[string]string{"full_name": "Иван Петров"})
	expect(t, upd, 200, "update profile")
	if upd.body["full_name"] != "Иван Петров" || upd.body["email"] != "ivan@example.com" {
		t.Fatalf("unexpected profile: %v", upd.body)
	}

	expect(t, a.do("POST", "/api/v1/auth/logout", tok2, nil), 204, "logout")
	expect(t, a.do("GET", "/api/v1/me", tok2, nil), 401, "revoked token")
	expect(t, a.do("GET", "/api/v1/me", tok, nil), 200, "other session still valid")

	expect(t, a.do("PUT", "/api/v1/me/password", tok, map[string]string{"current_password": "nope", "new_password": "newpassword1"}), 401, "wrong current password")
	pw := a.do("PUT", "/api/v1/me/password", tok, map[string]string{"current_password": "password123", "new_password": "newpassword1"})
	expect(t, pw, 200, "change password")
	expect(t, a.do("GET", "/api/v1/me", tok, nil), 401, "old token after password change")
	tok = pw.body["access_token"].(string)
	expect(t, a.do("POST", "/api/v1/auth/login", "", map[string]string{"email": "ivan@example.com", "password": "newpassword1"}), 200, "login new password")

	expect(t, a.do("DELETE", "/api/v1/me", tok, map[string]string{"password": "wrong"}), 401, "delete with wrong password")
	expect(t, a.do("DELETE", "/api/v1/me", tok, map[string]string{"password": "newpassword1"}), 204, "delete account")
	expect(t, a.do("GET", "/api/v1/me", tok, nil), 401, "deleted account token")
	expect(t, a.do("POST", "/api/v1/auth/login", "", map[string]string{"email": "ivan@example.com", "password": "newpassword1"}), 401, "login deleted")
}

func TestAutoserviceBotAndRequestsFlow(t *testing.T) {
	a := setup(t)
	tokA := register(a, "a@example.com")
	tokB := register(a, "b@example.com")

	expect(t, a.do("GET", "/api/v1/autoservice", tokA, nil), 404, "no autoservice yet")
	expect(t, a.do("GET", "/api/v1/requests", tokA, nil), 404, "requests without autoservice")
	expect(t, a.do("POST", "/api/v1/autoservice", tokA, map[string]string{"name": ""}), 400, "empty name")
	expect(t, a.do("POST", "/api/v1/autoservice", tokA, map[string]string{"name": "Сервис А", "address": "Москва"}), 201, "create A")
	r := a.do("POST", "/api/v1/autoservice", tokA, map[string]string{"name": "Второй"})
	expect(t, r, 409, "second autoservice")
	if errCode(r) != "autoservice_exists" {
		t.Fatalf("code %s", errCode(r))
	}
	r = a.do("PUT", "/api/v1/autoservice", tokA, map[string]string{"name": "Сервис А+", "phone": "+7 999 000-00-00"})
	expect(t, r, 200, "update A")
	if r.body["name"] != "Сервис А+" {
		t.Fatalf("not updated: %v", r.body)
	}
	expect(t, a.do("POST", "/api/v1/autoservice", tokB, map[string]string{"name": "Сервис Б"}), 201, "create B")

	expect(t, a.do("GET", "/api/v1/autoservice/bot", tokA, nil), 404, "no bot")
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tokA, map[string]string{"token": "not-a-token"}), 400, "malformed token")
	r = a.do("PUT", "/api/v1/autoservice/bot", tokA, map[string]string{"token": "999999:" + strings.Repeat("z", 35)})
	expect(t, r, 422, "token rejected by telegram")
	r = a.do("PUT", "/api/v1/autoservice/bot", tokA, map[string]string{"token": tokenA})
	expect(t, r, 200, "connect bot A")
	if r.body["username"] != "service_a_bot" || r.body["masked_token"] != "111111:****aaaa" ||
		r.body["status"] != "pending" || r.body["link"] != "https://t.me/service_a_bot" ||
		!strings.HasPrefix(r.body["notify_link"].(string), "https://t.me/service_a_bot?start=master_") {
		t.Fatalf("unexpected bot view: %v", r.body)
	}
	if strings.Contains(fmt.Sprint(a.do("GET", "/api/v1/autoservice/bot", tokA, nil).body), tokenA) {
		t.Fatal("full token returned to employee")
	}
	r = a.do("PUT", "/api/v1/autoservice/bot", tokB, map[string]string{"token": tokenA})
	expect(t, r, 409, "same bot to second autoservice")
	if errCode(r) != "bot_already_linked" {
		t.Fatalf("code %s", errCode(r))
	}
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tokB, map[string]string{"token": tokenC}), 200, "connect C to B")
	r = a.do("PUT", "/api/v1/autoservice/bot", tokB, map[string]string{"token": tokenB})
	expect(t, r, 200, "replace B token")
	if r.body["username"] != "service_b_bot" {
		t.Fatalf("not replaced: %v", r.body)
	}

	expect(t, a.do("GET", "/internal/v1/bots", "", nil), 401, "runner without key")
	expect(t, a.do("GET", "/internal/v1/bots", "", nil, "X-API-Key", "wrong"), 401, "runner wrong key")
	bots := a.runner("GET", "/bots", nil)
	expect(t, bots, 200, "runner bots")
	if bots.body["total"].(float64) != 2 || !strings.Contains(fmt.Sprint(bots.body), tokenA) {
		t.Fatalf("unexpected bots: %v", bots.body)
	}

	newReq := func(bot string, tgID int64, brand string) resp {
		return a.runner("POST", "/bots/"+bot+"/requests", map[string]any{
			"client":       map[string]any{"telegram_id": tgID, "telegram_username": "petya", "name": "Петя"},
			"service_name": "Диагностика",
			"answers":      []map[string]string{{"label": "Автомобиль", "value": brand + " Camry"}, {"label": "Что случилось", "value": "Стучит подвеска"}},
			"message":      "стучит подвеска на " + brand,
			"phone":        "+7 900 111-22-33",
		})
	}
	expect(t, newReq("111111", 0, "Toyota"), 400, "no telegram id")
	expect(t, newReq("999999", 42, "Toyota"), 404, "unknown bot")
	r1 := newReq("111111", 42, "Toyota")
	expect(t, r1, 201, "create request 1")
	if r1.body["status"] != "new" || r1.body["number"].(float64) != 1 || r1.body["phone"] != "+79001112233" {
		t.Fatalf("unexpected request: %v", r1.body)
	}
	id1 := r1.body["id"].(string)
	time.Sleep(10 * time.Millisecond)
	r2 := newReq("111111", 42, "Lada")
	expect(t, r2, 201, "create request 2")
	id2 := r2.body["id"].(string)
	r3 := newReq("111111", 77, "BMW")
	expect(t, r3, 201, "another client")
	rb := newReq("222222", 42, "Kia")
	expect(t, rb, 201, "same telegram user in autoservice B")
	if r3.body["number"].(float64) != 3 || rb.body["number"].(float64) != 1 {
		t.Fatalf("requests must be numbered per autoservice: A %v, B %v", r3.body["number"], rb.body["number"])
	}

	list := a.do("GET", "/api/v1/requests", tokA, nil)
	expect(t, list, 200, "list A")
	if list.body["total"].(float64) != 3 {
		t.Fatalf("A should see 3 requests: %v", list.body)
	}
	first := list.body["items"].([]any)[0].(map[string]any)
	if first["number"].(float64) != 3 || first["service_name"] != "Диагностика" ||
		first["answers"].([]any)[0].(map[string]any)["value"] != "BMW Camry" || first["message"] != "стучит подвеска на BMW" {
		t.Fatalf("default sort must be newest first, got %v", first)
	}
	if counts := list.body["counts"].(map[string]any); counts["new"].(float64) != 3 || counts["closed"].(float64) != 0 {
		t.Fatalf("unexpected counts: %v", counts)
	}
	asc := a.do("GET", "/api/v1/requests?sort=asc", tokA, nil)
	if asc.body["items"].([]any)[0].(map[string]any)["id"] != id1 {
		t.Fatal("asc sort broken")
	}
	expect(t, a.do("GET", "/api/v1/requests?status=bogus", tokA, nil), 400, "bad status filter")
	expect(t, a.do("GET", "/api/v1/requests/"+id1, tokB, nil), 404, "B cannot see A's request")

	full := a.do("GET", "/api/v1/requests/"+id1, tokA, nil)
	expect(t, full, 200, "get request")
	client := full.body["client"].(map[string]any)
	if client["name"] != "Петя" || client["phone"] != "+79001112233" {
		t.Fatalf("client contacts missing: %v", client)
	}

	clients := a.do("GET", "/api/v1/clients", tokA, nil)
	expect(t, clients, 200, "clients")
	if clients.body["total"].(float64) != 2 {
		t.Fatalf("A should have 2 clients: %v", clients.body)
	}
	cl := a.do("GET", "/api/v1/clients/"+client["id"].(string), tokA, nil)
	expect(t, cl, 200, "client details")
	if len(cl.body["requests"].([]any)) != 2 {
		t.Fatalf("client should have 2 requests: %v", cl.body)
	}
	expect(t, a.do("GET", "/api/v1/clients/"+client["id"].(string), tokB, nil), 404, "B cannot see A's client")

	mine := a.runner("GET", "/bots/111111/clients/42/requests", nil)
	expect(t, mine, 200, "client requests")
	if mine.body["total"].(float64) != 2 {
		t.Fatalf("client 42 should have 2 requests in A: %v", mine.body)
	}
	expect(t, a.runner("GET", "/bots/111111/clients/77/requests/"+id1, nil), 404, "other client's request")
	empty := a.runner("GET", "/bots/111111/clients/5555/requests", nil)
	expect(t, empty, 200, "unknown client has no requests")
	edit := map[string]any{"service_name": "Ремонт", "answers": []map[string]string{{"label": "Автомобиль", "value": "Toyota Corolla"}}, "phone": "8 900 111-22-33"}
	r = a.runner("PATCH", "/bots/111111/clients/42/requests/"+id1, edit)
	expect(t, r, 200, "client edits new request")
	if r.body["service_name"] != "Ремонт" || len(r.body["answers"].([]any)) != 1 || r.body["phone"] != "+79001112233" {
		t.Fatalf("not edited: %v", r.body)
	}
	expect(t, a.runner("POST", "/bots/111111/requests", map[string]any{
		"client": map[string]any{"telegram_id": 42}, "service_name": "Ремонт", "phone": "не скажу"}), 400, "bad phone")
	expect(t, a.runner("POST", "/bots/111111/clients/42/requests/"+id2+"/cancel", nil), 200, "client cancels")
	expect(t, a.runner("POST", "/bots/111111/clients/42/requests/"+id2+"/cancel", nil), 409, "cancel twice")

	expect(t, a.do("POST", "/api/v1/requests/"+id2+"/take", tokA, nil), 409, "take cancelled")
	expect(t, a.do("POST", "/api/v1/requests/"+id1+"/close", tokA, nil), 409, "close new")
	expect(t, a.do("PATCH", "/api/v1/requests/"+id1+"/status", tokA, map[string]string{"status": "cancelled"}), 409, "employee cannot cancel")
	r = a.do("POST", "/api/v1/requests/"+id1+"/take", tokA, nil)
	expect(t, r, 200, "take")
	if r.body["status"] != "in_progress" {
		t.Fatalf("status %v", r.body["status"])
	}
	expect(t, a.runner("PATCH", "/bots/111111/clients/42/requests/"+id1, edit), 409, "client edits in-progress")
	expect(t, a.runner("POST", "/bots/111111/clients/42/requests/"+id1+"/cancel", nil), 409, "client cancels in-progress")
	expect(t, a.do("POST", "/api/v1/requests/"+id1+"/reject", tokA, nil), 409, "reject in-progress")
	expect(t, a.do("PATCH", "/api/v1/requests/"+id1+"/status", tokA, map[string]string{"status": "closed"}), 200, "close")

	st := a.do("GET", "/api/v1/requests?status=closed", tokA, nil)
	if st.body["total"].(float64) != 1 || st.body["counts"].(map[string]any)["cancelled"].(float64) != 1 {
		t.Fatalf("status filter: %v", st.body)
	}

	r = a.do("PUT", "/api/v1/requests/"+id1+"/note", tokA, map[string]string{"note": " Перезвонить после 18:00 "})
	expect(t, r, 200, "save note")
	if r.body["note"] != "Перезвонить после 18:00" || r.body["status"] != "closed" {
		t.Fatalf("note not saved: %v", r.body)
	}
	expect(t, a.do("PUT", "/api/v1/requests/"+id1+"/note", tokB, map[string]string{"note": "x"}), 404, "B cannot annotate A's request")

	for i := range 5 {
		expect(t, newReq("222222", 500, "Kia"), 201, fmt.Sprintf("open request %d", i+1))
	}
	r = newReq("222222", 500, "Kia")
	expect(t, r, 429, "too many open requests of one client")
	if errCode(r) != "too_many_requests" {
		t.Fatalf("code %s", errCode(r))
	}

	expect(t, a.do("DELETE", "/api/v1/autoservice/bot", tokB, nil), 204, "disconnect B bot")
	expect(t, a.do("GET", "/api/v1/autoservice/bot", tokB, nil), 404, "B bot gone")
	expect(t, a.do("DELETE", "/api/v1/autoservice", tokA, nil), 204, "delete A")
	expect(t, a.do("GET", "/api/v1/autoservice", tokA, nil), 404, "A gone")
	expect(t, a.runner("GET", "/bots/111111/clients/42/requests", nil), 404, "A bot gone with autoservice")
	expect(t, a.do("POST", "/api/v1/autoservice", tokA, map[string]string{"name": "Новый сервис"}), 201, "recreate A")
	expect(t, a.do("PUT", "/api/v1/autoservice/bot", tokA, map[string]string{"token": tokenA}), 200, "bot A free again")
	list = a.do("GET", "/api/v1/requests", tokA, nil)
	if list.body["total"].(float64) != 0 {
		t.Fatalf("old requests must be deleted: %v", list.body)
	}
	cls := a.do("GET", "/api/v1/clients", tokA, nil)
	if cls.body["total"].(float64) != 0 {
		t.Fatalf("old clients must be deleted: %v", cls.body)
	}
}

func items(r resp) []map[string]any {
	raw, _ := r.body["items"].([]any)
	out := make([]map[string]any, len(raw))
	for i, v := range raw {
		out[i] = v.(map[string]any)
	}
	return out
}

func names(list []map[string]any) string {
	var out []string
	for _, s := range list {
		out = append(out, s["name"].(string))
	}
	return strings.Join(out, ", ")
}

func TestServiceCatalogFlow(t *testing.T) {
	a := setup(t)

	// Registration with an autoservice name creates the autoservice and its starter catalog in one step.
	r := a.do("POST", "/api/v1/auth/register", "", map[string]string{
		"email": "owner@example.com", "password": "password123", "full_name": "Иван", "autoservice_name": "Гараж №1"})
	expect(t, r, 201, "register with autoservice")
	tok := r.body["access_token"].(string)
	as := a.do("GET", "/api/v1/autoservice", tok, nil)
	expect(t, as, 200, "autoservice created on registration")
	if as.body["name"] != "Гараж №1" {
		t.Fatalf("unexpected autoservice: %v", as.body)
	}
	expect(t, a.do("POST", "/api/v1/auth/register", "", map[string]string{
		"email": "bad@example.com", "password": "password123", "full_name": "x", "autoservice_name": strings.Repeat("я", 201)}), 400, "bad autoservice name")
	expect(t, a.do("POST", "/api/v1/auth/login", "", map[string]string{"email": "bad@example.com", "password": "password123"}), 401,
		"a rejected registration must not leave an account behind")

	list := a.do("GET", "/api/v1/services", tok, nil)
	expect(t, list, 200, "list services")
	defaults := items(list)
	if got := names(defaults); got != "Диагностика, Техобслуживание и замена масла, Ремонт, Шиномонтаж, Другой вопрос" {
		t.Fatalf("unexpected starter catalog: %s", got)
	}
	field := defaults[0]["fields"].([]any)[0].(map[string]any)
	if field["label"] != "Автомобиль" || field["type"] != "text" || field["required"] != true || field["id"] == "" {
		t.Fatalf("unexpected field: %v", field)
	}
	r = a.do("POST", "/api/v1/services/defaults", tok, nil)
	expect(t, r, 409, "defaults only for an empty catalog")
	if errCode(r) != "services_exist" {
		t.Fatalf("code %s", errCode(r))
	}

	expect(t, a.do("POST", "/api/v1/services", tok, map[string]any{"name": ""}), 400, "empty name")
	r = a.do("POST", "/api/v1/services", tok, map[string]any{"name": "x", "fields": []map[string]any{{"label": "a", "type": "date"}}})
	expect(t, r, 400, "unknown field type")
	if e := r.body["error"].(map[string]any); e["field"] != "fields[0]" {
		t.Fatalf("validation error must point at the field: %v", e)
	}

	r = a.do("POST", "/api/v1/services", tok, map[string]any{
		"name": "Автомойка", "description": "Мойка кузова и салона",
		"fields": []map[string]any{
			{"label": "Тип мойки", "question": "Какая мойка нужна?", "type": "choice", "required": true, "options": []string{"Кузов", "Комплекс"}},
			{"label": "Комментарий", "type": "text"},
		}})
	expect(t, r, 201, "create service")
	wash := r.body
	washID := wash["id"].(string)
	fields := wash["fields"].([]any)
	if wash["is_active"] != true || len(fields) != 2 || fields[0].(map[string]any)["id"] == fields[1].(map[string]any)["id"] {
		t.Fatalf("unexpected service: %v", wash)
	}
	choiceID := fields[0].(map[string]any)["id"]

	// Updating keeps the IDs of existing fields, so half-filled bot dialogs stay valid.
	r = a.do("PUT", "/api/v1/services/"+washID, tok, map[string]any{
		"name": "Мойка", "is_active": false,
		"fields": []map[string]any{
			{"id": choiceID, "label": "Вид мойки", "type": "choice", "required": true, "options": []string{"Кузов", "Комплекс", "Двигатель"}},
		}})
	expect(t, r, 200, "update service")
	updated := r.body["fields"].([]any)
	if r.body["name"] != "Мойка" || r.body["is_active"] != false || len(updated) != 1 || updated[0].(map[string]any)["id"] != choiceID {
		t.Fatalf("unexpected update: %v", r.body)
	}

	order := []string{washID, defaults[4]["id"].(string)}
	r = a.do("PUT", "/api/v1/services/order", tok, map[string]any{"ids": order})
	expect(t, r, 200, "reorder")
	if got := names(items(r)); got != "Мойка, Другой вопрос, Диагностика, Техобслуживание и замена масла, Ремонт, Шиномонтаж" {
		t.Fatalf("unexpected order: %s", got)
	}

	other := register(a, "other@example.com")
	expect(t, a.do("POST", "/api/v1/autoservice", other, map[string]string{"name": "Чужой"}), 201, "second autoservice")
	expect(t, a.do("PUT", "/api/v1/services/"+washID, other, map[string]any{"name": "Взлом"}), 404, "foreign service is invisible")
	expect(t, a.do("DELETE", "/api/v1/services/"+washID, other, nil), 404, "foreign service cannot be deleted")
	if got := len(items(a.do("GET", "/api/v1/services", other, nil))); got != 5 {
		t.Fatalf("every autoservice gets its own starter catalog, got %d services", got)
	}

	expect(t, a.do("DELETE", "/api/v1/services/"+washID, tok, nil), 204, "delete service")
	expect(t, a.do("DELETE", "/api/v1/services/"+washID, tok, nil), 404, "delete twice")
	for _, s := range defaults {
		expect(t, a.do("DELETE", "/api/v1/services/"+s["id"].(string), tok, nil), 204, "delete default")
	}
	r = a.do("POST", "/api/v1/services/defaults", tok, nil)
	expect(t, r, 201, "restore defaults into an empty catalog")
	if len(items(r)) != 5 {
		t.Fatalf("defaults not restored: %v", r.body)
	}

	for i := len(items(r)); i < 30; i++ {
		expect(t, a.do("POST", "/api/v1/services", tok, map[string]any{"name": fmt.Sprintf("Услуга %d", i)}), 201, "fill the catalog")
	}
	r = a.do("POST", "/api/v1/services", tok, map[string]any{"name": "Лишняя"})
	expect(t, r, 409, "catalog size limit")
	if errCode(r) != "service_limit" {
		t.Fatalf("code %s", errCode(r))
	}
}

func TestInternalAPIIsOffWithoutKey(t *testing.T) {
	a := setup(t)
	a.cfg.BotAPIKey = ""
	router, err := app.NewRouter(a.cfg, a.pool, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/internal/v1/bots")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("internal API must not be mounted without BOT_API_KEY, got %d", res.StatusCode)
	}
}

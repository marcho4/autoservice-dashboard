package llm

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/bot"
	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/testkit"
)

var services = []domain.Service{
	{ID: uuid.New(), Name: "Диагностика", Description: "Клиент не знает причину", Fields: []domain.FormField{
		{ID: "car1", Label: "Автомобиль", Type: domain.FieldText},
	}},
	{ID: uuid.New(), Name: "Шиномонтаж", Fields: []domain.FormField{
		{ID: "work", Label: "Вид работ", Type: domain.FieldChoice, Options: []string{"Смена шин", "Балансировка"}},
		{ID: "size", Label: "Диаметр", Type: domain.FieldNumber},
	}},
}

func extract(t *testing.T, srv *testkit.LLM, message string) (bot.Extraction, error) {
	t.Helper()
	c := New(Config{BaseURL: srv.URL() + "/", APIKey: "sk-test", Model: "test-model"})
	return c.Extract(context.Background(), bot.ExtractInput{Autoservice: "Гараж", Services: services, Message: message})
}

func TestExtract(t *testing.T) {
	srv := testkit.NewLLM()
	defer srv.Close()
	srv.SetReply(func(string, string) (int, string) {
		return http.StatusOK, "```json\n{\"service\": 2, \"fields\": {\"work\": \"Смена шин\", \"size\": 16, \"car1\": null, \"x\": [1]}, \"phone\": \"+7 900 111-22-33\"}\n```"
	})

	ext, err := extract(t, srv, "переобуться на R16, тел +7 900 111-22-33")
	if err != nil {
		t.Fatal(err)
	}
	if ext.ServiceID != services[1].ID || ext.Phone != "+7 900 111-22-33" ||
		len(ext.Values) != 2 || ext.Values["work"] != "Смена шин" || ext.Values["size"] != "16" {
		t.Errorf("unexpected extraction: %+v", ext)
	}

	calls := srv.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected one LLM call, got %d", len(calls))
	}
	call := calls[0]
	if call.Model != "test-model" || call.AuthHeader != "Bearer sk-test" || !call.JSONMode {
		t.Errorf("unexpected request: %+v", call)
	}
	if call.User != "переобуться на R16, тел +7 900 111-22-33" {
		t.Errorf("the client message must go as the user turn: %q", call.User)
	}
	for _, part := range []string{"«Гараж»", `"key": "2"`, "Шиномонтаж", "Клиент не знает причину", `"id": "work"`, "Балансировка"} {
		if !strings.Contains(call.System, part) {
			t.Errorf("system prompt must contain %q", part)
		}
	}
	if strings.Contains(call.System, services[0].ID.String()) {
		t.Error("service UUIDs must not be sent to the model")
	}
}

func TestExtractNoService(t *testing.T) {
	srv := testkit.NewLLM()
	defer srv.Close()
	for _, answer := range []string{`{"service": null, "fields": {}, "phone": null}`, `{"service": "7"}`, `{"service": "null", "phone": "null"}`} {
		srv.SetReply(func(string, string) (int, string) { return http.StatusOK, answer })
		ext, err := extract(t, srv, "привет")
		if err != nil || ext.ServiceID != uuid.Nil || ext.Phone != "" || len(ext.Values) != 0 {
			t.Errorf("%s: expected an empty extraction, got %+v, %v", answer, ext, err)
		}
	}
}

func TestExtractErrors(t *testing.T) {
	srv := testkit.NewLLM()
	defer srv.Close()

	srv.SetReply(func(string, string) (int, string) {
		return http.StatusOK, "Конечно! Клиент хочет шиномонтаж."
	})
	if _, err := extract(t, srv, "x"); err == nil {
		t.Error("an answer without JSON must be an error")
	}
	srv.SetReply(func(string, string) (int, string) {
		return http.StatusUnauthorized, "bad key sk-test for client +79001112233"
	})
	if _, err := extract(t, srv, "x"); err == nil || strings.Contains(err.Error(), "sk-test") || strings.Contains(err.Error(), "+7900") {
		t.Errorf("expected an error with the status only, got %v", err)
	}
}

func TestExtractDoesNotSendHugeCatalog(t *testing.T) {
	srv := testkit.NewLLM()
	defer srv.Close()
	huge := []domain.Service{{ID: uuid.New(), Name: "Мойка", Description: strings.Repeat("я", maxCatalogLength)}}
	c := New(Config{BaseURL: srv.URL(), Model: "test-model"})
	if _, err := c.Extract(context.Background(), bot.ExtractInput{Services: huge, Message: "помыть"}); err == nil {
		t.Error("a catalog over the limit must be an error")
	}
	if n := len(srv.Calls()); n != 0 {
		t.Errorf("a catalog over the limit must not reach the LLM, got %d calls", n)
	}
}

func TestExtractFallsBackWithoutJSONMode(t *testing.T) {
	srv := testkit.NewLLM()
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL(), Model: "local"})
	in := bot.ExtractInput{Services: services, Message: "диагностика"}

	srv.SetReply(func(string, string) (int, string) {
		if last := srv.Calls(); last[len(last)-1].JSONMode {
			return http.StatusBadRequest, "response_format is not supported"
		}
		return http.StatusOK, `{"service": "1"}`
	})
	for range 2 {
		ext, err := c.Extract(context.Background(), in)
		if err != nil || ext.ServiceID != services[0].ID {
			t.Fatalf("unexpected result: %+v, %v", ext, err)
		}
	}
	calls := srv.Calls()
	if len(calls) != 3 || !calls[0].JSONMode || calls[1].JSONMode || calls[2].JSONMode {
		t.Errorf("expected json mode to be dropped after the first rejection: %+v", calls)
	}
	if calls[0].AuthHeader != "" {
		t.Errorf("no Authorization header without an API key: %q", calls[0].AuthHeader)
	}
}

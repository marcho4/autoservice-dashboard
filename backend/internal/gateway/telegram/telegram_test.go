package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCallWaitsOutShortFloodControl(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	}))
	defer srv.Close()

	if err := New(srv.URL).SendMessage(context.Background(), "1:x", OutMessage{ChatID: 1, Text: "Заявка принята"}); err != nil {
		t.Fatalf("the message must be sent after the flood-control pause: %v", err)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("expected one retry, got %d calls", n)
	}
}

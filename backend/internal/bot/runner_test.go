package bot

import (
	"io"
	"log/slog"
	"testing"

	"github.com/VictoriaMetrics/metrics"
)

func TestPanicInHandlerDoesNotStopTheChat(t *testing.T) {
	failures := metrics.GetOrCreateCounter("bot_update_errors_total")
	before := failures.Get()
	q := newChatQueue(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handled := false
	q.submit(clientChat, func() { panic("boom") })
	q.submit(clientChat, func() { handled = true })
	q.wait()
	if !handled {
		t.Error("the next update of the chat must still be handled")
	}
	if got := failures.Get() - before; got != 1 {
		t.Errorf("the panic must be counted in bot_update_errors_total, got %d", got)
	}
}

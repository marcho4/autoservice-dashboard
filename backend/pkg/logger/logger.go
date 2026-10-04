package logger

import (
	"log/slog"
	"os"
	"strings"
	"time"
)

func SetupLogging(appName string) *slog.Logger {
	logger := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Key = "timestamp"
				a.Value = slog.Int64Value(time.Now().UnixMilli())
			case slog.MessageKey:
				a.Key = "message"
			case slog.LevelKey:
				a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
			}
			return a
		},
	})
	l := slog.New(logger).With("service.name", appName)
	slog.SetDefault(l)
	return l
}

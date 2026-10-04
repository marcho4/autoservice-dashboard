package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/marcho4/autoservice-dashboard/backend/pkg/logger"
)

const usage = `Usage: app <command>

Commands:
  serve     start the HTTP API server
  migrate   apply pending database migrations
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	_ = logger.SetupLogging("autoservice-backend")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(ctx)
	case "migrate":
		err = migrate(ctx)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		slog.Error("command failed", "command", os.Args[1], "error", err)
		os.Exit(1)
	}
}

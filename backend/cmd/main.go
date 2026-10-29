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
  serve [spam]  start the HTTP API server; spam adds 150 goroutines for 15 minutes
  migrate       apply pending database migrations
  probe         run the black-box prober and expose its metrics
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
		spamGo := len(os.Args) > 2 && os.Args[2] == "spam"
		err = serve(ctx, spamGo)
	case "migrate":
		err = migrate(ctx)
	case "probe":
		err = probe(ctx)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		slog.Error("command failed", "command", os.Args[1], "error", err)
		os.Exit(1)
	}
}

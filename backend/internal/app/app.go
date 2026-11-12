package app

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/adapters/httpapi"
	"github.com/marcho4/autoservice-dashboard/backend/internal/bot"
	"github.com/marcho4/autoservice-dashboard/backend/internal/config"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/llm"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/postgres"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/telegram"
	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
	"github.com/marcho4/autoservice-dashboard/backend/pkg/jwtauth"
	"github.com/marcho4/autoservice-dashboard/backend/pkg/password"
)

func botRunnerService(pool *pgxpool.Pool) *usecases.BotRunnerService {
	return usecases.NewBotRunnerService(postgres.NewBotRepo(pool), postgres.NewAutoserviceRepo(pool),
		postgres.NewServiceRepo(pool), postgres.NewClientRepo(pool), postgres.NewRequestRepo(pool))
}

func NewRouter(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (http.Handler, error) {
	tokens, err := jwtauth.New(cfg.JWTSecret, cfg.JWTTTL)
	if err != nil {
		return nil, fmt.Errorf("JWT_SECRET: %w", err)
	}
	hasher := password.Bcrypt{}

	employees := postgres.NewEmployeeRepo(pool)
	clients := postgres.NewClientRepo(pool)
	requests := postgres.NewRequestRepo(pool)

	autoservices := usecases.NewAutoserviceService(postgres.NewAutoserviceRepo(pool), postgres.NewBotRepo(pool), telegram.New(cfg.TelegramAPIURL))
	auth := usecases.NewAuthService(employees, postgres.NewRevokedTokenRepo(pool), hasher, tokens, autoservices)

	return httpapi.NewRouter(httpapi.Deps{
		Auth:        auth,
		Account:     usecases.NewAccountService(employees, hasher, auth),
		Autoservice: autoservices,
		Catalog:     usecases.NewCatalogService(autoservices, postgres.NewServiceRepo(pool)),
		Dashboard:   usecases.NewDashboardService(autoservices, clients, requests),
		BotRunner:   botRunnerService(pool),
		BotAPIKey:   cfg.BotAPIKey,
		Logger:      log,
		Ready:       pool.Ping,
	}), nil
}

// NewBotRunner builds the built-in Telegram bot: it polls every connected bot and talks to clients.
func NewBotRunner(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) *bot.Runner {
	service := botRunnerService(pool)

	var extractor bot.Extractor
	if cfg.LLMModel != "" {
		extractor = llm.New(llm.Config{BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel, Timeout: cfg.LLMTimeout})
		log.Info("bot uses LLM to recognize requests", "model", cfg.LLMModel, "base_url", cfg.LLMBaseURL)
		if cfg.LLMAPIKey == "" {
			log.Warn("LLM_API_KEY is empty: fine for a local model server, but a hosted API will reject the requests")
		}
	} else {
		log.Warn("LLM_MODEL is not set: the bot will ask clients to pick a service with buttons")
	}

	engine := bot.NewEngine(service, postgres.NewConversationRepo(pool), extractor, cfg.AppURL, log)
	return bot.NewRunner(telegram.New(cfg.TelegramAPIURL), service, engine, postgres.NewLeaderLock(pool), cfg.Runner, log)
}

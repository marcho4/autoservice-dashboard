package app

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/adapters/httpapi"
	"github.com/marcho4/autoservice-dashboard/backend/internal/config"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/postgres"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/telegram"
	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
	"github.com/marcho4/autoservice-dashboard/backend/pkg/jwtauth"
	"github.com/marcho4/autoservice-dashboard/backend/pkg/password"
)

func NewRouter(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (http.Handler, error) {
	tokens, err := jwtauth.New(cfg.JWTSecret, cfg.JWTTTL)
	if err != nil {
		return nil, fmt.Errorf("JWT_SECRET: %w", err)
	}
	hasher := password.Bcrypt{}

	employees := postgres.NewEmployeeRepo(pool)
	bots := postgres.NewBotRepo(pool)
	clients := postgres.NewClientRepo(pool)
	requests := postgres.NewRequestRepo(pool)

	auth := usecases.NewAuthService(employees, postgres.NewRevokedTokenRepo(pool), hasher, tokens)
	autoservices := usecases.NewAutoserviceService(postgres.NewAutoserviceRepo(pool), bots, telegram.New(cfg.TelegramAPIURL))

	return httpapi.NewRouter(httpapi.Deps{
		Auth:        auth,
		Account:     usecases.NewAccountService(employees, hasher, auth),
		Autoservice: autoservices,
		Dashboard:   usecases.NewDashboardService(autoservices, clients, requests),
		BotRunner:   usecases.NewBotRunnerService(bots, clients, requests),
		BotAPIKey:   cfg.BotAPIKey,
		Logger:      log,
		Ready:       pool.Ping,
	}), nil
}

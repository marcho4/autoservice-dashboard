package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
)

type Handler struct {
	auth         *usecases.AuthService
	account      *usecases.AccountService
	autoservices *usecases.AutoserviceService
	dashboard    *usecases.DashboardService
	botRunner    *usecases.BotRunnerService
}

type Deps struct {
	Auth        *usecases.AuthService
	Account     *usecases.AccountService
	Autoservice *usecases.AutoserviceService
	Dashboard   *usecases.DashboardService
	BotRunner   *usecases.BotRunnerService
	BotAPIKey   string
	Logger      *slog.Logger
	Ready       func(ctx context.Context) error
}

func NewRouter(d Deps) http.Handler {
	h := &Handler{
		auth:         d.Auth,
		account:      d.Account,
		autoservices: d.Autoservice,
		dashboard:    d.Dashboard,
		botRunner:    d.BotRunner,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogger(d.Logger))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := d.Ready(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", "database is not reachable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", h.register)
		r.Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(h.requireEmployee)
			r.Post("/auth/logout", h.logout)

			r.Get("/me", h.getMe)
			r.Patch("/me", h.updateMe)
			r.Put("/me/password", h.changePassword)
			r.Delete("/me", h.deleteMe)

			r.Post("/autoservice", h.createAutoservice)
			r.Get("/autoservice", h.getAutoservice)
			r.Put("/autoservice", h.updateAutoservice)
			r.Delete("/autoservice", h.deleteAutoservice)

			r.Get("/autoservice/bot", h.getBot)
			r.Put("/autoservice/bot", h.connectBot)
			r.Delete("/autoservice/bot", h.disconnectBot)

			r.Get("/requests", h.listRequests)
			r.Get("/requests/{id}", h.getRequest)
			r.Patch("/requests/{id}/status", h.changeStatus)
			r.Post("/requests/{id}/take", h.statusAction(domain.StatusInProgress))
			r.Post("/requests/{id}/reject", h.statusAction(domain.StatusRejected))
			r.Post("/requests/{id}/close", h.statusAction(domain.StatusClosed))

			r.Get("/clients", h.listClients)
			r.Get("/clients/{id}", h.getClient)
		})
	})

	r.Route("/internal/v1", func(r chi.Router) {
		r.Use(requireAPIKey(d.BotAPIKey))
		r.Get("/bots", h.runnerListBots)
		r.Post("/bots/{botID}/requests", h.runnerCreateRequest)
		r.Get("/bots/{botID}/clients/{telegramID}/requests", h.runnerListRequests)
		r.Get("/bots/{botID}/clients/{telegramID}/requests/{id}", h.runnerGetRequest)
		r.Patch("/bots/{botID}/clients/{telegramID}/requests/{id}", h.runnerUpdateRequest)
		r.Post("/bots/{botID}/clients/{telegramID}/requests/{id}/cancel", h.runnerCancelRequest)
	})

	return r
}

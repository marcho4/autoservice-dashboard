package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
)

type loggerKey struct{}
type principalKey struct{}

func requestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			l := base.With("request_id", middleware.GetReqID(r.Context()))
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				if rec := recover(); rec != nil {
					l.Error("panic", "panic", rec, "stack", string(debug.Stack()))
					if ww.Status() == 0 {
						writeError(ww, http.StatusInternalServerError, "internal", "internal server error")
					}
				}
				l.Info("http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"bytes", ww.BytesWritten(),
					"duration_ms", time.Since(start).Milliseconds(),
				)
			}()
			next.ServeHTTP(ww, r.WithContext(context.WithValue(r.Context(), loggerKey{}, l)))
		})
	}
}

func logger(r *http.Request) *slog.Logger {
	if l, ok := r.Context().Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

func (h *Handler) requireEmployee(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		p, err := h.auth.Authenticate(r.Context(), token)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, p)
		ctx = context.WithValue(ctx, loggerKey{}, logger(r).With("employee_id", p.Employee.ID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func principal(r *http.Request) usecases.Principal {
	return r.Context().Value(principalKey{}).(usecases.Principal)
}

func requireAPIKey(key string) func(http.Handler) http.Handler {
	want := sha256.Sum256([]byte(key))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := sha256.Sum256([]byte(r.Header.Get("X-API-Key")))
			if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid API key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

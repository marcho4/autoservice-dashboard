package httpapi

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// The internal API lets an external integration act as a bot of an autoservice.
// The built-in Telegram bot does not use it; it is mounted only when BOT_API_KEY is set.

type botAutoserviceKey struct{}

// resolveBot maps the {botID} path parameter to the autoservice of that bot.
func (h *Handler) resolveBot(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		botID, ok := int64Param(w, r, "botID")
		if !ok {
			return
		}
		autoserviceID, err := h.botRunner.ResolveBot(r.Context(), botID)
		if err != nil {
			writeDomainError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), botAutoserviceKey{}, autoserviceID)))
	})
}

func botAutoservice(r *http.Request) uuid.UUID {
	return r.Context().Value(botAutoserviceKey{}).(uuid.UUID)
}

func (h *Handler) runnerListBots(w http.ResponseWriter, r *http.Request) {
	bots, err := h.botRunner.ListBots(r.Context())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listDTO[runnerBotDTO]{Items: mapAll(bots, toRunnerBot), Total: len(bots)})
}

func (h *Handler) runnerCreateRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Client clientIdentityDTO `json:"client"`
		requestInputDTO
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	req, err := h.botRunner.CreateRequest(r.Context(), botAutoservice(r), body.Client.toDomain(), body.requestInputDTO.toDomain())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRequest(req))
}

func (h *Handler) runnerListRequests(w http.ResponseWriter, r *http.Request) {
	telegramID, ok := int64Param(w, r, "telegramID")
	if !ok {
		return
	}
	requests, err := h.botRunner.ListClientRequests(r.Context(), botAutoservice(r), telegramID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listDTO[requestDTO]{Items: mapAll(requests, toRequest), Total: len(requests)})
}

func (h *Handler) runnerGetRequest(w http.ResponseWriter, r *http.Request) {
	telegramID, requestID, ok := clientRequestParams(w, r)
	if !ok {
		return
	}
	req, err := h.botRunner.GetClientRequest(r.Context(), botAutoservice(r), telegramID, requestID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRequest(req))
}

func (h *Handler) runnerUpdateRequest(w http.ResponseWriter, r *http.Request) {
	telegramID, requestID, ok := clientRequestParams(w, r)
	if !ok {
		return
	}
	var body requestInputDTO
	if !decodeJSON(w, r, &body) {
		return
	}
	req, err := h.botRunner.UpdateClientRequest(r.Context(), botAutoservice(r), telegramID, requestID, body.toDomain())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRequest(req))
}

func (h *Handler) runnerCancelRequest(w http.ResponseWriter, r *http.Request) {
	telegramID, requestID, ok := clientRequestParams(w, r)
	if !ok {
		return
	}
	req, err := h.botRunner.CancelClientRequest(r.Context(), botAutoservice(r), telegramID, requestID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRequest(req))
}

func clientRequestParams(w http.ResponseWriter, r *http.Request) (telegramID int64, requestID uuid.UUID, ok bool) {
	telegramID, ok = int64Param(w, r, "telegramID")
	if !ok {
		return 0, uuid.Nil, false
	}
	requestID, ok = uuidParam(w, r, "id")
	return telegramID, requestID, ok
}

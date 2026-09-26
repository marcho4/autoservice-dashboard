package httpapi

import (
	"net/http"

	"github.com/google/uuid"
)

func (h *Handler) runnerListBots(w http.ResponseWriter, r *http.Request) {
	bots, err := h.botRunner.ListBots(r.Context())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listDTO[runnerBotDTO]{Items: mapAll(bots, toRunnerBot), Total: len(bots)})
}

func (h *Handler) runnerCreateRequest(w http.ResponseWriter, r *http.Request) {
	botID, ok := int64Param(w, r, "botID")
	if !ok {
		return
	}
	var body struct {
		Client clientIdentityDTO `json:"client"`
		requestInputDTO
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	req, err := h.botRunner.CreateRequest(r.Context(), botID, body.Client.toDomain(), body.requestInputDTO.toDomain())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRequest(req))
}

func (h *Handler) runnerListRequests(w http.ResponseWriter, r *http.Request) {
	botID, telegramID, ok := clientParams(w, r)
	if !ok {
		return
	}
	requests, err := h.botRunner.ListClientRequests(r.Context(), botID, telegramID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listDTO[requestDTO]{Items: mapAll(requests, toRequest), Total: len(requests)})
}

func (h *Handler) runnerGetRequest(w http.ResponseWriter, r *http.Request) {
	botID, telegramID, requestID, ok := clientRequestParams(w, r)
	if !ok {
		return
	}
	req, err := h.botRunner.GetClientRequest(r.Context(), botID, telegramID, requestID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRequest(req))
}

func (h *Handler) runnerUpdateRequest(w http.ResponseWriter, r *http.Request) {
	botID, telegramID, requestID, ok := clientRequestParams(w, r)
	if !ok {
		return
	}
	var body requestInputDTO
	if !decodeJSON(w, r, &body) {
		return
	}
	req, err := h.botRunner.UpdateClientRequest(r.Context(), botID, telegramID, requestID, body.toDomain())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRequest(req))
}

func (h *Handler) runnerCancelRequest(w http.ResponseWriter, r *http.Request) {
	botID, telegramID, requestID, ok := clientRequestParams(w, r)
	if !ok {
		return
	}
	req, err := h.botRunner.CancelClientRequest(r.Context(), botID, telegramID, requestID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRequest(req))
}

func clientParams(w http.ResponseWriter, r *http.Request) (botID, telegramID int64, ok bool) {
	botID, ok = int64Param(w, r, "botID")
	if !ok {
		return 0, 0, false
	}
	telegramID, ok = int64Param(w, r, "telegramID")
	return botID, telegramID, ok
}

func clientRequestParams(w http.ResponseWriter, r *http.Request) (botID, telegramID int64, requestID uuid.UUID, ok bool) {
	botID, telegramID, ok = clientParams(w, r)
	if !ok {
		return 0, 0, uuid.Nil, false
	}
	requestID, ok = uuidParam(w, r, "id")
	return botID, telegramID, requestID, ok
}

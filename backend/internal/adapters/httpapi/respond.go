package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Error: errorPayload{Code: code, Message: msg}})
}

var domainErrors = []struct {
	err    error
	status int
	code   string
}{
	{domain.ErrNotFound, http.StatusNotFound, "not_found"},
	{domain.ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
	{domain.ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials"},
	{domain.ErrEmailTaken, http.StatusConflict, "email_taken"},
	{domain.ErrAutoserviceExists, http.StatusConflict, "autoservice_exists"},
	{domain.ErrNoAutoservice, http.StatusNotFound, "no_autoservice"},
	{domain.ErrBotAlreadyLinked, http.StatusConflict, "bot_already_linked"},
	{domain.ErrBotNotConnected, http.StatusNotFound, "bot_not_connected"},
	{domain.ErrInvalidBotToken, http.StatusUnprocessableEntity, "invalid_bot_token"},
	{domain.ErrInvalidTransition, http.StatusConflict, "invalid_transition"},
	{domain.ErrRequestNotEditable, http.StatusConflict, "request_not_editable"},
}

func writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: errorPayload{Code: "validation_error", Message: ve.Error(), Field: ve.Field}})
		return
	}
	for _, m := range domainErrors {
		if errors.Is(err, m.err) {
			writeError(w, m.status, m.code, m.err.Error())
			return
		}
	}
	logger(r).Error("internal error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal server error")
}

const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		msg := "invalid JSON body"
		if errors.Is(err, io.EOF) {
			msg = "request body is empty"
		}
		writeError(w, http.StatusBadRequest, "bad_request", msg)
		return false
	}
	return true
}

func uuidParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return uuid.Nil, false
	}
	return id, true
}

func int64Param(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", name+" must be an integer")
		return 0, false
	}
	return v, true
}

func intQuery(w http.ResponseWriter, q url.Values, name string) (int, bool) {
	v := q.Get(name)
	if v == "" {
		return 0, true
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", name+" must be an integer")
		return 0, false
	}
	return n, true
}

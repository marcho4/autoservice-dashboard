package httpapi

import (
	"net/http"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
)

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		FullName string `json:"full_name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	s, err := h.auth.Register(r.Context(), usecases.RegisterInput{Email: body.Email, Password: body.Password, FullName: body.FullName})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toSession(s))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	s, err := h.auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSession(s))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.Logout(r.Context(), principal(r)); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toEmployee(principal(r).Employee))
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    *string `json:"email"`
		FullName *string `json:"full_name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	e, err := h.account.UpdateProfile(r.Context(), principal(r).Employee, usecases.UpdateProfileInput{Email: body.Email, FullName: body.FullName})
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toEmployee(e))
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	s, err := h.account.ChangePassword(r.Context(), principal(r).Employee, body.CurrentPassword, body.NewPassword)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSession(s))
}

func (h *Handler) deleteMe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.account.Delete(r.Context(), principal(r).Employee, body.Password); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createAutoservice(w http.ResponseWriter, r *http.Request) {
	var body autoserviceInputDTO
	if !decodeJSON(w, r, &body) {
		return
	}
	a, err := h.autoservices.Create(r.Context(), principal(r).Employee.ID, body.toDomain())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAutoservice(a))
}

func (h *Handler) getAutoservice(w http.ResponseWriter, r *http.Request) {
	a, err := h.autoservices.Get(r.Context(), principal(r).Employee.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAutoservice(a))
}

func (h *Handler) updateAutoservice(w http.ResponseWriter, r *http.Request) {
	var body autoserviceInputDTO
	if !decodeJSON(w, r, &body) {
		return
	}
	a, err := h.autoservices.Update(r.Context(), principal(r).Employee.ID, body.toDomain())
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAutoservice(a))
}

func (h *Handler) deleteAutoservice(w http.ResponseWriter, r *http.Request) {
	if err := h.autoservices.Delete(r.Context(), principal(r).Employee.ID); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getBot(w http.ResponseWriter, r *http.Request) {
	bot, err := h.autoservices.GetBot(r.Context(), principal(r).Employee.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toBot(bot))
}

func (h *Handler) connectBot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	bot, err := h.autoservices.ConnectBot(r.Context(), principal(r).Employee.ID, body.Token)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toBot(bot))
}

func (h *Handler) disconnectBot(w http.ResponseWriter, r *http.Request) {
	if err := h.autoservices.DisconnectBot(r.Context(), principal(r).Employee.ID); err != nil {
		writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRequests(w http.ResponseWriter, r *http.Request) {
	f, ok := parseRequestFilter(w, r)
	if !ok {
		return
	}
	items, total, err := h.dashboard.ListRequests(r.Context(), principal(r).Employee.ID, f)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listDTO[requestWithClientDTO]{Items: mapAll(items, toRequestWithClient), Total: total})
}

func parseRequestFilter(w http.ResponseWriter, r *http.Request) (domain.RequestFilter, bool) {
	q := r.URL.Query()
	var f domain.RequestFilter
	if s := q.Get("status"); s != "" {
		status, err := domain.ParseRequestStatus(s)
		if err != nil {
			writeDomainError(w, r, err)
			return f, false
		}
		f.Status = &status
	}
	switch q.Get("sort") {
	case "", "desc":
		f.Sort = domain.SortDesc
	case "asc":
		f.Sort = domain.SortAsc
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "sort must be asc or desc")
		return f, false
	}
	var ok bool
	if f.Limit, ok = intQuery(w, q, "limit"); !ok {
		return f, false
	}
	f.Offset, ok = intQuery(w, q, "offset")
	return f, ok
}

func (h *Handler) getRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	req, err := h.dashboard.GetRequest(r.Context(), principal(r).Employee.ID, id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRequestWithClient(req))
}

func (h *Handler) changeStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	status, err := domain.ParseRequestStatus(body.Status)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	h.setStatus(w, r, status)
}

func (h *Handler) statusAction(status domain.RequestStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { h.setStatus(w, r, status) }
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request, status domain.RequestStatus) {
	id, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	req, err := h.dashboard.ChangeStatus(r.Context(), principal(r).Employee.ID, id, status)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toRequestWithClient(req))
}

func (h *Handler) listClients(w http.ResponseWriter, r *http.Request) {
	items, err := h.dashboard.ListClients(r.Context(), principal(r).Employee.ID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listDTO[clientSummaryDTO]{Items: mapAll(items, toClientSummary), Total: len(items)})
}

func (h *Handler) getClient(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidParam(w, r, "id")
	if !ok {
		return
	}
	c, requests, err := h.dashboard.GetClient(r.Context(), principal(r).Employee.ID, id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, clientDetailsDTO{clientDTO: toClient(c), Requests: mapAll(requests, toRequest)})
}

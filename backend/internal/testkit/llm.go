package testkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
)

// LLM is a fake OpenAI-compatible chat completions server. Reply decides the answer for each request.
type LLM struct {
	Server *httptest.Server

	mu    sync.Mutex
	reply func(system, user string) (status int, content string)
	calls []LLMCall
}

type LLMCall struct {
	Model      string
	System     string
	User       string
	JSONMode   bool
	AuthHeader string
}

func NewLLM() *LLM {
	l := &LLM{reply: func(string, string) (int, string) { return http.StatusOK, `{"service": null}` }}
	l.Server = httptest.NewServer(http.HandlerFunc(l.serve))
	return l
}

func (l *LLM) Close() { l.Server.Close() }

// URL is the base URL to put into LLM_BASE_URL.
func (l *LLM) URL() string { return l.Server.URL + "/v1" }

func (l *LLM) SetReply(reply func(system, user string) (status int, content string)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reply = reply
}

func (l *LLM) Calls() []LLMCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]LLMCall(nil), l.calls...)
}

func (l *LLM) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/chat/completions" {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		ResponseFormat *struct {
			Type string `json:"type"`
		} `json:"response_format"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	call := LLMCall{Model: req.Model, JSONMode: req.ResponseFormat != nil, AuthHeader: r.Header.Get("Authorization")}
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			call.System = m.Content
		case "user":
			call.User = m.Content
		}
	}
	l.mu.Lock()
	l.calls = append(l.calls, call)
	reply := l.reply
	l.mu.Unlock()

	status, content := reply(call.System, call.User)
	if status != http.StatusOK {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": content}})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": content}}},
	})
}

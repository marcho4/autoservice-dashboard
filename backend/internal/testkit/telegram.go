package testkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TelegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

type SentMessage struct {
	MessageID   int64
	ChatID      int64
	Text        string
	ReplyMarkup struct {
		InlineKeyboard [][]struct {
			Text         string `json:"text"`
			CallbackData string `json:"callback_data"`
			URL          string `json:"url"`
		} `json:"inline_keyboard"`
		Keyboard [][]struct {
			Text           string `json:"text"`
			RequestContact bool   `json:"request_contact"`
		} `json:"keyboard"`
		RemoveKeyboard bool `json:"remove_keyboard"`
	}
}

func (m SentMessage) Button(text string) (string, bool) {
	for _, row := range m.ReplyMarkup.InlineKeyboard {
		for _, b := range row {
			if b.Text == text {
				return b.CallbackData, true
			}
		}
	}
	return "", false
}

func (m SentMessage) ButtonTexts() []string {
	var texts []string
	for _, row := range m.ReplyMarkup.InlineKeyboard {
		for _, b := range row {
			texts = append(texts, b.Text)
		}
	}
	for _, row := range m.ReplyMarkup.Keyboard {
		for _, b := range row {
			texts = append(texts, b.Text)
		}
	}
	return texts
}

type fakeBot struct {
	id               int64
	username         string
	updates          []map[string]any
	sent             []SentMessage
	read             map[int64]int
	toasts           []string
	polling          int
	description      string
	shortDescription string
}

type Telegram struct {
	Server *httptest.Server

	mu      sync.Mutex
	changed chan struct{}
	bots    map[string]*fakeBot
	nextID  int64
}

func NewTelegram() *Telegram {
	t := &Telegram{bots: map[string]*fakeBot{}, changed: make(chan struct{}), nextID: 100}
	t.Server = httptest.NewServer(http.HandlerFunc(t.serve))
	return t
}

func (t *Telegram) Close() {
	t.Server.CloseClientConnections()
	t.Server.Close()
}

func (t *Telegram) URL() string { return t.Server.URL }

func (t *Telegram) AddBot(token, username string) {
	id, _ := strconv.ParseInt(strings.SplitN(token, ":", 2)[0], 10, 64)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bots[token] = &fakeBot{id: id, username: username, read: map[int64]int{}}
}

func (t *Telegram) RemoveBot(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.bots, token)
	t.broadcast()
}

func (t *Telegram) broadcast() {
	close(t.changed)
	t.changed = make(chan struct{})
}

func (t *Telegram) push(token string, update map[string]any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	b, ok := t.bots[token]
	if !ok {
		panic("testkit: unknown bot token")
	}
	t.nextID++
	update["update_id"] = t.nextID
	b.updates = append(b.updates, update)
	t.broadcast()
}

func (t *Telegram) message(token string, from TelegramUser, extra map[string]any) {
	t.mu.Lock()
	t.nextID++
	msg := map[string]any{
		"message_id": t.nextID,
		"from":       from,
		"chat":       map[string]any{"id": from.ID, "type": "private"},
	}
	t.mu.Unlock()
	for k, v := range extra {
		msg[k] = v
	}
	t.push(token, map[string]any{"message": msg})
}

func (t *Telegram) SendText(token string, from TelegramUser, text string) {
	t.message(token, from, map[string]any{"text": text})
}

func (t *Telegram) SendContact(token string, from TelegramUser, phone string) {
	t.message(token, from, map[string]any{"contact": map[string]any{"phone_number": phone, "user_id": from.ID}})
}

func (t *Telegram) SendPhoto(token string, from TelegramUser) {
	t.message(token, from, map[string]any{"photo": []map[string]any{{"file_id": "x"}}})
}

func (t *Telegram) Click(token string, from TelegramUser, msg SentMessage, data string) {
	t.mu.Lock()
	t.nextID++
	id := t.nextID
	t.mu.Unlock()
	t.push(token, map[string]any{"callback_query": map[string]any{
		"id":   strconv.FormatInt(id, 10),
		"from": from,
		"data": data,
		"message": map[string]any{
			"message_id": msg.MessageID,
			"chat":       map[string]any{"id": msg.ChatID, "type": "private"},
			"text":       msg.Text,
		},
	}})
}

func (t *Telegram) NextMessage(token string, chatID int64, timeout time.Duration) (SentMessage, error) {
	deadline := time.After(timeout)
	for {
		t.mu.Lock()
		b := t.bots[token]
		if b == nil {
			t.mu.Unlock()
			return SentMessage{}, fmt.Errorf("unknown bot")
		}
		seen := 0
		for _, m := range b.sent {
			if m.ChatID != chatID {
				continue
			}
			if seen == b.read[chatID] {
				b.read[chatID]++
				t.mu.Unlock()
				return m, nil
			}
			seen++
		}
		changed := t.changed
		t.mu.Unlock()
		select {
		case <-changed:
		case <-deadline:
			return SentMessage{}, fmt.Errorf("no new message for chat %d within %s", chatID, timeout)
		}
	}
}

func (t *Telegram) Sent(token string, chatID int64) []SentMessage {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []SentMessage
	if b := t.bots[token]; b != nil {
		for _, m := range b.sent {
			if m.ChatID == chatID {
				out = append(out, m)
			}
		}
	}
	return out
}

func (t *Telegram) Message(token string, messageID int64) SentMessage {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, m := range t.bots[token].sent {
		if m.MessageID == messageID {
			return m
		}
	}
	return SentMessage{}
}

func (t *Telegram) Toasts(token string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.bots[token].toasts...)
}

func (t *Telegram) Pending(token string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.bots[token].updates)
}

func (t *Telegram) Description(token string) (description, short string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	b := t.bots[token]
	return b.description, b.shortDescription
}

func (t *Telegram) Polling(token string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if b := t.bots[token]; b != nil {
		return b.polling
	}
	return 0
}

func (t *Telegram) serve(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/bot")
	i := strings.LastIndex(path, "/")
	if i < 0 {
		http.NotFound(w, r)
		return
	}
	token, method := path[:i], path[i+1:]
	var params map[string]json.RawMessage
	_ = json.NewDecoder(r.Body).Decode(&params)

	t.mu.Lock()
	b, ok := t.bots[token]
	t.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
		return
	}

	var result any = true
	switch method {
	case "getMe":
		result = map[string]any{"id": b.id, "is_bot": true, "username": b.username}
	case "getUpdates":
		result = t.getUpdates(r.Context(), token, params)
		if result == nil {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
			return
		}
	case "sendMessage":
		var m SentMessage
		_ = json.Unmarshal(params["chat_id"], &m.ChatID)
		_ = json.Unmarshal(params["text"], &m.Text)
		_ = json.Unmarshal(params["reply_markup"], &m.ReplyMarkup)
		t.mu.Lock()
		t.nextID++
		m.MessageID = t.nextID
		b.sent = append(b.sent, m)
		t.broadcast()
		t.mu.Unlock()
		result = map[string]any{"message_id": m.MessageID}
	case "editMessageText", "editMessageReplyMarkup":
		var id int64
		_ = json.Unmarshal(params["message_id"], &id)
		t.mu.Lock()
		for i := range b.sent {
			if b.sent[i].MessageID != id {
				continue
			}
			if text, ok := params["text"]; ok {
				_ = json.Unmarshal(text, &b.sent[i].Text)
			}
			b.sent[i].ReplyMarkup.InlineKeyboard = nil
		}
		t.mu.Unlock()
	case "answerCallbackQuery":
		var text string
		_ = json.Unmarshal(params["text"], &text)
		t.mu.Lock()
		b.toasts = append(b.toasts, text)
		t.mu.Unlock()
	case "setMyDescription", "setMyShortDescription":
		t.mu.Lock()
		_ = json.Unmarshal(params["description"], &b.description)
		_ = json.Unmarshal(params["short_description"], &b.shortDescription)
		t.mu.Unlock()
	case "deleteWebhook", "setMyCommands", "sendChatAction":
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"ok":false,"error_code":404,"description":"Not Found"}`)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
}

func (t *Telegram) getUpdates(ctx context.Context, token string, params map[string]json.RawMessage) any {
	var offset int64
	var timeoutSec int
	_ = json.Unmarshal(params["offset"], &offset)
	_ = json.Unmarshal(params["timeout"], &timeoutSec)
	deadline := time.After(time.Duration(timeoutSec) * time.Second)

	t.mu.Lock()
	if b := t.bots[token]; b != nil {
		b.polling++
		defer func() {
			t.mu.Lock()
			b.polling--
			t.mu.Unlock()
		}()
	}
	t.mu.Unlock()

	for {
		t.mu.Lock()
		b := t.bots[token]
		if b == nil {
			t.mu.Unlock()
			return nil
		}
		pending := b.updates[:0]
		for _, u := range b.updates {
			if u["update_id"].(int64) >= offset {
				pending = append(pending, u)
			}
		}
		b.updates = pending
		out := append([]map[string]any{}, pending...)
		changed := t.changed
		t.mu.Unlock()
		if len(out) > 0 {
			return out
		}
		select {
		case <-changed:
		case <-deadline:
			return out
		case <-ctx.Done():
			return out
		}
	}
}

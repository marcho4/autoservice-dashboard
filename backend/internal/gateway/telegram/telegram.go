// Package telegram is a minimal Telegram Bot API client: just the methods the dashboard and the bot runner need.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
)

type Client struct {
	baseURL string
	http    *http.Client
	// poll has no client-side timeout: getUpdates is bounded by its own long-poll timeout and the request context.
	poll *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
		poll:    &http.Client{},
	}
}

// APIError is a non-ok answer of the Bot API.
type APIError struct {
	Code        int
	Description string
	RetryAfter  time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram: %d %s", e.Code, e.Description)
}

// ErrorCode returns the Bot API error code of err, or 0 for network and other errors.
func ErrorCode(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return 0
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func (c *Client) call(ctx context.Context, hc *http.Client, token, method string, payload, result any) error {
	body := []byte("{}")
	if payload != nil {
		var err error
		if body, err = json.Marshal(payload); err != nil {
			return fmt.Errorf("telegram %s: %w", method, err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, withoutToken(err))
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, withoutToken(err))
	}
	defer resp.Body.Close()

	var parsed apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		if resp.StatusCode != http.StatusOK {
			return &APIError{Code: resp.StatusCode, Description: http.StatusText(resp.StatusCode)}
		}
		return fmt.Errorf("telegram %s: decode response: %w", method, err)
	}
	if !parsed.OK {
		code := parsed.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return &APIError{Code: code, Description: parsed.Description, RetryAfter: time.Duration(parsed.Parameters.RetryAfter) * time.Second}
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(parsed.Result, result); err != nil {
		return fmt.Errorf("telegram %s: decode result: %w", method, err)
	}
	return nil
}

// GetMe checks a bot token. It is the only call made on behalf of an employee, so it keeps its own counters.
func (c *Client) GetMe(ctx context.Context, token string) (usecases.TelegramBotInfo, error) {
	var me User
	err := c.call(ctx, c.http, token, "getMe", nil, &me)
	switch code := ErrorCode(err); {
	case err == nil && me.IsBot:
		metrics.GetOrCreateCounter("tg_status_ok").Inc()
		metrics.GetOrCreateCounter("tg_success").Inc()
		return usecases.TelegramBotInfo{ID: me.ID, Username: me.Username}, nil
	case err == nil:
		metrics.GetOrCreateCounter("tg_wrong_response").Inc()
		return usecases.TelegramBotInfo{}, domain.ErrInvalidBotToken
	case code == http.StatusUnauthorized || code == http.StatusNotFound:
		metrics.GetOrCreateCounter("tg_status_bad_req").Inc()
		return usecases.TelegramBotInfo{}, domain.ErrInvalidBotToken
	case code != 0:
		metrics.GetOrCreateCounter("tg_unexpected_err").Inc()
		return usecases.TelegramBotInfo{}, err
	default:
		metrics.GetOrCreateCounter("tg_network_error").Inc()
		return usecases.TelegramBotInfo{}, err
	}
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

func (u User) FullName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Contact struct {
	PhoneNumber string `json:"phone_number"`
	UserID      int64  `json:"user_id"`
}

type Message struct {
	MessageID int64    `json:"message_id"`
	From      *User    `json:"from"`
	Chat      Chat     `json:"chat"`
	Text      string   `json:"text"`
	Contact   *Contact `json:"contact"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

// GetUpdates long-polls for updates after offset. Calling it confirms every update before offset.
func (c *Client) GetUpdates(ctx context.Context, token string, offset int64, timeout time.Duration) ([]Update, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout+15*time.Second)
	defer cancel()
	var updates []Update
	err := c.call(ctx, c.poll, token, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         int(timeout.Seconds()),
		"allowed_updates": []string{"message", "callback_query"},
	}, &updates)
	return updates, err
}

type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

type KeyboardButton struct {
	Text           string `json:"text"`
	RequestContact bool   `json:"request_contact,omitempty"`
}

// OutMessage is a text message in HTML parse mode with at most one kind of keyboard.
type OutMessage struct {
	ChatID         int64
	Text           string
	Inline         [][]InlineButton
	Keyboard       [][]KeyboardButton
	RemoveKeyboard bool
}

func (c *Client) SendMessage(ctx context.Context, token string, m OutMessage) error {
	payload := map[string]any{
		"chat_id":              m.ChatID,
		"text":                 m.Text,
		"parse_mode":           "HTML",
		"link_preview_options": map[string]any{"is_disabled": true},
	}
	switch {
	case len(m.Inline) > 0:
		payload["reply_markup"] = map[string]any{"inline_keyboard": m.Inline}
	case len(m.Keyboard) > 0:
		payload["reply_markup"] = map[string]any{"keyboard": m.Keyboard, "resize_keyboard": true, "one_time_keyboard": true}
	case m.RemoveKeyboard:
		payload["reply_markup"] = map[string]any{"remove_keyboard": true}
	}
	return c.call(ctx, c.http, token, "sendMessage", payload, nil)
}

func (c *Client) AnswerCallbackQuery(ctx context.Context, token, id, text string) error {
	return c.call(ctx, c.http, token, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text}, nil)
}

// EditMessageText replaces the text of a bot message and drops its inline keyboard.
func (c *Client) EditMessageText(ctx context.Context, token string, chatID, messageID int64, text string) error {
	return c.call(ctx, c.http, token, "editMessageText", map[string]any{
		"chat_id": chatID, "message_id": messageID, "text": text, "parse_mode": "HTML",
		"link_preview_options": map[string]any{"is_disabled": true},
	}, nil)
}

func (c *Client) ClearInlineKeyboard(ctx context.Context, token string, chatID, messageID int64) error {
	return c.call(ctx, c.http, token, "editMessageReplyMarkup", map[string]any{
		"chat_id": chatID, "message_id": messageID, "reply_markup": map[string]any{"inline_keyboard": [][]InlineButton{}},
	}, nil)
}

func (c *Client) SendTyping(ctx context.Context, token string, chatID int64) error {
	return c.call(ctx, c.http, token, "sendChatAction", map[string]any{"chat_id": chatID, "action": "typing"}, nil)
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// Setup prepares a bot for long polling: getUpdates does not work while a webhook is set.
func (c *Client) Setup(ctx context.Context, token string, commands []BotCommand) error {
	if err := c.call(ctx, c.http, token, "deleteWebhook", nil, nil); err != nil {
		return err
	}
	return c.call(ctx, c.http, token, "setMyCommands", map[string]any{"commands": commands}, nil)
}

// withoutToken strips the request URL from transport errors: it contains the bot token.
func withoutToken(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

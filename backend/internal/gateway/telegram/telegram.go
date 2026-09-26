package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/usecases"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

type getMeResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		ID       int64  `json:"id"`
		IsBot    bool   `json:"is_bot"`
		Username string `json:"username"`
	} `json:"result"`
}

func (c *Client) GetMe(ctx context.Context, token string) (usecases.TelegramBotInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/bot"+token+"/getMe", nil)
	if err != nil {
		return usecases.TelegramBotInfo{}, fmt.Errorf("telegram getMe: %w", withoutToken(err))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return usecases.TelegramBotInfo{}, fmt.Errorf("telegram getMe: %w", withoutToken(err))
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusNotFound:
		return usecases.TelegramBotInfo{}, domain.ErrInvalidBotToken
	default:
		return usecases.TelegramBotInfo{}, fmt.Errorf("telegram getMe: unexpected status %d", resp.StatusCode)
	}

	var body getMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return usecases.TelegramBotInfo{}, fmt.Errorf("telegram getMe: decode response: %w", err)
	}
	if !body.OK || !body.Result.IsBot {
		return usecases.TelegramBotInfo{}, domain.ErrInvalidBotToken
	}
	return usecases.TelegramBotInfo{ID: body.Result.ID, Username: body.Result.Username}, nil
}

func withoutToken(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

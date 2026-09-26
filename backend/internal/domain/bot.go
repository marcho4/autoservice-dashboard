package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Bot struct {
	AutoserviceID uuid.UUID
	TelegramBotID int64
	Username      string
	Token         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

var botTokenRe = regexp.MustCompile(`^\d{3,20}:[A-Za-z0-9_-]{30,64}$`)

func NormalizeBotToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if !botTokenRe.MatchString(token) {
		return "", invalid("token", "has invalid format, expected <bot_id>:<secret>")
	}
	return token, nil
}

func MaskBotToken(token string) string {
	id, secret, ok := strings.Cut(token, ":")
	if !ok || len(secret) < 8 {
		return "****"
	}
	return id + ":****" + secret[len(secret)-4:]
}

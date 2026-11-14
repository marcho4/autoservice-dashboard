package domain

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type BotStatus string

const (
	BotPending BotStatus = "pending"
	BotRunning BotStatus = "running"
	BotError   BotStatus = "error"
)

const (
	BotErrUnauthorized = "unauthorized"
	BotErrConflict     = "conflict"
	BotErrUnavailable  = "unavailable"
)

type Bot struct {
	AutoserviceID uuid.UUID
	TelegramBotID int64
	Username      string
	Token         string
	Status        BotStatus
	StatusError   string
	StatusAt      time.Time
	LinkCode      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const masterLinkPrefix = "master_"

func (b Bot) Link() string { return "https://t.me/" + b.Username }

func (b Bot) MasterLink() string { return b.Link() + "?start=" + masterLinkPrefix + b.LinkCode }

func MasterLinkCode(payload string) (string, bool) {
	return strings.CutPrefix(payload, masterLinkPrefix)
}

func NewLinkCode() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type NotifyRecipient struct {
	AutoserviceID uuid.UUID
	ChatID        int64
	Name          string
	CreatedAt     time.Time
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

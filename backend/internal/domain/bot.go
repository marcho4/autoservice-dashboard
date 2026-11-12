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
	// BotPending: the token is saved, the runner has not picked the bot up yet.
	BotPending BotStatus = "pending"
	BotRunning BotStatus = "running"
	BotError   BotStatus = "error"
)

// Reasons the runner reports with BotError.
const (
	BotErrUnauthorized = "unauthorized" // Telegram no longer accepts the token
	BotErrConflict     = "conflict"     // something else is polling the same bot
	BotErrUnavailable  = "unavailable"  // Telegram API is unreachable
)

type Bot struct {
	AutoserviceID uuid.UUID
	TelegramBotID int64
	Username      string
	Token         string
	Status        BotStatus
	StatusError   string
	StatusAt      time.Time
	// LinkCode is the one-time secret of the deep link that subscribes a master to new-request notifications.
	LinkCode  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const masterLinkPrefix = "master_"

func (b Bot) Link() string { return "https://t.me/" + b.Username }

func (b Bot) MasterLink() string { return b.Link() + "?start=" + masterLinkPrefix + b.LinkCode }

// MasterLinkCode extracts the code from a /start payload, if the payload is a master link.
func MasterLinkCode(payload string) (string, bool) {
	return strings.CutPrefix(payload, masterLinkPrefix)
}

func NewLinkCode() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NotifyRecipient is a Telegram chat that receives a message about every new request.
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

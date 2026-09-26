package domain

import (
	"time"

	"github.com/google/uuid"
)

type Client struct {
	ID               uuid.UUID
	AutoserviceID    uuid.UUID
	TelegramID       int64
	TelegramUsername string
	Name             string
	Phone            string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ClientSummary struct {
	Client
	RequestsCount int
	LastRequestAt *time.Time
}

type ClientIdentity struct {
	TelegramID       int64
	TelegramUsername string
	Name             string
}

func (c ClientIdentity) Validate() error {
	if c.TelegramID <= 0 {
		return invalid("telegram_id", "must be positive")
	}
	return nil
}

package domain

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	minPasswordLength = 8
	maxPasswordBytes  = 72
	maxNameLength     = 200
)

type Employee struct {
	ID           uuid.UUID
	Email        string
	FullName     string
	PasswordHash string
	TokenVersion int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", invalid("email", "must be a valid email address")
	}
	return email, nil
}

func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return invalid("password", fmt.Sprintf("must be at least %d characters", minPasswordLength))
	}
	if len(password) > maxPasswordBytes {
		return invalid("password", fmt.Sprintf("must be at most %d bytes", maxPasswordBytes))
	}
	return nil
}

func NormalizeFullName(name string) (string, error) {
	name = strings.TrimSpace(name)
	return name, validateRequiredText("full_name", name, maxNameLength)
}

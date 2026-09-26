package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Autoservice struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Name        string
	Address     string
	Phone       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AutoserviceInput struct {
	Name        string
	Address     string
	Phone       string
	Description string
}

func (in AutoserviceInput) Normalize() (AutoserviceInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Address = strings.TrimSpace(in.Address)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Description = strings.TrimSpace(in.Description)
	return in, firstError(
		validateRequiredText("name", in.Name, maxNameLength),
		validateOptionalText("address", in.Address, 500),
		validateOptionalPhone(in.Phone),
		validateOptionalText("description", in.Description, 2000),
	)
}

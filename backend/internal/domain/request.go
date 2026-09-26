package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type RequestStatus string

const (
	StatusNew        RequestStatus = "new"
	StatusInProgress RequestStatus = "in_progress"
	StatusClosed     RequestStatus = "closed"
	StatusRejected   RequestStatus = "rejected"
	StatusCancelled  RequestStatus = "cancelled"
)

func ParseRequestStatus(s string) (RequestStatus, error) {
	switch st := RequestStatus(s); st {
	case StatusNew, StatusInProgress, StatusClosed, StatusRejected, StatusCancelled:
		return st, nil
	}
	return "", invalid("status", "unknown status")
}

type Actor int

const (
	ActorEmployee Actor = iota
	ActorClient
)

var transitions = map[RequestStatus]map[RequestStatus]Actor{
	StatusNew: {
		StatusInProgress: ActorEmployee,
		StatusRejected:   ActorEmployee,
		StatusCancelled:  ActorClient,
	},
	StatusInProgress: {
		StatusClosed: ActorEmployee,
	},
}

func CanTransition(from, to RequestStatus, actor Actor) bool {
	allowedActor, ok := transitions[from][to]
	return ok && allowedActor == actor
}

type Request struct {
	ID            uuid.UUID
	AutoserviceID uuid.UUID
	ClientID      uuid.UUID
	Status        RequestStatus
	CarBrand      string
	CarModel      string
	Description   string
	Phone         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (r Request) IsEditable() bool {
	return r.Status == StatusNew
}

type RequestWithClient struct {
	Request
	Client Client
}

type RequestInput struct {
	CarBrand    string
	CarModel    string
	Description string
	Phone       string
}

func (in RequestInput) Normalize() (RequestInput, error) {
	in.CarBrand = strings.TrimSpace(in.CarBrand)
	in.CarModel = strings.TrimSpace(in.CarModel)
	in.Description = strings.TrimSpace(in.Description)
	in.Phone = strings.TrimSpace(in.Phone)
	return in, firstError(
		validateRequiredText("car_brand", in.CarBrand, 100),
		validateRequiredText("car_model", in.CarModel, 100),
		validateRequiredText("description", in.Description, 4000),
		validatePhone(in.Phone),
	)
}

type SortOrder string

const (
	SortDesc SortOrder = "desc"
	SortAsc  SortOrder = "asc"
)

type RequestFilter struct {
	Status *RequestStatus
	Sort   SortOrder
	Limit  int
	Offset int
}

package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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

const MaxRequestsPerClientPerDay = 5

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

type Answer struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Request struct {
	ID            uuid.UUID
	AutoserviceID uuid.UUID
	ClientID      uuid.UUID
	Number        int
	Status        RequestStatus
	ServiceID     *uuid.UUID
	ServiceName   string
	Answers       []Answer
	Message       string
	Phone         string
	Note          string
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
	DraftID     uuid.UUID
	ServiceID   *uuid.UUID
	ServiceName string
	Answers     []Answer
	Message     string
	Phone       string
}

const (
	maxAnswers       = 30
	maxAnswerLabel   = 100
	maxAnswerValue   = 1000
	maxClientMessage = 4000
	maxNoteLength    = 4000
)

func (in RequestInput) Normalize() (RequestInput, error) {
	in.ServiceName = strings.TrimSpace(in.ServiceName)
	in.Message = strings.TrimSpace(in.Message)
	if err := firstError(
		validateRequiredText("service_name", in.ServiceName, maxNameLength),
		validateOptionalText("message", in.Message, maxClientMessage),
	); err != nil {
		return in, err
	}
	if len(in.Answers) > maxAnswers {
		return in, invalid("answers", fmt.Sprintf("must have at most %d items", maxAnswers))
	}
	answers := make([]Answer, 0, len(in.Answers))
	for i, a := range in.Answers {
		a.Label, a.Value = strings.TrimSpace(a.Label), strings.TrimSpace(a.Value)
		if a.Value == "" {
			continue
		}
		if a.Label == "" || utf8.RuneCountInString(a.Label) > maxAnswerLabel || utf8.RuneCountInString(a.Value) > maxAnswerValue {
			return in, invalid(fmt.Sprintf("answers[%d]", i), "label must be 1-100 characters and value at most 1000")
		}
		answers = append(answers, a)
	}
	in.Answers = answers
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		return in, err
	}
	in.Phone = phone
	return in, nil
}

func NormalizeNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	return note, validateOptionalText("note", note, maxNoteLength)
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

type RequestCounts map[RequestStatus]int

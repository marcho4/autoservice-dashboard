package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already registered")
	ErrAutoserviceExists  = errors.New("employee already has an autoservice")
	ErrNoAutoservice      = errors.New("employee has no autoservice")
	ErrBotAlreadyLinked   = errors.New("bot is already connected to an autoservice")
	ErrBotNotConnected    = errors.New("no bot connected")
	ErrInvalidBotToken    = errors.New("telegram rejected the bot token")
	ErrInvalidTransition  = errors.New("status transition is not allowed")
	ErrRequestNotEditable = errors.New("request can be changed only in status new")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

func (e *ValidationError) Unwrap() error { return ErrInvalidInput }

func invalid(field, msg string) error { return &ValidationError{Field: field, Message: msg} }

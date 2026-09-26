package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type AuthService struct {
	employees EmployeeRepository
	revoked   RevokedTokenRepository
	hasher    PasswordHasher
	tokens    TokenManager
}

func NewAuthService(e EmployeeRepository, r RevokedTokenRepository, h PasswordHasher, t TokenManager) *AuthService {
	return &AuthService{employees: e, revoked: r, hasher: h, tokens: t}
}

type Session struct {
	Token     string
	ExpiresAt time.Time
	Employee  domain.Employee
}

type Principal struct {
	Employee       domain.Employee
	TokenID        string
	TokenExpiresAt time.Time
}

type RegisterInput struct {
	Email    string
	Password string
	FullName string
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (Session, error) {
	email, err := domain.NormalizeEmail(in.Email)
	if err != nil {
		return Session{}, err
	}
	name, err := domain.NormalizeFullName(in.FullName)
	if err != nil {
		return Session{}, err
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return Session{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return Session{}, fmt.Errorf("hash password: %w", err)
	}
	emp, err := s.employees.Create(ctx, domain.Employee{Email: email, FullName: name, PasswordHash: hash})
	if err != nil {
		return Session{}, err
	}
	return s.newSession(emp)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (Session, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return Session{}, domain.ErrInvalidCredentials
	}
	emp, err := s.employees.GetByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return Session{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if !s.hasher.Compare(emp.PasswordHash, password) {
		return Session{}, domain.ErrInvalidCredentials
	}
	return s.newSession(emp)
}

func (s *AuthService) Logout(ctx context.Context, p Principal) error {
	return s.revoked.Revoke(ctx, p.TokenID, p.TokenExpiresAt)
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (Principal, error) {
	claims, err := s.tokens.Parse(token)
	if err != nil {
		return Principal{}, domain.ErrUnauthorized
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Principal{}, domain.ErrUnauthorized
	}
	revoked, err := s.revoked.IsRevoked(ctx, claims.ID)
	if err != nil {
		return Principal{}, err
	}
	if revoked {
		return Principal{}, domain.ErrUnauthorized
	}
	emp, err := s.employees.GetByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return Principal{}, domain.ErrUnauthorized
	}
	if err != nil {
		return Principal{}, err
	}
	if emp.TokenVersion != claims.Version {
		return Principal{}, domain.ErrUnauthorized
	}
	return Principal{Employee: emp, TokenID: claims.ID, TokenExpiresAt: claims.ExpiresAt.Time}, nil
}

func (s *AuthService) newSession(emp domain.Employee) (Session, error) {
	token, claims, err := s.tokens.Issue(emp.ID.String(), emp.TokenVersion)
	if err != nil {
		return Session{}, fmt.Errorf("issue token: %w", err)
	}
	return Session{Token: token, ExpiresAt: claims.ExpiresAt.Time, Employee: emp}, nil
}

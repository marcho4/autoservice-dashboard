package usecases

import (
	"context"
	"fmt"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type AccountService struct {
	employees EmployeeRepository
	hasher    PasswordHasher
	auth      *AuthService
}

func NewAccountService(e EmployeeRepository, h PasswordHasher, auth *AuthService) *AccountService {
	return &AccountService{employees: e, hasher: h, auth: auth}
}

type UpdateProfileInput struct {
	Email    *string
	FullName *string
}

func (s *AccountService) UpdateProfile(ctx context.Context, current domain.Employee, in UpdateProfileInput) (domain.Employee, error) {
	email, name := current.Email, current.FullName
	var err error
	if in.Email != nil {
		if email, err = domain.NormalizeEmail(*in.Email); err != nil {
			return domain.Employee{}, err
		}
	}
	if in.FullName != nil {
		if name, err = domain.NormalizeFullName(*in.FullName); err != nil {
			return domain.Employee{}, err
		}
	}
	return s.employees.UpdateProfile(ctx, current.ID, email, name)
}

func (s *AccountService) ChangePassword(ctx context.Context, current domain.Employee, oldPassword, newPassword string) (Session, error) {
	if !s.hasher.Compare(current.PasswordHash, oldPassword) {
		return Session{}, domain.ErrInvalidCredentials
	}
	if err := domain.ValidatePassword(newPassword); err != nil {
		return Session{}, err
	}
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return Session{}, fmt.Errorf("hash password: %w", err)
	}
	emp, err := s.employees.UpdatePassword(ctx, current.ID, hash)
	if err != nil {
		return Session{}, err
	}
	return s.auth.newSession(emp)
}

func (s *AccountService) Delete(ctx context.Context, current domain.Employee, password string) error {
	if !s.hasher.Compare(current.PasswordHash, password) {
		return domain.ErrInvalidCredentials
	}
	return s.employees.Delete(ctx, current.ID)
}

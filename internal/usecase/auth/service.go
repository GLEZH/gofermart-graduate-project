package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type Credentials struct {
	Login    string
	Password string
}

type Service struct {
	users  userStore
	hasher passwordHasher
	tokens tokenIssuer
}

func NewService(users userStore, hasher passwordHasher, tokens tokenIssuer) *Service {
	return &Service{users: users, hasher: hasher, tokens: tokens}
}

func (s *Service) Register(ctx context.Context, credentials Credentials) (string, error) {
	credentials.Login = strings.TrimSpace(credentials.Login)
	if credentials.Login == "" || credentials.Password == "" {
		return "", user.ErrInvalidInput
	}

	hash, err := s.hasher.Hash(credentials.Password)
	if err != nil {
		return "", err
	}
	registered, err := s.users.CreateWithAccount(ctx, credentials.Login, hash)
	if err != nil {
		return "", err
	}
	return s.tokens.Issue(registered.ID)
}

func (s *Service) Login(ctx context.Context, credentials Credentials) (string, error) {
	credentials.Login = strings.TrimSpace(credentials.Login)
	if credentials.Login == "" || credentials.Password == "" {
		return "", user.ErrInvalidInput
	}

	registered, err := s.users.FindByLogin(ctx, credentials.Login)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return "", user.ErrInvalidCredentials
		}
		return "", err
	}
	if err = s.hasher.Compare(registered.PasswordHash, credentials.Password); err != nil {
		return "", user.ErrInvalidCredentials
	}
	return s.tokens.Issue(registered.ID)
}

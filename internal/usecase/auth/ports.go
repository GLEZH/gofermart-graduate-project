package auth

import (
	"context"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type userStore interface {
	CreateWithAccount(ctx context.Context, login, passwordHash string) (user.User, error)
	FindByLogin(ctx context.Context, login string) (user.User, error)
}

type passwordHasher interface {
	Hash(password string) (string, error)
	Compare(passwordHash, password string) error
}

type tokenIssuer interface {
	Issue(userID user.ID) (string, error)
}

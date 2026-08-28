package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type fakeUsers struct {
	created user.User
	found   user.User
	err     error
}

func (f *fakeUsers) CreateWithAccount(context.Context, string, string) (user.User, error) {
	return f.created, f.err
}

func (f *fakeUsers) FindByLogin(context.Context, string) (user.User, error) {
	return f.found, f.err
}

type fakeHasher struct{ err error }

func (f fakeHasher) Hash(password string) (string, error) { return "hash:" + password, f.err }
func (f fakeHasher) Compare(hash, password string) error {
	if f.err != nil {
		return f.err
	}
	if hash != "hash:"+password {
		return errors.New("mismatch")
	}
	return nil
}

type fakeTokens struct{ err error }

func (f fakeTokens) Issue(user.ID) (string, error) { return "token", f.err }

func TestServiceRegister(t *testing.T) {
	users := &fakeUsers{created: user.User{ID: 1}}
	service := NewService(users, fakeHasher{}, fakeTokens{})
	token, err := service.Register(context.Background(), Credentials{Login: " login ", Password: "password"})
	if err != nil || token != "token" {
		t.Fatalf("Register() = %q, %v", token, err)
	}
	for _, credentials := range []Credentials{{}, {Login: "login"}, {Password: "password"}} {
		if _, err = service.Register(context.Background(), credentials); !errors.Is(err, user.ErrInvalidInput) {
			t.Fatalf("Register() error = %v", err)
		}
	}
	users.err = user.ErrLoginTaken
	if _, err = service.Register(context.Background(), Credentials{Login: "login", Password: "password"}); !errors.Is(err, user.ErrLoginTaken) {
		t.Fatalf("Register() error = %v", err)
	}
	service = NewService(users, fakeHasher{err: errors.New("hash")}, fakeTokens{})
	if _, err = service.Register(context.Background(), Credentials{Login: "login", Password: "password"}); err == nil {
		t.Fatal("Register() error = nil")
	}
}

func TestServiceLogin(t *testing.T) {
	users := &fakeUsers{found: user.User{ID: 1, PasswordHash: "hash:password"}}
	service := NewService(users, fakeHasher{}, fakeTokens{})
	token, err := service.Login(context.Background(), Credentials{Login: "login", Password: "password"})
	if err != nil || token != "token" {
		t.Fatalf("Login() = %q, %v", token, err)
	}
	users.err = user.ErrNotFound
	if _, err = service.Login(context.Background(), Credentials{Login: "login", Password: "password"}); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v", err)
	}
	users.err = errors.New("database")
	if _, err = service.Login(context.Background(), Credentials{Login: "login", Password: "password"}); err == nil {
		t.Fatal("Login() error = nil")
	}
	users.err = nil
	if _, err = service.Login(context.Background(), Credentials{Login: "login", Password: "wrong"}); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v", err)
	}
	if _, err = service.Login(context.Background(), Credentials{}); !errors.Is(err, user.ErrInvalidInput) {
		t.Fatalf("Login() error = %v", err)
	}
}

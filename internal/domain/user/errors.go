package user

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrLoginTaken         = errors.New("login already taken")
	ErrInvalidInput       = errors.New("invalid user input")
	ErrNotFound           = errors.New("user not found")
)

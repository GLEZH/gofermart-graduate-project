package user

import "time"

type ID int64

type User struct {
	ID           ID
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

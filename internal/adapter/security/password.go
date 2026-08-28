package security

import "golang.org/x/crypto/bcrypt"

type PasswordHasher struct {
	cost int
}

func NewPasswordHasher(cost int) *PasswordHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &PasswordHasher{cost: cost}
}

func (h *PasswordHasher) Hash(password string) (string, error) {
	value, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	return string(value), err
}

func (h *PasswordHasher) Compare(passwordHash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
}

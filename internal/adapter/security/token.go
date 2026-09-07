package security

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
	"github.com/golang-jwt/jwt/v4"
)

const tokenTTL = 24 * time.Hour

type TokenManager struct {
	secret []byte
	now    func() time.Time
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret), now: time.Now}
}

func (m *TokenManager) Issue(userID user.ID) (string, error) {
	now := m.now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(int64(userID), 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *TokenManager) Verify(value string) (user.ID, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.secret, nil
	})
	if err != nil {
		return 0, err
	}
	if !token.Valid || claims.Subject == "" {
		return 0, errors.New("invalid token")
	}
	valueID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || valueID <= 0 {
		return 0, errors.New("invalid token subject")
	}
	return user.ID(valueID), nil
}

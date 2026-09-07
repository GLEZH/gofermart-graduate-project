package security

import (
	"testing"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHasher(t *testing.T) {
	hasher := NewPasswordHasher(bcrypt.MinCost)
	hash, err := hasher.Hash("password")
	if err != nil {
		t.Fatal(err)
	}
	if err = hasher.Compare(hash, "password"); err != nil {
		t.Fatal(err)
	}
	if err = hasher.Compare(hash, "wrong"); err == nil {
		t.Fatal("Compare() error = nil")
	}
	if NewPasswordHasher(0).cost != bcrypt.DefaultCost {
		t.Fatal("default cost not applied")
	}
}

func TestTokenManager(t *testing.T) {
	manager := NewTokenManager("secret")
	token, err := manager.Issue(user.ID(42))
	if err != nil {
		t.Fatal(err)
	}
	userID, err := manager.Verify(token)
	if err != nil || userID != 42 {
		t.Fatalf("Verify() = %d, %v", userID, err)
	}
	if _, err = NewTokenManager("other").Verify(token); err == nil {
		t.Fatal("wrong secret error = nil")
	}

	expired := NewTokenManager("secret")
	expired.now = func() time.Time { return time.Now().Add(-48 * time.Hour) }
	expiredToken, err := expired.Issue(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Verify(expiredToken); err == nil {
		t.Fatal("expired token error = nil")
	}

	invalidSubject := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "bad", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))})
	invalidToken, _ := invalidSubject.SignedString([]byte("secret"))
	if _, err = manager.Verify(invalidToken); err == nil {
		t.Fatal("invalid subject error = nil")
	}

	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Subject: "1", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))})
	unsignedToken, _ := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err = manager.Verify(unsignedToken); err == nil {
		t.Fatal("unexpected method error = nil")
	}
}

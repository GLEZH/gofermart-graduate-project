package app

import (
	"context"
	"testing"

	"github.com/GLEZH/gofermart-graduate-project/internal/config"
)

func TestNewValidation(t *testing.T) {
	if _, err := New(context.Background(), nil); err == nil {
		t.Fatal("nil config error = nil")
	}
	if _, err := New(context.Background(), &config.Config{RunAddress: "localhost:8080"}); err == nil {
		t.Fatal("empty accrual address error = nil")
	}
	if _, err := New(context.Background(), &config.Config{RunAddress: "localhost:8080", AccrualSystemAddress: "http://localhost"}); err == nil {
		t.Fatal("empty authentication secret error = nil")
	}
	if _, err := New(context.Background(), &config.Config{RunAddress: "localhost:8080", AccrualSystemAddress: "http://localhost", AuthSecret: "secret"}); err == nil {
		t.Fatal("empty database URI error = nil")
	}
}

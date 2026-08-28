package accrual

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	accrualusecase "github.com/GLEZH/gofermart-graduate-project/internal/usecase/accrual"
)

func TestClientCalculate(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		retryAfter string
		wantFound  bool
		wantStatus string
		wantAmount string
		wantRate   bool
		wantErr    bool
	}{
		{name: "processed", status: http.StatusOK, body: `{"order":"9278923470","status":"PROCESSED","accrual":500.5}`, wantFound: true, wantStatus: "PROCESSED", wantAmount: "500.5"},
		{name: "processing", status: http.StatusOK, body: `{"status":"PROCESSING"}`, wantFound: true, wantStatus: "PROCESSING", wantAmount: "nil"},
		{name: "not found", status: http.StatusNoContent},
		{name: "rate limited", status: http.StatusTooManyRequests, retryAfter: "60", wantRate: true, wantErr: true},
		{name: "invalid json", status: http.StatusOK, body: `{`, wantErr: true},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/orders/9278923470" {
					t.Errorf("path = %s", r.URL.Path)
				}
				w.Header().Set("Retry-After", test.retryAfter)
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			result, err := NewClient(server.URL+"/", nil).Calculate(context.Background(), order.Number("9278923470"))
			if (err != nil) != test.wantErr || result.Found != test.wantFound || result.Status != test.wantStatus {
				t.Fatalf("Calculate() = %+v, %v", result, err)
			}
			amount := "nil"
			if result.Accrual != nil {
				amount = result.Accrual.String()
			}
			if amount != test.wantAmount && !(test.wantAmount == "" && amount == "nil") {
				t.Fatalf("amount = %s, want %s", amount, test.wantAmount)
			}
			var rate *accrualusecase.RateLimitError
			if errors.As(err, &rate) != test.wantRate {
				t.Fatalf("rate limit = %v", err)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	if parseRetryAfter("2") != 2*time.Second {
		t.Fatal("seconds not parsed")
	}
	if parseRetryAfter("bad") != time.Second {
		t.Fatal("fallback not used")
	}
	future := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	delay := parseRetryAfter(future)
	if delay < 50*time.Second || delay > time.Minute {
		t.Fatalf("date delay = %s", delay)
	}
}

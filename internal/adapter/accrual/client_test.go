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
			client := NewClient(server.URL+"/", nil, nil)
			client.retryDelay = time.Millisecond
			result, err := client.Calculate(context.Background(), order.Number("9278923470"))
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

func TestCalculateRetriesServerErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"status":"PROCESSED","accrual":10}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, nil, nil)
	client.retryDelay = time.Millisecond
	result, err := client.Calculate(context.Background(), order.Number("9278923470"))
	if err != nil || !result.Found || result.Status != "PROCESSED" {
		t.Fatalf("Calculate() = %+v, %v", result, err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestCalculateGivesUpAfterAttempts(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil, nil)
	client.retryDelay = time.Millisecond
	if _, err := client.Calculate(context.Background(), order.Number("9278923470")); err == nil {
		t.Fatal("Calculate() error = nil")
	}
	if requests != defaultAttempts {
		t.Fatalf("requests = %d, want %d", requests, defaultAttempts)
	}
}

func TestCalculateStopsOnCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client := NewClient(server.URL, nil, nil)
	client.retryDelay = time.Minute
	cancel()
	if _, err := client.Calculate(ctx, order.Number("9278923470")); err == nil {
		t.Fatal("Calculate() error = nil")
	}
}

func TestParseRetryAfter(t *testing.T) {
	delay, err := parseRetryAfter("2")
	if delay != 2*time.Second || err != nil {
		t.Fatalf("seconds not parsed: %s, %v", delay, err)
	}
	delay, err = parseRetryAfter("bad")
	if delay != defaultRetryAfter || err == nil {
		t.Fatalf("fallback not reported: %s, %v", delay, err)
	}
	future := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	delay, err = parseRetryAfter(future)
	if err != nil || delay < 50*time.Second || delay > time.Minute {
		t.Fatalf("date delay = %s, %v", delay, err)
	}
}

package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	accrualusecase "github.com/GLEZH/gofermart-graduate-project/internal/usecase/accrual"
	"go.uber.org/zap"
)

const (
	requestTimeout    = 5 * time.Second
	defaultAttempts   = 3
	defaultRetryDelay = time.Second
	defaultRetryAfter = time.Second
)

type serverError struct {
	status int
}

func (e *serverError) Error() string {
	return fmt.Sprintf("accrual status: %d", e.status)
}

type Client struct {
	baseURL    string
	http       *http.Client
	attempts   int
	retryDelay time.Duration
	log        *zap.SugaredLogger
}

func NewClient(baseURL string, client *http.Client, log *zap.SugaredLogger) *Client {
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		http:       client,
		attempts:   defaultAttempts,
		retryDelay: defaultRetryDelay,
		log:        log,
	}
}

func (c *Client) Calculate(ctx context.Context, number order.Number) (accrualusecase.Calculation, error) {
	var lastErr error
	attempts := 0
	for attempts < c.attempts {
		if attempts > 0 && !wait(ctx, c.retryDelay<<(attempts-1)) {
			break
		}
		attempts++
		calculation, err := c.request(ctx, number)
		if err == nil {
			return calculation, nil
		}
		var unavailable *serverError
		if !errors.As(err, &unavailable) {
			return accrualusecase.Calculation{}, err
		}
		lastErr = err
		c.logWarn("retry accrual request", err, "order", number.String(), "attempt", attempts)
	}
	return accrualusecase.Calculation{}, fmt.Errorf("accrual unavailable after %d attempts: %w", attempts, lastErr)
}

func (c *Client) request(ctx context.Context, number order.Number) (accrualusecase.Calculation, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/orders/"+number.String(), nil)
	if err != nil {
		return accrualusecase.Calculation{}, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return accrualusecase.Calculation{}, fmt.Errorf("request accrual: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusNoContent:
		return accrualusecase.Calculation{Found: false}, nil
	case http.StatusTooManyRequests:
		retryAfter, err := parseRetryAfter(response.Header.Get("Retry-After"))
		if err != nil {
			c.logWarn("accrual retry-after header", err)
		}
		return accrualusecase.Calculation{}, &accrualusecase.RateLimitError{RetryAfter: retryAfter}
	case http.StatusOK:
		var payload struct {
			Status  string          `json:"status"`
			Accrual *loyalty.Amount `json:"accrual,omitempty"`
		}
		if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
			return accrualusecase.Calculation{}, fmt.Errorf("decode accrual: %w", err)
		}
		return accrualusecase.Calculation{Found: true, Status: payload.Status, Accrual: payload.Accrual}, nil
	default:
		if response.StatusCode >= http.StatusInternalServerError {
			return accrualusecase.Calculation{}, &serverError{status: response.StatusCode}
		}
		return accrualusecase.Calculation{}, fmt.Errorf("accrual status: %d", response.StatusCode)
	}
}

func (c *Client) logWarn(message string, err error, fields ...any) {
	if c.log != nil {
		c.log.Warnw(message, append([]any{"error", err}, fields...)...)
	}
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func parseRetryAfter(value string) (time.Duration, error) {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second, nil
	}
	when, err := http.ParseTime(value)
	if err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay, nil
		}
	}
	return defaultRetryAfter, fmt.Errorf("unsupported Retry-After value %q", value)
}

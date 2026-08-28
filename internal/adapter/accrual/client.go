package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	accrualusecase "github.com/GLEZH/gofermart-graduate-project/internal/usecase/accrual"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

func (c *Client) Calculate(ctx context.Context, number order.Number) (accrualusecase.Calculation, error) {
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
		return accrualusecase.Calculation{}, &accrualusecase.RateLimitError{RetryAfter: parseRetryAfter(response.Header.Get("Retry-After"))}
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
		return accrualusecase.Calculation{}, fmt.Errorf("accrual status: %d", response.StatusCode)
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err == nil {
		delay := time.Until(when)
		if delay > 0 {
			return delay
		}
	}
	return time.Second
}

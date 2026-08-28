package accrual

import (
	"context"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
)

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return "accrual rate limit exceeded"
}

type Calculation struct {
	Found   bool
	Status  string
	Accrual *loyalty.Amount
}

type calculator interface {
	Calculate(ctx context.Context, number order.Number) (Calculation, error)
}

type orderStore interface {
	Pending(ctx context.Context, limit int) ([]order.Order, error)
	UpdateStatus(ctx context.Context, number order.Number, status order.Status) error
	Settle(ctx context.Context, number order.Number, amount *loyalty.Amount) error
}

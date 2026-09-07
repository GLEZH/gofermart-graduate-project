package accrual

import (
	"context"
	"errors"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
)

const (
	defaultRetryInterval = time.Second
	maxRetryInterval     = time.Minute
	maxRetryShift        = 6
)

var errUnknownStatus = errors.New("unknown accrual status")

type Processor struct {
	calculator calculator
	orders     orderStore
	retry      time.Duration
}

func NewProcessor(calculator calculator, orders orderStore, retry time.Duration) *Processor {
	if retry <= 0 {
		retry = defaultRetryInterval
	}
	return &Processor{calculator: calculator, orders: orders, retry: retry}
}

func (p *Processor) Pending(ctx context.Context, limit int) ([]order.Order, error) {
	return p.orders.Pending(ctx, limit)
}

func (p *Processor) Process(ctx context.Context, target order.Order) error {
	calculation, err := p.calculator.Calculate(ctx, target.Number)
	if err != nil {
		return err
	}
	if !calculation.Found {
		return p.orders.Defer(ctx, target.Number, p.retryAfter(target.Attempts))
	}

	switch calculation.Status {
	case "REGISTERED":
		return p.orders.UpdateStatus(ctx, target.Number, order.StatusNew)
	case "PROCESSING":
		return p.orders.UpdateStatus(ctx, target.Number, order.StatusProcessing)
	case "INVALID":
		return p.orders.UpdateStatus(ctx, target.Number, order.StatusInvalid)
	case "PROCESSED":
		return p.orders.Settle(ctx, target.Number, calculation.Accrual)
	default:
		return errUnknownStatus
	}
}

func (p *Processor) retryAfter(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > maxRetryShift {
		attempts = maxRetryShift
	}
	delay := p.retry << attempts
	if delay <= 0 || delay > maxRetryInterval {
		return maxRetryInterval
	}
	return delay
}

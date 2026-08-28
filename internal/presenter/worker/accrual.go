package worker

import (
	"context"
	"errors"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	accrualusecase "github.com/GLEZH/gofermart-graduate-project/internal/usecase/accrual"
	"go.uber.org/zap"
)

type processor interface {
	Pending(ctx context.Context, limit int) ([]order.Order, error)
	Process(ctx context.Context, target order.Order) error
}

type Accrual struct {
	processor processor
	interval  time.Duration
	log       *zap.SugaredLogger
}

func NewAccrual(processor processor, interval time.Duration, log *zap.SugaredLogger) *Accrual {
	return &Accrual{processor: processor, interval: interval, log: log}
}

func (w *Accrual) Run(ctx context.Context) error {
	delay := time.Duration(0)
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		delay = w.processBatch(ctx)
		if delay <= 0 {
			delay = w.interval
		}
	}
}

func (w *Accrual) processBatch(ctx context.Context) time.Duration {
	items, err := w.processor.Pending(ctx, 20)
	if err != nil {
		w.logError("list pending orders", err)
		return w.interval
	}
	for _, item := range items {
		if err = w.processor.Process(ctx, item); err == nil {
			continue
		}
		var rateLimit *accrualusecase.RateLimitError
		if errors.As(err, &rateLimit) {
			return rateLimit.RetryAfter
		}
		w.logError("process order", err)
	}
	return w.interval
}

func (w *Accrual) logError(message string, err error) {
	if w.log != nil {
		w.log.Errorw(message, "error", err)
	}
}

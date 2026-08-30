package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	accrualusecase "github.com/GLEZH/gofermart-graduate-project/internal/usecase/accrual"
	"go.uber.org/zap"
)

const (
	batchSize      = 20
	defaultWorkers = 5
)

type processor interface {
	Pending(ctx context.Context, limit int) ([]order.Order, error)
	Process(ctx context.Context, target order.Order) error
}

type Accrual struct {
	processor processor
	interval  time.Duration
	workers   int
	log       *zap.SugaredLogger
}

func NewAccrual(processor processor, interval time.Duration, log *zap.SugaredLogger) *Accrual {
	return &Accrual{processor: processor, interval: interval, workers: defaultWorkers, log: log}
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
	items, err := w.processor.Pending(ctx, batchSize)
	if err != nil {
		w.logError("list pending orders", err)
		return w.interval
	}
	if len(items) == 0 {
		return w.interval
	}

	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan order.Order, w.workers)

	var mu sync.Mutex
	retryAfter := time.Duration(0)
	var group sync.WaitGroup
	for i := 0; i < w.workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for item := range jobs {
				if batchCtx.Err() != nil {
					continue
				}
				delay, processErr := w.process(batchCtx, item)
				if processErr != nil {
					w.logError("process order", processErr)
					continue
				}
				if delay <= 0 {
					continue
				}
				mu.Lock()
				if delay > retryAfter {
					retryAfter = delay
				}
				mu.Unlock()
				cancel()
			}
		}()
	}

	for _, item := range items {
		select {
		case jobs <- item:
		case <-batchCtx.Done():
		}
	}
	close(jobs)
	group.Wait()

	if retryAfter > 0 {
		return retryAfter
	}
	return w.interval
}

func (w *Accrual) process(ctx context.Context, item order.Order) (time.Duration, error) {
	err := w.processor.Process(ctx, item)
	if err == nil {
		return 0, nil
	}
	var rateLimit *accrualusecase.RateLimitError
	if errors.As(err, &rateLimit) {
		return rateLimit.RetryAfter, nil
	}
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return 0, nil
	}
	return 0, err
}

func (w *Accrual) logError(message string, err error) {
	if w.log != nil {
		w.log.Errorw(message, "error", err)
	}
}

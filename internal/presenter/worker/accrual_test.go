package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	accrualusecase "github.com/GLEZH/gofermart-graduate-project/internal/usecase/accrual"
)

type fakeProcessor struct {
	items      []order.Order
	pendingErr error
	processErr error
	processed  int
}

func (f *fakeProcessor) Pending(context.Context, int) ([]order.Order, error) {
	return f.items, f.pendingErr
}

func (f *fakeProcessor) Process(context.Context, order.Order) error {
	f.processed++
	return f.processErr
}

func TestProcessBatch(t *testing.T) {
	processor := &fakeProcessor{items: []order.Order{{Number: "9278923470"}}}
	worker := NewAccrual(processor, time.Second, nil)
	if delay := worker.processBatch(context.Background()); delay != time.Second || processor.processed != 1 {
		t.Fatalf("processBatch() delay=%s processed=%d", delay, processor.processed)
	}
	processor.pendingErr = errors.New("database")
	if delay := worker.processBatch(context.Background()); delay != time.Second {
		t.Fatalf("pending error delay=%s", delay)
	}
	processor.pendingErr = nil
	processor.processErr = &accrualusecase.RateLimitError{RetryAfter: time.Minute}
	if delay := worker.processBatch(context.Background()); delay != time.Minute {
		t.Fatalf("rate-limit delay=%s", delay)
	}
	processor.processErr = errors.New("client")
	if delay := worker.processBatch(context.Background()); delay != time.Second {
		t.Fatalf("client error delay=%s", delay)
	}
}

func TestRunStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewAccrual(&fakeProcessor{}, time.Millisecond, nil).Run(ctx); err != nil {
		t.Fatal(err)
	}
}

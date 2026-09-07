package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	accrualusecase "github.com/GLEZH/gofermart-graduate-project/internal/usecase/accrual"
)

type fakeProcessor struct {
	mu         sync.Mutex
	items      []order.Order
	pendingErr error
	processErr error
	processed  int
	running    int
	peak       int
	delay      time.Duration
}

func (f *fakeProcessor) Pending(context.Context, int) ([]order.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.items, f.pendingErr
}

func (f *fakeProcessor) Process(ctx context.Context, _ order.Order) error {
	f.mu.Lock()
	f.processed++
	f.running++
	if f.running > f.peak {
		f.peak = f.running
	}
	delay, err := f.delay, f.processErr
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(delay):
		}
	}

	f.mu.Lock()
	f.running--
	f.mu.Unlock()
	return err
}

func (f *fakeProcessor) counters() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.processed, f.peak
}

func TestProcessBatch(t *testing.T) {
	processor := &fakeProcessor{items: []order.Order{{Number: "9278923470"}}}
	worker := NewAccrual(processor, time.Second, nil)
	delay := worker.processBatch(context.Background())
	processed, _ := processor.counters()
	if delay != time.Second || processed != 1 {
		t.Fatalf("processBatch() delay=%s processed=%d", delay, processed)
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

func TestProcessBatchRunsOrdersConcurrently(t *testing.T) {
	items := make([]order.Order, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		items = append(items, order.Order{Number: order.Number("9278923470")})
	}
	processor := &fakeProcessor{items: items, delay: 20 * time.Millisecond}
	worker := NewAccrual(processor, time.Second, nil)

	started := time.Now()
	worker.processBatch(context.Background())
	elapsed := time.Since(started)

	processed, peak := processor.counters()
	if processed != len(items) {
		t.Fatalf("processed = %d, want %d", processed, len(items))
	}
	if peak < 2 || peak > worker.workers {
		t.Fatalf("peak concurrency = %d, want between 2 and %d", peak, worker.workers)
	}
	if sequential := time.Duration(len(items)) * processor.delay; elapsed >= sequential {
		t.Fatalf("batch took %s, sequential run takes %s", elapsed, sequential)
	}
}

func TestProcessBatchEmpty(t *testing.T) {
	processor := &fakeProcessor{}
	if delay := NewAccrual(processor, time.Second, nil).processBatch(context.Background()); delay != time.Second {
		t.Fatalf("empty batch delay = %s", delay)
	}
	if processed, _ := processor.counters(); processed != 0 {
		t.Fatalf("processed = %d, want 0", processed)
	}
}

func TestRunStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewAccrual(&fakeProcessor{}, time.Millisecond, nil).Run(ctx); err != nil {
		t.Fatal(err)
	}
}

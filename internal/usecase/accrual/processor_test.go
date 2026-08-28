package accrual

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
)

type fakeCalculator struct {
	calculation Calculation
	err         error
}

func (f fakeCalculator) Calculate(context.Context, order.Number) (Calculation, error) {
	return f.calculation, f.err
}

type fakeOrders struct {
	pending []order.Order
	status  order.Status
	amount  loyalty.Amount
	err     error
}

func (f *fakeOrders) Pending(context.Context, int) ([]order.Order, error) { return f.pending, f.err }
func (f *fakeOrders) UpdateStatus(_ context.Context, _ order.Number, status order.Status) error {
	f.status = status
	return f.err
}
func (f *fakeOrders) Settle(_ context.Context, _ order.Number, amount *loyalty.Amount) error {
	f.status = order.StatusProcessed
	if amount != nil {
		f.amount = *amount
	}
	return f.err
}

func TestProcessor(t *testing.T) {
	amount := loyalty.Amount(50050)
	tests := []struct {
		name       string
		result     Calculation
		calculator error
		wantStatus order.Status
		wantAmount loyalty.Amount
		wantErr    bool
	}{
		{name: "not found", result: Calculation{Found: false}},
		{name: "registered", result: Calculation{Found: true, Status: "REGISTERED"}, wantStatus: order.StatusNew},
		{name: "processing", result: Calculation{Found: true, Status: "PROCESSING"}, wantStatus: order.StatusProcessing},
		{name: "invalid", result: Calculation{Found: true, Status: "INVALID"}, wantStatus: order.StatusInvalid},
		{name: "processed", result: Calculation{Found: true, Status: "PROCESSED", Accrual: &amount}, wantStatus: order.StatusProcessed, wantAmount: amount},
		{name: "processed without amount", result: Calculation{Found: true, Status: "PROCESSED"}, wantStatus: order.StatusProcessed},
		{name: "unknown", result: Calculation{Found: true, Status: "UNKNOWN"}, wantErr: true},
		{name: "client error", calculator: errors.New("client"), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeOrders{}
			processor := NewProcessor(fakeCalculator{calculation: test.result, err: test.calculator}, store)
			err := processor.Process(context.Background(), order.Order{Number: "9278923470"})
			if (err != nil) != test.wantErr || store.status != test.wantStatus || store.amount != test.wantAmount {
				t.Fatalf("Process() status=%s amount=%s error=%v", store.status, store.amount.String(), err)
			}
		})
	}
}

func TestProcessorPendingAndRateLimit(t *testing.T) {
	store := &fakeOrders{pending: []order.Order{{Number: "9278923470"}}}
	processor := NewProcessor(fakeCalculator{}, store)
	items, err := processor.Pending(context.Background(), 1)
	if err != nil || len(items) != 1 {
		t.Fatalf("Pending() = %v, %v", items, err)
	}
	rateLimit := &RateLimitError{RetryAfter: time.Minute}
	if rateLimit.Error() == "" {
		t.Fatal("empty rate limit error")
	}
}

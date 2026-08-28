package accrual

import (
	"context"
	"errors"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
)

var errUnknownStatus = errors.New("unknown accrual status")

type Processor struct {
	calculator calculator
	orders     orderStore
}

func NewProcessor(calculator calculator, orders orderStore) *Processor {
	return &Processor{calculator: calculator, orders: orders}
}

func (p *Processor) Pending(ctx context.Context, limit int) ([]order.Order, error) {
	return p.orders.Pending(ctx, limit)
}

func (p *Processor) Process(ctx context.Context, target order.Order) error {
	calculation, err := p.calculator.Calculate(ctx, target.Number)
	if err != nil || !calculation.Found {
		return err
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

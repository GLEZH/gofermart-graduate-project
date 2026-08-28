package orders

import (
	"context"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type Service struct {
	store store
}

func NewService(store store) *Service {
	return &Service{store: store}
}

func (s *Service) Submit(ctx context.Context, userID user.ID, value string) error {
	number, err := order.ParseNumber(value)
	if err != nil {
		return err
	}
	return s.store.Submit(ctx, userID, number)
}

func (s *Service) List(ctx context.Context, userID user.ID) ([]order.Order, error) {
	return s.store.ListByUser(ctx, userID)
}

package orders

import (
	"context"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type store interface {
	Submit(ctx context.Context, userID user.ID, number order.Number) error
	ListByUser(ctx context.Context, userID user.ID) ([]order.Order, error)
}

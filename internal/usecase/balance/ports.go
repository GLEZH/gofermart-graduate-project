package balance

import (
	"context"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type store interface {
	GetAccount(ctx context.Context, userID user.ID) (loyalty.Account, error)
	Withdraw(ctx context.Context, userID user.ID, number order.Number, amount loyalty.Amount) error
	ListWithdrawals(ctx context.Context, userID user.ID) ([]loyalty.Withdrawal, error)
}

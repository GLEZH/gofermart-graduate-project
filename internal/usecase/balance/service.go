package balance

import (
	"context"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type Service struct {
	store store
}

func NewService(store store) *Service {
	return &Service{store: store}
}

func (s *Service) Get(ctx context.Context, userID user.ID) (loyalty.Account, error) {
	return s.store.GetAccount(ctx, userID)
}

func (s *Service) Withdraw(ctx context.Context, userID user.ID, orderValue string, amount loyalty.Amount) error {
	number, err := order.ParseNumber(orderValue)
	if err != nil {
		return err
	}
	if amount <= 0 {
		return loyalty.ErrInvalidAmount
	}
	return s.store.Withdraw(ctx, userID, number, amount)
}

func (s *Service) Withdrawals(ctx context.Context, userID user.ID) ([]loyalty.Withdrawal, error) {
	return s.store.ListWithdrawals(ctx, userID)
}

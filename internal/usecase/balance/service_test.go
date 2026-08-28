package balance

import (
	"context"
	"errors"
	"testing"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type fakeStore struct {
	account     loyalty.Account
	withdrawals []loyalty.Withdrawal
	err         error
}

func (f *fakeStore) GetAccount(context.Context, user.ID) (loyalty.Account, error) {
	return f.account, f.err
}
func (f *fakeStore) Withdraw(context.Context, user.ID, order.Number, loyalty.Amount) error {
	return f.err
}
func (f *fakeStore) ListWithdrawals(context.Context, user.ID) ([]loyalty.Withdrawal, error) {
	return f.withdrawals, f.err
}

func TestService(t *testing.T) {
	store := &fakeStore{account: loyalty.Account{Current: 100}, withdrawals: []loyalty.Withdrawal{{Order: "9278923470"}}}
	service := NewService(store)
	account, err := service.Get(context.Background(), 1)
	if err != nil || account.Current != 100 {
		t.Fatalf("Get() = %+v, %v", account, err)
	}
	if err = service.Withdraw(context.Background(), 1, "9278923470", 100); err != nil {
		t.Fatal(err)
	}
	if err = service.Withdraw(context.Background(), 1, "bad", 100); !errors.Is(err, order.ErrInvalidNumber) {
		t.Fatalf("Withdraw() error = %v", err)
	}
	if err = service.Withdraw(context.Background(), 1, "9278923470", 0); !errors.Is(err, loyalty.ErrInvalidAmount) {
		t.Fatalf("Withdraw() error = %v", err)
	}
	items, err := service.Withdrawals(context.Background(), 1)
	if err != nil || len(items) != 1 {
		t.Fatalf("Withdrawals() = %v, %v", items, err)
	}
}

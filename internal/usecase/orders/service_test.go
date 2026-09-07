package orders

import (
	"context"
	"errors"
	"testing"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

type fakeStore struct {
	items []order.Order
	err   error
}

func (f *fakeStore) Submit(context.Context, user.ID, order.Number) error { return f.err }
func (f *fakeStore) ListByUser(context.Context, user.ID) ([]order.Order, error) {
	return f.items, f.err
}

func TestService(t *testing.T) {
	store := &fakeStore{items: []order.Order{{Number: "9278923470"}}}
	service := NewService(store)
	if err := service.Submit(context.Background(), 1, "9278923470"); err != nil {
		t.Fatal(err)
	}
	if err := service.Submit(context.Background(), 1, "123"); !errors.Is(err, order.ErrInvalidNumber) {
		t.Fatalf("Submit() error = %v", err)
	}
	items, err := service.List(context.Background(), 1)
	if err != nil || len(items) != 1 {
		t.Fatalf("List() = %v, %v", items, err)
	}
}

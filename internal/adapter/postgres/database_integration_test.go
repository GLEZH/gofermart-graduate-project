package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
)

func testDatabase(t *testing.T) *Database {
	t.Helper()
	uri := os.Getenv("TEST_DATABASE_URI")
	if uri == "" {
		t.Skip("TEST_DATABASE_URI is not set")
	}
	database, err := NewDatabase(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.db.Exec(`DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public`)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.db.Exec(`DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public`)
		_ = database.Close()
	})
	return database
}

func TestMigrations(t *testing.T) {
	database := testDatabase(t)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"users", "loyalty_accounts", "orders", "withdrawals"} {
		var exists bool
		if err := database.db.QueryRow(`SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&exists); err != nil || !exists {
			t.Fatalf("table %s exists=%t error=%v", table, exists, err)
		}
	}
	if err := database.Down(); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := database.db.QueryRow(`SELECT to_regclass('public.users') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("users after down exists=%t error=%v", exists, err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
}

func TestStore(t *testing.T) {
	database := testDatabase(t)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(database.SQLDB())
	ctx := context.Background()
	first, err := store.CreateWithAccount(ctx, "first", "hash")
	if err != nil || first.ID == 0 {
		t.Fatalf("CreateWithAccount() = %+v, %v", first, err)
	}
	if _, err = store.CreateWithAccount(ctx, "first", "hash"); !errors.Is(err, user.ErrLoginTaken) {
		t.Fatalf("duplicate user error = %v", err)
	}
	found, err := store.FindByLogin(ctx, "first")
	if err != nil || found.ID != first.ID || found.PasswordHash != "hash" {
		t.Fatalf("FindByLogin() = %+v, %v", found, err)
	}
	if _, err = store.FindByLogin(ctx, "missing"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("missing user error = %v", err)
	}
	second, err := store.CreateWithAccount(ctx, "second", "hash")
	if err != nil {
		t.Fatal(err)
	}
	number := order.Number("9278923470")
	if err = store.Submit(ctx, first.ID, number); err != nil {
		t.Fatal(err)
	}
	if err = store.Submit(ctx, first.ID, number); !errors.Is(err, order.ErrOwnedByUser) {
		t.Fatalf("same owner error = %v", err)
	}
	if err = store.Submit(ctx, second.ID, number); !errors.Is(err, order.ErrOwnedByAnotherUser) {
		t.Fatalf("other owner error = %v", err)
	}
	items, err := store.ListByUser(ctx, first.ID)
	if err != nil || len(items) != 1 || items[0].Status != order.StatusNew {
		t.Fatalf("ListByUser() = %+v, %v", items, err)
	}
	pending, err := store.Pending(ctx, 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("Pending() = %+v, %v", pending, err)
	}
	if err = store.UpdateStatus(ctx, number, order.StatusProcessing); err != nil {
		t.Fatal(err)
	}
	accrual := loyalty.Amount(10000)
	if err = store.Settle(ctx, number, &accrual); err != nil {
		t.Fatal(err)
	}
	if err = store.Settle(ctx, number, &accrual); err != nil {
		t.Fatal(err)
	}
	account, err := store.GetAccount(ctx, first.ID)
	if err != nil || account.Current != 10000 {
		t.Fatalf("GetAccount() = %+v, %v", account, err)
	}
	if err = store.Withdraw(ctx, first.ID, order.Number("12345678903"), 20000); !errors.Is(err, loyalty.ErrInsufficientFunds) {
		t.Fatalf("insufficient error = %v", err)
	}
	if err = store.Withdraw(ctx, first.ID, order.Number("12345678903"), 2500); err != nil {
		t.Fatal(err)
	}
	withdrawals, err := store.ListWithdrawals(ctx, first.ID)
	if err != nil || len(withdrawals) != 1 || withdrawals[0].Amount != 2500 {
		t.Fatalf("ListWithdrawals() = %+v, %v", withdrawals, err)
	}
	account, _ = store.GetAccount(ctx, first.ID)
	if account.Current != 7500 || account.Withdrawn != 2500 {
		t.Fatalf("account after withdrawal = %+v", account)
	}
	items, err = store.ListByUser(ctx, first.ID)
	if err != nil || items[0].Accrual == nil || *items[0].Accrual != 10000 {
		t.Fatalf("processed orders = %+v, %v", items, err)
	}
	if database.SQLDB() == nil {
		t.Fatal("SQLDB() = nil")
	}
}

func TestPendingSkipsDeferredOrders(t *testing.T) {
	database := testDatabase(t)
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(database.SQLDB())
	ctx := context.Background()
	owner, err := store.CreateWithAccount(ctx, "deferred", "hash")
	if err != nil {
		t.Fatal(err)
	}
	unknown, known := order.Number("9278923470"), order.Number("12345678903")
	for _, number := range []order.Number{unknown, known} {
		if err = store.Submit(ctx, owner.ID, number); err != nil {
			t.Fatal(err)
		}
	}
	if pending, err := store.Pending(ctx, 10); err != nil || len(pending) != 2 {
		t.Fatalf("Pending() = %+v, %v", pending, err)
	}

	if err = store.Defer(ctx, unknown, time.Minute); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Pending(ctx, 1)
	if err != nil || len(pending) != 1 || pending[0].Number != known {
		t.Fatalf("deferred order still occupies the batch: %+v, %v", pending, err)
	}

	if err = store.Defer(ctx, unknown, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	pending, err = store.Pending(ctx, 10)
	if err != nil || len(pending) != 2 {
		t.Fatalf("deferred order did not return: %+v, %v", pending, err)
	}
	for _, item := range pending {
		if item.Number == unknown && item.Attempts != 2 {
			t.Fatalf("attempts = %d, want 2", item.Attempts)
		}
	}

	if err = store.UpdateStatus(ctx, unknown, order.StatusProcessing); err != nil {
		t.Fatal(err)
	}
	pending, err = store.Pending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range pending {
		if item.Number == unknown && item.Attempts != 0 {
			t.Fatalf("attempts after status update = %d, want 0", item.Attempts)
		}
	}
}

func TestNewDatabaseValidation(t *testing.T) {
	if _, err := NewDatabase(context.Background(), ""); err == nil {
		t.Fatal("empty URI error = nil")
	}
	if _, err := NewDatabase(context.Background(), "invalid"); err == nil {
		t.Fatal("invalid URI error = nil")
	}
}

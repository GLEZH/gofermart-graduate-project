package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/GLEZH/gofermart-graduate-project/internal/domain/loyalty"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/order"
	"github.com/GLEZH/gofermart-graduate-project/internal/domain/user"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

const operationTimeout = 3 * time.Second

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateWithAccount(ctx context.Context, login, passwordHash string) (user.User, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return user.User{}, fmt.Errorf("begin create user: %w", err)
	}
	defer tx.Rollback()

	created := user.User{Login: login, PasswordHash: passwordHash}
	err = tx.QueryRowContext(ctx, `INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id, created_at`, login, passwordHash).Scan(&created.ID, &created.CreatedAt)
	if err != nil {
		if uniqueViolation(err) {
			return user.User{}, user.ErrLoginTaken
		}
		return user.User{}, fmt.Errorf("insert user: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO loyalty_accounts (user_id) VALUES ($1)`, created.ID); err != nil {
		return user.User{}, fmt.Errorf("insert account: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return user.User{}, fmt.Errorf("commit create user: %w", err)
	}
	return created, nil
}

func (s *Store) FindByLogin(ctx context.Context, login string) (user.User, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	var found user.User
	err := s.db.QueryRowContext(ctx, `SELECT id, login, password_hash, created_at FROM users WHERE login = $1`, login).Scan(&found.ID, &found.Login, &found.PasswordHash, &found.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return user.User{}, user.ErrNotFound
	}
	if err != nil {
		return user.User{}, fmt.Errorf("find user: %w", err)
	}
	return found, nil
}

func (s *Store) Submit(ctx context.Context, userID user.ID, number order.Number) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO orders (number, user_id) VALUES ($1, $2)`, number.String(), userID)
	if err == nil {
		return nil
	}
	if !uniqueViolation(err) {
		return fmt.Errorf("insert order: %w", err)
	}
	var owner user.ID
	if err = s.db.QueryRowContext(ctx, `SELECT user_id FROM orders WHERE number = $1`, number.String()).Scan(&owner); err != nil {
		return fmt.Errorf("find order owner: %w", err)
	}
	if owner == userID {
		return order.ErrOwnedByUser
	}
	return order.ErrOwnedByAnotherUser
}

func (s *Store) ListByUser(ctx context.Context, userID user.ID) ([]order.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT number, status, accrual_amount, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC, number DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()
	items := make([]order.Order, 0)
	for rows.Next() {
		var item order.Order
		var number string
		var amount sql.NullInt64
		if err = rows.Scan(&number, &item.Status, &amount, &item.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		item.Number = order.Number(number)
		item.UserID = userID
		if amount.Valid {
			value := loyalty.Amount(amount.Int64)
			item.Accrual = &value
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("read orders: %w", err)
	}
	return items, nil
}

func (s *Store) GetAccount(ctx context.Context, userID user.ID) (loyalty.Account, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	account := loyalty.Account{UserID: userID}
	err := s.db.QueryRowContext(ctx, `SELECT current_amount, withdrawn_amount FROM loyalty_accounts WHERE user_id = $1`, userID).Scan(&account.Current, &account.Withdrawn)
	if err != nil {
		return loyalty.Account{}, fmt.Errorf("get account: %w", err)
	}
	return account, nil
}

func (s *Store) Withdraw(ctx context.Context, userID user.ID, number order.Number, amount loyalty.Amount) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin withdrawal: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE loyalty_accounts SET current_amount = current_amount - $2, withdrawn_amount = withdrawn_amount + $2 WHERE user_id = $1 AND current_amount >= $2`, userID, amount)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("withdraw rows affected: %w", err)
	}
	if updated == 0 {
		return loyalty.ErrInsufficientFunds
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO withdrawals (user_id, order_number, amount) VALUES ($1, $2, $3)`, userID, number.String(), amount); err != nil {
		return fmt.Errorf("insert withdrawal: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit withdrawal: %w", err)
	}
	return nil
}

func (s *Store) ListWithdrawals(ctx context.Context, userID user.ID) ([]loyalty.Withdrawal, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT order_number, amount, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	defer rows.Close()
	items := make([]loyalty.Withdrawal, 0)
	for rows.Next() {
		item := loyalty.Withdrawal{UserID: userID}
		if err = rows.Scan(&item.Order, &item.Amount, &item.ProcessedAt); err != nil {
			return nil, fmt.Errorf("scan withdrawal: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("read withdrawals: %w", err)
	}
	return items, nil
}

func (s *Store) Pending(ctx context.Context, limit int) ([]order.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT number, user_id, status, uploaded_at, attempts FROM orders WHERE status IN ('NEW', 'PROCESSING') AND next_attempt_at <= now() ORDER BY next_attempt_at LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending orders: %w", err)
	}
	defer rows.Close()
	items := make([]order.Order, 0)
	for rows.Next() {
		var item order.Order
		var number string
		if err = rows.Scan(&number, &item.UserID, &item.Status, &item.UploadedAt, &item.Attempts); err != nil {
			return nil, fmt.Errorf("scan pending order: %w", err)
		}
		item.Number = order.Number(number)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateStatus(ctx context.Context, number order.Number, status order.Status) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET status = $2, attempts = 0, next_attempt_at = now() WHERE number = $1 AND status NOT IN ('INVALID', 'PROCESSED')`, number.String(), status)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	return nil
}

func (s *Store) Defer(ctx context.Context, number order.Number, retryAfter time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `UPDATE orders SET attempts = attempts + 1, next_attempt_at = now() + make_interval(secs => $2) WHERE number = $1 AND status IN ('NEW', 'PROCESSING')`, number.String(), retryAfter.Seconds())
	if err != nil {
		return fmt.Errorf("defer order: %w", err)
	}
	return nil
}

func (s *Store) Settle(ctx context.Context, number order.Number, amount *loyalty.Amount) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin settlement: %w", err)
	}
	defer tx.Rollback()
	var storedAmount any
	credit := loyalty.Amount(0)
	if amount != nil {
		storedAmount = int64(*amount)
		credit = *amount
	}
	var userID user.ID
	err = tx.QueryRowContext(ctx, `UPDATE orders SET status = 'PROCESSED', accrual_amount = $2 WHERE number = $1 AND status NOT IN ('INVALID', 'PROCESSED') RETURNING user_id`, number.String(), storedAmount).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("settle order: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE loyalty_accounts SET current_amount = current_amount + $2 WHERE user_id = $1`, userID, credit); err != nil {
		return fmt.Errorf("credit account: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit settlement: %w", err)
	}
	return nil
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

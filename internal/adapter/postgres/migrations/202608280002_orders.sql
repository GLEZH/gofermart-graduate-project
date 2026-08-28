-- +goose Up
CREATE TABLE orders (
    number TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED')),
    accrual_amount BIGINT CHECK (accrual_amount IS NULL OR accrual_amount >= 0),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX orders_user_uploaded_idx ON orders (user_id, uploaded_at DESC);
CREATE INDEX orders_pending_idx ON orders (uploaded_at) WHERE status IN ('NEW', 'PROCESSING');

-- +goose Down
DROP TABLE orders;

-- +goose Up
CREATE TYPE order_status AS ENUM ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED');

CREATE TABLE orders (
    number TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status order_status NOT NULL DEFAULT 'NEW',
    accrual_amount BIGINT CHECK (accrual_amount IS NULL OR accrual_amount >= 0),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX orders_user_uploaded_idx ON orders (user_id, uploaded_at DESC);
CREATE INDEX orders_pending_idx ON orders (uploaded_at) WHERE status IN ('NEW', 'PROCESSING');

-- +goose Down
DROP TABLE orders;
DROP TYPE order_status;

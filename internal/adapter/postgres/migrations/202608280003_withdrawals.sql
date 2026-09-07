-- +goose Up
CREATE TABLE withdrawals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_number TEXT NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX withdrawals_user_processed_idx ON withdrawals (user_id, processed_at DESC);

-- +goose Down
DROP TABLE withdrawals;

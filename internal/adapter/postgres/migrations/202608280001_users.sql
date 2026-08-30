-- +goose Up
CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE loyalty_accounts (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_amount BIGINT NOT NULL DEFAULT 0 CHECK (current_amount >= 0),
    withdrawn_amount BIGINT NOT NULL DEFAULT 0 CHECK (withdrawn_amount >= 0)
);

-- +goose Down
DROP TABLE loyalty_accounts;
DROP TABLE users;

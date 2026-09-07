-- +goose Up
ALTER TABLE orders ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();

DROP INDEX orders_pending_idx;
CREATE INDEX orders_pending_idx ON orders (next_attempt_at) WHERE status IN ('NEW', 'PROCESSING');

-- +goose Down
DROP INDEX orders_pending_idx;
CREATE INDEX orders_pending_idx ON orders (uploaded_at) WHERE status IN ('NEW', 'PROCESSING');

ALTER TABLE orders DROP COLUMN next_attempt_at;
ALTER TABLE orders DROP COLUMN attempts;

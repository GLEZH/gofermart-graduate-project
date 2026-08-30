package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

const (
	migrationsDir   = "migrations"
	maxConnections  = 20
	maxConnIdleTime = 5 * time.Minute
	connectTimeout  = 5 * time.Second
)

type Database struct {
	pool *pgxpool.Pool
}

func NewDatabase(ctx context.Context, uri string) (*Database, error) {
	if uri == "" {
		return nil, errors.New("database URI is empty")
	}
	poolConfig, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return nil, fmt.Errorf("parse database URI: %w", err)
	}
	poolConfig.MaxConns = maxConnections
	poolConfig.MaxConnIdleTime = maxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err = pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Database{pool: pool}, nil
}

func (d *Database) Migrate() error {
	return d.runMigrations(func(db *sql.DB) error { return goose.Up(db, migrationsDir) })
}

func (d *Database) Down() error {
	return d.runMigrations(func(db *sql.DB) error { return goose.DownTo(db, migrationsDir, 0) })
}

func (d *Database) runMigrations(action func(*sql.DB) error) error {
	goose.SetBaseFS(migrations)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	db := stdlib.OpenDBFromPool(d.pool)
	defer db.Close()
	return action(db)
}

func (d *Database) Pool() *pgxpool.Pool {
	return d.pool
}

func (d *Database) Close() {
	d.pool.Close()
}

package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Database struct {
	db *sql.DB
}

func NewDatabase(ctx context.Context, uri string) (*Database, error) {
	if uri == "" {
		return nil, errors.New("database URI is empty")
	}
	db, err := sql.Open("pgx", uri)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Database{db: db}, nil
}

func (d *Database) Migrate() error {
	goose.SetBaseFS(migrations)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(d.db, "migrations")
}

func (d *Database) Down() error {
	goose.SetBaseFS(migrations)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.DownTo(d.db, "migrations", 0)
}

func (d *Database) SQLDB() *sql.DB {
	return d.db
}

func (d *Database) Close() error {
	return d.db.Close()
}

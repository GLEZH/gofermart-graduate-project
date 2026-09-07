package config

import (
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		clearEnv(t)
		cfg, err := New(nil)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.RunAddress != "localhost:8080" || cfg.AuthSecret != "" || cfg.DatabaseURI != "" || cfg.AccrualPollInterval != time.Second {
			t.Fatalf("unexpected config: %+v", cfg)
		}
	})

	t.Run("flags and env", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("RUN_ADDRESS", "env:8081")
		t.Setenv("DATABASE_URI", "env-db")
		t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "env-accrual")
		t.Setenv("AUTH_SECRET", "env-secret")
		t.Setenv("ACCRUAL_POLL_INTERVAL", "25ms")
		cfg, err := New([]string{"-a", "flag:8080", "-d", "flag-db", "-r", "flag-accrual", "-s", "flag-secret"})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.RunAddress != "env:8081" || cfg.DatabaseURI != "env-db" || cfg.AccrualSystemAddress != "env-accrual" || cfg.AuthSecret != "env-secret" || cfg.AccrualPollInterval != 25*time.Millisecond {
			t.Fatalf("unexpected config: %+v", cfg)
		}
	})

	t.Run("database URI from parts", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DB_HOST", "db")
		t.Setenv("DB_PORT", "6432")
		t.Setenv("DB_USER", "gophermart")
		t.Setenv("DB_PASSWORD", "p@ss word")
		t.Setenv("DB_TABLE_NAME", "loyalty")
		cfg, err := New(nil)
		if err != nil {
			t.Fatal(err)
		}
		want := "postgres://gophermart:p%40ss%20word@db:6432/loyalty?sslmode=disable"
		if cfg.DatabaseURI != want {
			t.Fatalf("DatabaseURI = %s, want %s", cfg.DatabaseURI, want)
		}
	})

	t.Run("database URI parts defaults", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DB_PORT", "5432")
		t.Setenv("DB_USER", "gophermart")
		t.Setenv("DB_TABLE_NAME", "gophermart")
		cfg, err := New(nil)
		if err != nil {
			t.Fatal(err)
		}
		want := "postgres://gophermart@localhost:5432/gophermart?sslmode=disable"
		if cfg.DatabaseURI != want {
			t.Fatalf("DatabaseURI = %s, want %s", cfg.DatabaseURI, want)
		}
		t.Setenv("DB_SSLMODE", "require")
		t.Setenv("DATABASE_URI", "postgres://explicit")
		if cfg, err = New(nil); err != nil || cfg.DatabaseURI != "postgres://explicit" {
			t.Fatalf("DATABASE_URI must win: %+v, %v", cfg, err)
		}
	})

	t.Run("incomplete database parts", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DB_USER", "gophermart")
		t.Setenv("DB_PASSWORD", "gophermart")
		cfg, err := New(nil)
		if err != nil || cfg.DatabaseURI != "" {
			t.Fatalf("DatabaseURI = %q, %v", cfg.DatabaseURI, err)
		}
	})

	t.Run("errors", func(t *testing.T) {
		clearEnv(t)
		if _, err := New([]string{"-unknown"}); err == nil {
			t.Fatal("unknown flag error = nil")
		}
		t.Setenv("ACCRUAL_POLL_INTERVAL", "invalid")
		if _, err := New(nil); err == nil {
			t.Fatal("duration error = nil")
		}
	})
}

func clearEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "AUTH_SECRET", "ACCRUAL_POLL_INTERVAL",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_TABLE_NAME", "DB_SSLMODE",
	}
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if ok {
			t.Cleanup(func() { _ = os.Setenv(key, value) })
		} else {
			t.Cleanup(func() { _ = os.Unsetenv(key) })
		}
		_ = os.Unsetenv(key)
	}
}

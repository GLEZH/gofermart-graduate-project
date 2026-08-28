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
		if cfg.RunAddress != "localhost:8080" || cfg.AuthSecret != defaultAuthSecret || cfg.AccrualPollInterval != time.Second {
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
	for _, key := range []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "AUTH_SECRET", "ACCRUAL_POLL_INTERVAL"} {
		value, ok := os.LookupEnv(key)
		if ok {
			t.Cleanup(func() { _ = os.Setenv(key, value) })
		} else {
			t.Cleanup(func() { _ = os.Unsetenv(key) })
		}
		_ = os.Unsetenv(key)
	}
}

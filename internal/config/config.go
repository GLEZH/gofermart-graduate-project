package config

import (
	"flag"
	"os"
	"time"
)

const defaultAuthSecret = "gophermart-development-secret"

type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
	AuthSecret           string
	AccrualPollInterval  time.Duration
}

func New(args []string) (*Config, error) {
	cfg := &Config{}
	flags := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	flags.StringVar(&cfg.RunAddress, "a", "localhost:8080", "HTTP server address")
	flags.StringVar(&cfg.DatabaseURI, "d", "", "database connection string")
	flags.StringVar(&cfg.AccrualSystemAddress, "r", "", "accrual system address")
	flags.StringVar(&cfg.AuthSecret, "s", defaultAuthSecret, "authentication secret")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	override(&cfg.RunAddress, "RUN_ADDRESS")
	override(&cfg.DatabaseURI, "DATABASE_URI")
	override(&cfg.AccrualSystemAddress, "ACCRUAL_SYSTEM_ADDRESS")
	override(&cfg.AuthSecret, "AUTH_SECRET")

	cfg.AccrualPollInterval = time.Second
	if value := os.Getenv("ACCRUAL_POLL_INTERVAL"); value != "" {
		interval, err := time.ParseDuration(value)
		if err != nil {
			return nil, err
		}
		cfg.AccrualPollInterval = interval
	}
	return cfg, nil
}

func override(target *string, key string) {
	if value, ok := os.LookupEnv(key); ok {
		*target = value
	}
}

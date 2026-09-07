package config

import (
	"flag"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultDatabaseHost    = "localhost"
	defaultDatabaseSSLMode = "disable"
)

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
	flags.StringVar(&cfg.AuthSecret, "s", "", "authentication secret")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	override(&cfg.RunAddress, "RUN_ADDRESS")
	override(&cfg.DatabaseURI, "DATABASE_URI")
	override(&cfg.AccrualSystemAddress, "ACCRUAL_SYSTEM_ADDRESS")
	override(&cfg.AuthSecret, "AUTH_SECRET")
	if strings.TrimSpace(cfg.DatabaseURI) == "" {
		cfg.DatabaseURI = DatabaseURIFromEnv()
	}

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

func DatabaseURIFromEnv() string {
	user, name, port := os.Getenv("DB_USER"), os.Getenv("DB_TABLE_NAME"), os.Getenv("DB_PORT")
	if user == "" || name == "" || port == "" {
		return ""
	}
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = defaultDatabaseHost
	}
	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = defaultDatabaseSSLMode
	}
	credentials := url.User(user)
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		credentials = url.UserPassword(user, password)
	}
	uri := url.URL{
		Scheme:   "postgres",
		User:     credentials,
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + name,
		RawQuery: "sslmode=" + sslMode,
	}
	return uri.String()
}

func override(target *string, key string) {
	if value, ok := os.LookupEnv(key); ok {
		*target = value
	}
}

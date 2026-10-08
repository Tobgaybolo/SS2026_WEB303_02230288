package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Config holds every setting, read from environment variables (see .env.example).
type Config struct {
	GRPCPort    string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	CatalogAddr string // host:port of the Catalog service's gRPC server
	Resilience  ResilienceConfig
}

func loadConfig() (Config, error) {
	var missing []string
	get := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := Config{
		GRPCPort:    get("GRPC_PORT"),
		DBHost:      get("DB_HOST"),
		DBPort:      get("DB_PORT"),
		DBUser:      get("DB_USER"),
		DBPassword:  get("DB_PASSWORD"),
		DBName:      get("DB_NAME"),
		CatalogAddr: get("CATALOG_ADDR"),
	}
	callTimeout := get("CATALOG_TIMEOUT")
	maxAttempts := get("CATALOG_MAX_ATTEMPTS")
	retryBase := get("CATALOG_RETRY_BASE")
	threshold := get("CATALOG_BREAKER_THRESHOLD")
	openTimeout := get("CATALOG_BREAKER_OPEN_TIMEOUT")

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}

	var err error
	if cfg.Resilience.CallTimeout, err = parseDuration("CATALOG_TIMEOUT", callTimeout); err != nil {
		return Config{}, err
	}
	if cfg.Resilience.RetryBase, err = parseDuration("CATALOG_RETRY_BASE", retryBase); err != nil {
		return Config{}, err
	}
	if cfg.Resilience.BreakerOpenTimeout, err = parseDuration("CATALOG_BREAKER_OPEN_TIMEOUT", openTimeout); err != nil {
		return Config{}, err
	}
	if cfg.Resilience.MaxAttempts, err = parsePositiveInt("CATALOG_MAX_ATTEMPTS", maxAttempts); err != nil {
		return Config{}, err
	}
	th, err := parsePositiveInt("CATALOG_BREAKER_THRESHOLD", threshold)
	if err != nil {
		return Config{}, err
	}
	cfg.Resilience.BreakerThreshold = uint32(th)

	return cfg, nil
}

func parseDuration(key, v string) (time.Duration, error) {
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as 1s, got %q", key, v)
	}
	return d, nil
}

func parsePositiveInt(key, v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, v)
	}
	return n, nil
}

func (c Config) DatabaseURL() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DBUser, c.DBPassword),
		Host:     net.JoinHostPort(c.DBHost, c.DBPort),
		Path:     c.DBName,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}
package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

// Config holds every setting the service needs. All values come from
// environment variables so nothing is hard-coded (see .env.example).
type Config struct {
	GRPCPort   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

// loadConfig reads the environment and fails fast, listing every missing
// variable at once, instead of starting in a half-configured state.
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
		GRPCPort:   get("GRPC_PORT"),
		DBHost:     get("DB_HOST"),
		DBPort:     get("DB_PORT"),
		DBUser:     get("DB_USER"),
		DBPassword: get("DB_PASSWORD"),
		DBName:     get("DB_NAME"),
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}
	return cfg, nil
}

// DatabaseURL builds a Postgres connection string, escaping the credentials safely.
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
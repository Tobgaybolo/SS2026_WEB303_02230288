package main

import (
	"fmt"
	"os"
	"time"
)

// Config holds every setting, read from environment variables (see .env.example).
type Config struct {
	Port            string
	CatalogAddr     string        // gRPC address of the Catalog service
	PurchaseAddr    string        // gRPC address of the Purchase service
	UpstreamTimeout time.Duration // max time for one downstream gRPC call
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
		Port:         get("GATEWAY_PORT"),
		CatalogAddr:  get("CATALOG_ADDR"),
		PurchaseAddr: get("PURCHASE_ADDR"),
	}
	timeoutStr := get("UPSTREAM_TIMEOUT")
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("UPSTREAM_TIMEOUT must be a positive duration such as 10s")
	}
	cfg.UpstreamTimeout = timeout
	return cfg, nil
}
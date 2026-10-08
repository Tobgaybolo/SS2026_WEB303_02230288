package main

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"time"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ResilienceConfig tunes the protection around calls to Catalog.
type ResilienceConfig struct {
	CallTimeout        time.Duration // deadline for ONE attempt
	MaxAttempts        int           // total attempts, including the first
	RetryBase          time.Duration // first backoff delay; doubles each retry
	BreakerThreshold   uint32        // consecutive failures that open the circuit
	BreakerOpenTimeout time.Duration // how long the circuit stays open before probing
}

// ResilientCatalogClient decorates a CatalogClient with timeout, retry with
// backoff, and a circuit breaker. The handlers only see the CatalogClient
// interface, so none of them needed to change.
type ResilientCatalogClient struct {
	next    CatalogClient
	cfg     ResilienceConfig
	breaker *gobreaker.CircuitBreaker[*PlantInfo]
}

func NewResilientCatalogClient(next CatalogClient, cfg ResilienceConfig) *ResilientCatalogClient {
	breaker := gobreaker.NewCircuitBreaker[*PlantInfo](gobreaker.Settings{
		Name:        "catalog",
		MaxRequests: 1,                      // half-open: let one probe request through
		Interval:    0,                      // closed state: never reset counts on a timer
		Timeout:     cfg.BreakerOpenTimeout, // open -> half-open after this long
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= cfg.BreakerThreshold
		},
		// Only "Catalog is unreachable or too slow" counts against the breaker.
		// NOT_FOUND, INVALID_ARGUMENT etc. mean Catalog answered, so it is healthy.
		IsSuccessful: func(err error) bool {
			return err == nil || !isUnavailable(err)
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("circuit breaker %q: %s -> %s", name, from, to)
		},
	})
	return &ResilientCatalogClient{next: next, cfg: cfg, breaker: breaker}
}

// GetPlant calls Catalog with all three protections applied. It returns gRPC
// status errors: UNAVAILABLE when Catalog cannot be reached (or the circuit is
// open), and Catalog's own NOT_FOUND / INVALID_ARGUMENT answers unchanged.
func (c *ResilientCatalogClient) GetPlant(ctx context.Context, id string) (*PlantInfo, error) {
	var lastErr error

	for attempt := 1; attempt <= c.cfg.MaxAttempts; attempt++ {
		// The caller (e.g. the gateway) may already have run out of time.
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}

		plant, err := c.breaker.Execute(func() (*PlantInfo, error) {
			callCtx, cancel := context.WithTimeout(ctx, c.cfg.CallTimeout) // per-attempt timeout
			defer cancel()
			return c.next.GetPlant(callCtx, id)
		})
		if err == nil {
			return plant, nil
		}

		// Circuit open (or a probe already in flight): fail fast, never retry.
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			log.Printf("catalog call rejected: circuit breaker is not accepting requests")
			return nil, status.Error(codes.Unavailable, "catalog circuit breaker is open")
		}

		// A real answer from Catalog (NOT_FOUND, ...): retrying would not change it.
		if !isUnavailable(err) {
			return nil, err
		}

		lastErr = err
		if attempt == c.cfg.MaxAttempts {
			break
		}

		delay := c.backoff(attempt)
		log.Printf("catalog GetPlant attempt %d/%d failed (%s); retrying in %s",
			attempt, c.cfg.MaxAttempts, status.Code(err), delay.Round(time.Millisecond))

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	}

	log.Printf("catalog GetPlant failed after %d attempts: %v", c.cfg.MaxAttempts, lastErr)
	return nil, status.Errorf(codes.Unavailable, "catalog unavailable after %d attempts", c.cfg.MaxAttempts)
}

// backoff returns the delay before the next retry: exponential growth
// (base, 2*base, 4*base...) with "equal jitter", i.e. a random value between
// half and the full delay, so many callers don't retry at the same instant.
func (c *ResilientCatalogClient) backoff(attempt int) time.Duration {
	d := c.cfg.RetryBase << (attempt - 1)
	half := d / 2
	if half <= 0 {
		return d
	}
	return half + rand.N(half)
}
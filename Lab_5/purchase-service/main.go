package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	purchasev1 "plant-nursery/proto/purchase/v1"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := connectWithRetry(ctx, cfg.DatabaseURL(), 15, 2*time.Second)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	repo := NewRepository(pool)
	if err := repo.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// grpc.NewClient connects lazily, so this service starts fine even if
	// Catalog is not up yet. The reconnect backoff is capped at 2s so we notice
	// quickly when Catalog comes back (the default cap is 2 minutes).
	conn, err := grpc.NewClient(cfg.CatalogAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  200 * time.Millisecond,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   2 * time.Second,
			},
			MinConnectTimeout: 2 * time.Second,
		}),
	)
	if err != nil {
		log.Fatalf("catalog client: %v", err)
	}
	defer conn.Close()

	// Plain gRPC client wrapped with timeout + retry + circuit breaker.
	catalog := NewResilientCatalogClient(NewGRPCCatalogClient(conn), cfg.Resilience)
	log.Printf("catalog resilience: timeout=%s attempts=%d retry-base=%s breaker-threshold=%d breaker-open=%s",
		cfg.Resilience.CallTimeout, cfg.Resilience.MaxAttempts, cfg.Resilience.RetryBase,
		cfg.Resilience.BreakerThreshold, cfg.Resilience.BreakerOpenTimeout)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	purchasev1.RegisterPurchaseServiceServer(grpcServer, NewServer(repo, catalog))
	reflection.Register(grpcServer)

	// Stop gracefully on Ctrl+C or when Docker sends SIGTERM.
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Println("shutting down purchase-service...")
		grpcServer.GracefulStop()
	}()

	log.Printf("purchase-service listening on :%s (catalog at %s)", cfg.GRPCPort, cfg.CatalogAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// connectWithRetry waits for the database to accept connections. Under Docker
// Compose the service can start before Postgres is ready, so we retry.
func connectWithRetry(ctx context.Context, dsn string, attempts int, delay time.Duration) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = pool.Ping(pingCtx)
			cancel()
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		log.Printf("database not ready (attempt %d/%d): %v", i, attempts, err)
		time.Sleep(delay)
	}
	return nil, lastErr
}
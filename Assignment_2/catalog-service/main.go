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
	"google.golang.org/grpc/reflection"

	catalogv1 "plant-nursery/proto/catalog/v1"
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

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(grpcServer, NewServer(repo))
	reflection.Register(grpcServer) // lets grpcurl discover the API; handy for development

	// Stop gracefully on Ctrl+C or when Docker sends SIGTERM.
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Println("shutting down catalog-service...")
		grpcServer.GracefulStop()
	}()

	log.Printf("catalog-service listening on :%s", cfg.GRPCPort)
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
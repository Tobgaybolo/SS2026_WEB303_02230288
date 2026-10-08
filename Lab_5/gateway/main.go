package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	catalogv1 "plant-nursery/proto/catalog/v1"
	purchasev1 "plant-nursery/proto/purchase/v1"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// grpc.NewClient connects lazily, so the gateway starts even if a service is down.
	catalogConn, err := grpc.NewClient(cfg.CatalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("catalog client: %v", err)
	}
	defer catalogConn.Close()

	purchaseConn, err := grpc.NewClient(cfg.PurchaseAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("purchase client: %v", err)
	}
	defer purchaseConn.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	(&PlantHandler{client: catalogv1.NewCatalogServiceClient(catalogConn), timeout: cfg.UpstreamTimeout}).Register(mux)
	(&PurchaseHandler{client: purchasev1.NewPurchaseServiceClient(purchaseConn), timeout: cfg.UpstreamTimeout}).Register(mux)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           recoverPanics(logging(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      cfg.UpstreamTimeout + 5*time.Second, // must outlast a downstream call
	}

	// Drain in-flight requests on Ctrl+C or when Docker sends SIGTERM.
	done := make(chan struct{})
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Println("shutting down gateway...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
		close(done)
	}()

	log.Printf("gateway listening on :%s (catalog=%s, purchase=%s)", cfg.Port, cfg.CatalogAddr, cfg.PurchaseAddr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
	<-done
}
package main

import (
	"context"

	"google.golang.org/grpc"

	catalogv1 "plant-nursery/proto/catalog/v1"
)

// PlantInfo is the subset of Catalog data this service cares about.
type PlantInfo struct {
	ID         string
	Name       string
	PriceCents int64
	Stock      int32
}

// CatalogClient is how Purchase talks to Catalog. It is an interface so that
// Step 5 can wrap it with timeout/retry/circuit-breaker behaviour (and tests
// can substitute a fake) without touching the handlers.
type CatalogClient interface {
	// GetPlant returns gRPC status errors (NOT_FOUND, UNAVAILABLE, ...).
	GetPlant(ctx context.Context, id string) (*PlantInfo, error)
}

type grpcCatalogClient struct {
	client catalogv1.CatalogServiceClient
}

func NewGRPCCatalogClient(conn grpc.ClientConnInterface) CatalogClient {
	return &grpcCatalogClient{client: catalogv1.NewCatalogServiceClient(conn)}
}

func (c *grpcCatalogClient) GetPlant(ctx context.Context, id string) (*PlantInfo, error) {
	p, err := c.client.GetPlant(ctx, &catalogv1.GetPlantRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return &PlantInfo{ID: p.GetId(), Name: p.GetName(), PriceCents: p.GetPriceCents(), Stock: p.GetStock()}, nil
}
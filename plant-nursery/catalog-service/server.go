package main

import (
	"context"
	"errors"
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "plant-nursery/proto/catalog/v1"
)

// Server implements the CatalogService gRPC API defined in catalog.proto.
type Server struct {
	catalogv1.UnimplementedCatalogServiceServer
	repo *Repository
}

func NewServer(repo *Repository) *Server {
	return &Server{repo: repo}
}

func (s *Server) CreatePlant(ctx context.Context, req *catalogv1.CreatePlantRequest) (*catalogv1.Plant, error) {
	in := newPlantInput(req.GetName(), req.GetSpecies(), req.GetPriceCents(), req.GetStock())
	if err := validatePlantInput(in); err != nil {
		return nil, err
	}
	rec, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, repoError("create plant", err)
	}
	return toProto(rec), nil
}

func (s *Server) GetPlant(ctx context.Context, req *catalogv1.GetPlantRequest) (*catalogv1.Plant, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	rec, err := s.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, repoError("get plant", err)
	}
	return toProto(rec), nil
}

func (s *Server) ListPlants(ctx context.Context, _ *catalogv1.ListPlantsRequest) (*catalogv1.ListPlantsResponse, error) {
	recs, err := s.repo.List(ctx)
	if err != nil {
		return nil, repoError("list plants", err)
	}
	plants := make([]*catalogv1.Plant, 0, len(recs))
	for _, rec := range recs {
		plants = append(plants, toProto(rec))
	}
	return &catalogv1.ListPlantsResponse{Plants: plants}, nil
}

func (s *Server) UpdatePlant(ctx context.Context, req *catalogv1.UpdatePlantRequest) (*catalogv1.Plant, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	in := newPlantInput(req.GetName(), req.GetSpecies(), req.GetPriceCents(), req.GetStock())
	if err := validatePlantInput(in); err != nil {
		return nil, err
	}
	rec, err := s.repo.Update(ctx, req.GetId(), in)
	if err != nil {
		return nil, repoError("update plant", err)
	}
	return toProto(rec), nil
}

func (s *Server) DeletePlant(ctx context.Context, req *catalogv1.DeletePlantRequest) (*catalogv1.DeletePlantResponse, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, repoError("delete plant", err)
	}
	return &catalogv1.DeletePlantResponse{}, nil
}

// repoError converts repository errors into gRPC status errors. Unknown errors
// are logged in full but only a generic message goes back to the caller, so
// database details never leak through the API.
func repoError(op string, err error) error {
	if errors.Is(err, errNotFound) {
		return status.Error(codes.NotFound, "plant not found")
	}
	log.Printf("%s failed: %v", op, err)
	return status.Error(codes.Internal, "internal error")
}

func toProto(p *plantRecord) *catalogv1.Plant {
	return &catalogv1.Plant{
		Id:         p.ID,
		Name:       p.Name,
		Species:    p.Species,
		PriceCents: p.PriceCents,
		Stock:      p.Stock,
		CreatedAt:  p.CreatedAt.UTC().Format(time.RFC3339),
	}
}
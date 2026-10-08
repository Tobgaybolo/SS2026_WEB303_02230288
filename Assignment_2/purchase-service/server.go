package main

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	purchasev1 "plant-nursery/proto/purchase/v1"
)

// Server implements the PurchaseService gRPC API defined in purchase.proto.
type Server struct {
	purchasev1.UnimplementedPurchaseServiceServer
	repo    *Repository
	catalog CatalogClient
}

func NewServer(repo *Repository, catalog CatalogClient) *Server {
	return &Server{repo: repo, catalog: catalog}
}

// CreatePurchase validates the request, confirms the plant exists (and is in
// stock) by calling Catalog over gRPC, then stores a purchase with a snapshot
// of the plant's name and price.
func (s *Server) CreatePurchase(ctx context.Context, req *purchasev1.CreatePurchaseRequest) (*purchasev1.Purchase, error) {
	plantID := strings.TrimSpace(req.GetPlantId())
	customer := strings.TrimSpace(req.GetCustomerName())
	if err := validateCreate(plantID, req.GetQuantity(), customer); err != nil {
		return nil, err
	}

	plant, err := s.catalog.GetPlant(ctx, plantID)
	if err != nil {
		return nil, catalogError(err)
	}
	if err := checkStock(plant, req.GetQuantity()); err != nil {
		return nil, err
	}

	rec, err := s.repo.Create(ctx, newPurchase{
		PlantID:        plant.ID,
		PlantName:      plant.Name,
		UnitPriceCents: plant.PriceCents,
		Quantity:       req.GetQuantity(),
		CustomerName:   customer,
	})
	if err != nil {
		return nil, repoError("create purchase", err)
	}

	p := toProto(rec)
	p.CurrentPriceCents = plant.PriceCents
	p.CurrentStock = plant.Stock
	return p, nil
}

// GetPurchase returns the stored purchase enriched with live Catalog data.
func (s *Server) GetPurchase(ctx context.Context, req *purchasev1.GetPurchaseRequest) (*purchasev1.Purchase, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	rec, err := s.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, repoError("get purchase", err)
	}
	p := toProto(rec)
	s.enrich(ctx, []*purchasev1.Purchase{p})
	return p, nil
}

func (s *Server) ListPurchases(ctx context.Context, _ *purchasev1.ListPurchasesRequest) (*purchasev1.ListPurchasesResponse, error) {
	recs, err := s.repo.List(ctx)
	if err != nil {
		return nil, repoError("list purchases", err)
	}
	purchases := make([]*purchasev1.Purchase, 0, len(recs))
	for _, rec := range recs {
		purchases = append(purchases, toProto(rec))
	}
	s.enrich(ctx, purchases)
	return &purchasev1.ListPurchasesResponse{Purchases: purchases}, nil
}

// UpdatePurchase changes quantity, customer name and status. Only a quantity
// change needs Catalog (to re-check stock); the unit price stays as snapshotted.
func (s *Server) UpdatePurchase(ctx context.Context, req *purchasev1.UpdatePurchaseRequest) (*purchasev1.Purchase, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	customer := strings.TrimSpace(req.GetCustomerName())
	st := strings.ToUpper(strings.TrimSpace(req.GetStatus()))
	if err := validateUpdate(req.GetQuantity(), customer, st); err != nil {
		return nil, err
	}

	existing, err := s.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, repoError("update purchase", err)
	}

	if req.GetQuantity() != existing.Quantity {
		plant, err := s.catalog.GetPlant(ctx, existing.PlantID)
		if err != nil {
			return nil, catalogError(err)
		}
		if err := checkStock(plant, req.GetQuantity()); err != nil {
			return nil, err
		}
	}

	total := existing.UnitPriceCents * int64(req.GetQuantity())
	rec, err := s.repo.Update(ctx, req.GetId(), req.GetQuantity(), total, customer, st)
	if err != nil {
		return nil, repoError("update purchase", err)
	}
	p := toProto(rec)
	s.enrich(ctx, []*purchasev1.Purchase{p})
	return p, nil
}

func (s *Server) DeletePurchase(ctx context.Context, req *purchasev1.DeletePurchaseRequest) (*purchasev1.DeletePurchaseResponse, error) {
	if err := validateID(req.GetId()); err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, repoError("delete purchase", err)
	}
	return &purchasev1.DeletePurchaseResponse{}, nil
}

// enrich attaches live Catalog data to purchases (one lookup per distinct
// plant). This is the read-side FALLBACK: if Catalog cannot be reached, the
// purchase is still returned from our own database with plant_details_stale set,
// instead of failing the whole request.
func (s *Server) enrich(ctx context.Context, purchases []*purchasev1.Purchase) {
	lookups := make(map[string]*PlantInfo) // a nil value means the lookup failed
	catalogDown := false

	for _, p := range purchases {
		info, seen := lookups[p.PlantId]
		if !seen {
			if !catalogDown {
				var err error
				info, err = s.catalog.GetPlant(ctx, p.PlantId)
				if err != nil {
					log.Printf("catalog lookup for plant %s failed: %v", p.PlantId, err)
					info = nil
					// No point hammering a service that is down: skip the rest.
					catalogDown = isUnavailable(err)
				}
			}
			lookups[p.PlantId] = info
		}

		if info == nil {
			p.PlantDetailsStale = true
			continue
		}
		p.CurrentPriceCents = info.PriceCents
		p.CurrentStock = info.Stock
	}
}

func checkStock(plant *PlantInfo, quantity int32) error {
	if plant.Stock < quantity {
		return status.Errorf(codes.FailedPrecondition,
			"insufficient stock for %q: requested %d, available %d", plant.Name, quantity, plant.Stock)
	}
	return nil
}

// catalogError translates an error from the Catalog call into what our own
// callers should see.
func catalogError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return status.Error(codes.NotFound, "plant not found in catalog")
	case codes.InvalidArgument:
		return status.Error(codes.InvalidArgument, status.Convert(err).Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		return status.Error(codes.Unavailable, "catalog service is unavailable, please try again later")
	default:
		log.Printf("unexpected catalog error: %v", err)
		return status.Error(codes.Internal, "internal error")
	}
}

func isUnavailable(err error) bool {
	c := status.Code(err)
	return c == codes.Unavailable || c == codes.DeadlineExceeded
}

// repoError converts repository errors into gRPC status errors without leaking
// database details to the caller.
func repoError(op string, err error) error {
	if errors.Is(err, errNotFound) {
		return status.Error(codes.NotFound, "purchase not found")
	}
	log.Printf("%s failed: %v", op, err)
	return status.Error(codes.Internal, "internal error")
}

func toProto(r *purchaseRecord) *purchasev1.Purchase {
	return &purchasev1.Purchase{
		Id:             r.ID,
		PlantId:        r.PlantID,
		PlantName:      r.PlantName,
		UnitPriceCents: r.UnitPriceCents,
		Quantity:       r.Quantity,
		TotalCents:     r.TotalCents,
		CustomerName:   r.CustomerName,
		Status:         r.Status,
		CreatedAt:      r.CreatedAt.UTC().Format(time.RFC3339),
	}
}
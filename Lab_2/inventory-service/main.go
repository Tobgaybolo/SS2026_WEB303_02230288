package main

import (
	"context"
	"log"
	"net"

	pb "Lab_2/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type InventoryServiceServer struct {
	pb.UnimplementedInventoryServiceServer
	stock map[string]int32
}

func (s *InventoryServiceServer) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.StockInfo, error) {
	qty, exists := s.stock[req.GetProductId()]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "product %s not found in inventory", req.GetProductId())
	}
	return &pb.StockInfo{
		ProductId:         req.GetProductId(),
		QuantityAvailable: qty,
	}, nil
}

func (s *InventoryServiceServer) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	qty, exists := s.stock[req.GetProductId()]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "product %s not found in inventory", req.GetProductId())
	}
	if qty < req.GetQuantity() {
		return nil, status.Errorf(codes.FailedPrecondition,
			"insufficient stock for %s: requested %d, available %d",
			req.GetProductId(), req.GetQuantity(), qty)
	}
	s.stock[req.GetProductId()] -= req.GetQuantity()
	return &pb.ReserveStockResponse{
		Success:            true,
		Message:            "stock reserved successfully",
		RemainingQuantity:  s.stock[req.GetProductId()],
	}, nil
}

func main() {
	stock := map[string]int32{
		"P001": 10,
		"P002": 25,
		"P003": 40,
		"P004": 5,
	}

	listener, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen on port 50052: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterInventoryServiceServer(grpcServer, &InventoryServiceServer{stock: stock})
	reflection.Register(grpcServer)

	log.Println("Inventory Service is running on port 50052...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}
}
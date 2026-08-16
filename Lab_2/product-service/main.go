package main

import (
	"context"
	"log"
	"net"

	pb "Lab_2/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/reflection"
)

// ProductServiceServer implements the ProductService
// defined in product.proto.
type ProductServiceServer struct {
	pb.UnimplementedProductServiceServer

	products map[string]*pb.Product
}

// GetProduct receives a product ID and returns the
// corresponding product information.
func (s *ProductServiceServer) GetProduct(
	ctx context.Context,
	req *pb.GetProductRequest,
) (*pb.Product, error) {

	// Check whether the product ID was provided.
	if req.GetProductId() == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"product ID is required",
		)
	}

	// Search for the product.
	product, exists := s.products[req.GetProductId()]

	// If the product does not exist, return NOT_FOUND.
	if !exists {
		return nil, status.Errorf(
			codes.NotFound,
			"product with ID %s not found",
			req.GetProductId(),
		)
	}

	// Return the product information.
	return product, nil
}

func main() {

	// Product information stored in memory.
	products := map[string]*pb.Product{
		"P001": {
			ProductId: "P001",
			Name:      "Laptop",
			Price:     75000,
		},
		"P002": {
			ProductId: "P002",
			Name:      "Mechanical Keyboard",
			Price:     4500,
		},
		"P003": {
			ProductId: "P003",
			Name:      "Wireless Mouse",
			Price:     1800,
		},
		"P004": {
			ProductId: "P004",
			Name:      "Monitor",
			Price:     25000,
		},
	}

	// Create the Product Service server.
	productServer := &ProductServiceServer{
		products: products,
	}

	// Listen on port 50051.
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen on port 50051: %v", err)
	}

	// Create a gRPC server.
	grpcServer := grpc.NewServer()

	// Register the Product Service.
	pb.RegisterProductServiceServer(
		grpcServer,
		productServer,
	)

	// Enable server reflection so tools like grpcurl can discover services.
	reflection.Register(grpcServer)

	log.Println("Product Service is running on port 50051...")

	// Start the gRPC server.
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}
}
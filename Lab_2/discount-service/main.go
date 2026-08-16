package main

import (
	"log"
	"net"
	"time"

	pb "Lab_2/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type DiscountServiceServer struct {
	pb.UnimplementedDiscountServiceServer
	discounts map[string][]*pb.Discount
}

func (s *DiscountServiceServer) StreamDiscounts(req *pb.DiscountRequest, stream pb.DiscountService_StreamDiscountsServer) error {
	list, exists := s.discounts[req.GetProductId()]
	if !exists {
		return nil // no discounts, stream just ends
	}
	for _, d := range list {
		if err := stream.Send(d); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond) // simulate discounts arriving over time
	}
	return nil
}

func main() {
	discounts := map[string][]*pb.Discount{
		"P001": {
			{DiscountId: "D1", Description: "Back to school sale", PercentageOff: 10},
			{DiscountId: "D2", Description: "Loyalty member discount", PercentageOff: 5},
		},
		"P004": {
			{DiscountId: "D3", Description: "Clearance", PercentageOff: 20},
		},
	}

	listener, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("failed to listen on port 50053: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterDiscountServiceServer(grpcServer, &DiscountServiceServer{discounts: discounts})
	reflection.Register(grpcServer)

	log.Println("Discount Service is running on port 50053...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}
}
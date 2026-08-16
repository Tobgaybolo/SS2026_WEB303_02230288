package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	pb "Lab_2/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type NotificationServiceServer struct {
	pb.UnimplementedNotificationServiceServer
}

// Client-streaming: receive a stream of order updates, reply once at the end.
func (s *NotificationServiceServer) SendOrderUpdates(stream pb.NotificationService_SendOrderUpdatesServer) error {
	count := int32(0)
	for {
		update, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.NotificationSummary{
				UpdatesReceived: count,
				Message:         "all order updates processed",
			})
		}
		if err != nil {
			return err
		}
		log.Printf("received update for order %s: %s", update.GetOrderId(), update.GetStatus())
		count++
	}
}

// Bidirectional streaming: echo back a server message for every client message.
func (s *NotificationServiceServer) SubscribeNotifications(stream pb.NotificationService_SubscribeNotificationsServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		log.Printf("message from client %s: %s", msg.GetClientId(), msg.GetMessage())

		reply := &pb.ServerMessage{
			Message:   fmt.Sprintf("acknowledged: %s", msg.GetMessage()),
			Timestamp: time.Now().Format(time.RFC3339),
		}
		if err := stream.Send(reply); err != nil {
			return err
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", ":50054")
	if err != nil {
		log.Fatalf("failed to listen on port 50054: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterNotificationServiceServer(grpcServer, &NotificationServiceServer{})
	reflection.Register(grpcServer)

	log.Println("Notification Service is running on port 50054...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}
}
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	pb "Lab_2/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient(
		"localhost:50054",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to Notification Service: %v", err)
	}
	defer conn.Close()

	client := pb.NewNotificationServiceClient(conn)

	// No fixed timeout here — the stream stays open for the life of the
	// conversation, so we cancel manually once both sides are done.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.SubscribeNotifications(ctx)
	if err != nil {
		log.Fatalf("failed to open bidi stream: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Goroutine: continuously receive server messages while we send.
	go func() {
		defer wg.Done()
		for {
			serverMsg, err := stream.Recv()
			if err == io.EOF {
				fmt.Println("server closed the stream")
				return
			}
			if err != nil {
				fmt.Printf("receive error: %v\n", err)
				return
			}
			fmt.Printf("[server @ %s]: %s\n", serverMsg.GetTimestamp(), serverMsg.GetMessage())
		}
	}()

	// Send a handful of client messages, spaced out to show the
	// interleaving of sends and receives.
	messages := []string{
		"client-1 connected",
		"checking order status",
		"any updates on my order?",
		"thanks, disconnecting now",
	}

	for _, m := range messages {
		msg := &pb.ClientMessage{
			ClientId: "client-1",
			Message:  m,
		}
		fmt.Printf("[client sending]: %s\n", m)
		if err := stream.Send(msg); err != nil {
			log.Fatalf("send error: %v", err)
		}
		time.Sleep(700 * time.Millisecond)
	}

	// Signal we're done sending; the server will finish echoing back
	// and then close its side, which ends the Recv() loop above.
	if err := stream.CloseSend(); err != nil {
		log.Fatalf("failed to close send stream: %v", err)
	}

	wg.Wait()
}
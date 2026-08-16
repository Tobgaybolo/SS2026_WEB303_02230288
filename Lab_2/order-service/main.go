package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	pb "Lab_2/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {

	// Check whether a product ID was provided.
	if len(os.Args) < 2 {
		log.Println("Usage: go run ./order-service <product-id>")
		log.Println("Example: go run ./order-service P001")
		return
	}

	productID := os.Args[1]

	// -----------------------------------------------------------
	// 1. Connect to the Product Service and fetch product info.
	// -----------------------------------------------------------
	productConn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to Product Service: %v", err)
	}
	defer productConn.Close()

	productClient := pb.NewProductServiceClient(productConn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	product, err := productClient.GetProduct(
		ctx,
		&pb.GetProductRequest{ProductId: productID},
	)

	if err != nil {
		grpcStatus, ok := status.FromError(err)
		if ok {
			fmt.Printf("gRPC Error [%s]: %s\n", grpcStatus.Code(), grpcStatus.Message())
		} else {
			fmt.Printf("Error: %v\n", err)
		}
		return
	}

	fmt.Println("Product Information")
	fmt.Println("--------------------")
	fmt.Printf("Product ID: %s\n", product.GetProductId())
	fmt.Printf("Name:       %s\n", product.GetName())
	fmt.Printf("Price:      %.2f\n", product.GetPrice())

	// -----------------------------------------------------------
	// 2. Connect to the Inventory Service and reserve stock.
	// -----------------------------------------------------------
	inventoryConn, err := grpc.NewClient(
		"localhost:50052",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to Inventory Service: %v", err)
	}
	defer inventoryConn.Close()

	inventoryClient := pb.NewInventoryServiceClient(inventoryConn)

	invCtx, invCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer invCancel()

	reserveResp, err := inventoryClient.ReserveStock(
		invCtx,
		&pb.ReserveStockRequest{ProductId: productID, Quantity: 1},
	)

	fmt.Println()
	fmt.Println("Inventory Check")
	fmt.Println("--------------------")
	if err != nil {
		grpcStatus, ok := status.FromError(err)
		if ok {
			fmt.Printf("gRPC Error [%s]: %s\n", grpcStatus.Code(), grpcStatus.Message())
		} else {
			fmt.Printf("Error: %v\n", err)
		}
	} else {
		fmt.Printf("Reserved:   %v\n", reserveResp.GetSuccess())
		fmt.Printf("Message:    %s\n", reserveResp.GetMessage())
		fmt.Printf("Remaining:  %d\n", reserveResp.GetRemainingQuantity())
	}

	// -----------------------------------------------------------
	// 3. Connect to the Discount Service and stream discounts.
	// -----------------------------------------------------------
	discountConn, err := grpc.NewClient(
		"localhost:50053",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to Discount Service: %v", err)
	}
	defer discountConn.Close()

	discountClient := pb.NewDiscountServiceClient(discountConn)

	discCtx, discCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer discCancel()

	stream, err := discountClient.StreamDiscounts(
		discCtx,
		&pb.DiscountRequest{ProductId: productID},
	)

	fmt.Println()
	fmt.Println("Available Discounts")
	fmt.Println("--------------------")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		found := false
		for {
			discount, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Printf("stream error: %v\n", err)
				break
			}
			found = true
			fmt.Printf("- %s (%.0f%% off): %s\n",
				discount.GetDiscountId(),
				discount.GetPercentageOff(),
				discount.GetDescription(),
			)
		}
		if !found {
			fmt.Println("No active discounts.")
		}
	}

	// -----------------------------------------------------------
	// 4. Connect to the Notification Service and stream order
	//    status updates (client-streaming).
	// -----------------------------------------------------------
	notifConn, err := grpc.NewClient(
		"localhost:50054",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to Notification Service: %v", err)
	}
	defer notifConn.Close()

	notifClient := pb.NewNotificationServiceClient(notifConn)

	notifCtx, notifCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer notifCancel()

	updateStream, err := notifClient.SendOrderUpdates(notifCtx)
	if err != nil {
		log.Fatalf("failed to open notification stream: %v", err)
	}

	statuses := []string{"CREATED", "STOCK_RESERVED", "CONFIRMED"}
	for _, s := range statuses {
		update := &pb.OrderUpdate{
			OrderId:   fmt.Sprintf("ORDER-%s", productID),
			Status:    s,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		if err := updateStream.Send(update); err != nil {
			log.Fatalf("failed to send order update: %v", err)
		}
		time.Sleep(200 * time.Millisecond) // simulate updates over time
	}

	summary, err := updateStream.CloseAndRecv()

	fmt.Println()
	fmt.Println("Notification Summary")
	fmt.Println("--------------------")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Updates received: %d\n", summary.GetUpdatesReceived())
		fmt.Printf("Message:          %s\n", summary.GetMessage())
	}
}
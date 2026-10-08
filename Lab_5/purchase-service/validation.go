package main

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	minQuantity        = 1
	maxQuantity        = 100
	maxCustomerNameLen = 100
)

var validStatuses = map[string]bool{
	"PENDING":   true,
	"COMPLETED": true,
	"CANCELLED": true,
}

// validateCreate checks a CreatePurchase request. All problems are reported together.
func validateCreate(plantID string, quantity int32, customerName string) error {
	var problems []string
	if plantID == "" {
		problems = append(problems, "plant_id is required")
	} else if _, err := uuid.Parse(plantID); err != nil {
		problems = append(problems, "plant_id must be a valid UUID")
	}
	problems = append(problems, checkQuantity(quantity)...)
	problems = append(problems, checkCustomerName(customerName)...)
	return invalidArgument(problems)
}

// validateUpdate checks an UpdatePurchase request (st is already upper-cased).
func validateUpdate(quantity int32, customerName, st string) error {
	var problems []string
	problems = append(problems, checkQuantity(quantity)...)
	problems = append(problems, checkCustomerName(customerName)...)
	if !validStatuses[st] {
		problems = append(problems, "status must be one of PENDING, COMPLETED, CANCELLED")
	}
	return invalidArgument(problems)
}

func checkQuantity(q int32) []string {
	if q < minQuantity || q > maxQuantity {
		return []string{"quantity must be between 1 and 100"}
	}
	return nil
}

func checkCustomerName(name string) []string {
	if name == "" {
		return []string{"customer_name is required"}
	}
	if utf8.RuneCountInString(name) > maxCustomerNameLen {
		return []string{"customer_name must be at most 100 characters"}
	}
	return nil
}

func validateID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return status.Error(codes.InvalidArgument, "id must be a valid UUID")
	}
	return nil
}

// invalidArgument turns a list of problems into an INVALID_ARGUMENT error
// (HTTP 400 at the gateway), or nil if there are none.
func invalidArgument(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return status.Error(codes.InvalidArgument, strings.Join(problems, "; "))
}
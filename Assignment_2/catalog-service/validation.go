package main

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxNameLen    = 100
	maxSpeciesLen = 100
)

// plantInput holds the fields shared by create and update requests.
type plantInput struct {
	Name       string
	Species    string
	PriceCents int64
	Stock      int32
}

// newPlantInput trims whitespace so "   " counts as empty.
func newPlantInput(name, species string, priceCents int64, stock int32) plantInput {
	return plantInput{
		Name:       strings.TrimSpace(name),
		Species:    strings.TrimSpace(species),
		PriceCents: priceCents,
		Stock:      stock,
	}
}

// validatePlantInput checks every rule and reports all problems together.
// Failures map to gRPC INVALID_ARGUMENT, which the gateway turns into HTTP 400.
func validatePlantInput(in plantInput) error {
	var problems []string

	if in.Name == "" {
		problems = append(problems, "name is required")
	} else if utf8.RuneCountInString(in.Name) > maxNameLen {
		problems = append(problems, "name must be at most 100 characters")
	}
	if in.Species == "" {
		problems = append(problems, "species is required")
	} else if utf8.RuneCountInString(in.Species) > maxSpeciesLen {
		problems = append(problems, "species must be at most 100 characters")
	}
	if in.PriceCents <= 0 {
		problems = append(problems, "price_cents must be greater than 0")
	}
	if in.Stock < 0 {
		problems = append(problems, "stock must not be negative")
	}

	if len(problems) > 0 {
		return status.Error(codes.InvalidArgument, strings.Join(problems, "; "))
	}
	return nil
}

// validateID rejects malformed IDs before they reach the database.
func validateID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return status.Error(codes.InvalidArgument, "id must be a valid UUID")
	}
	return nil
}
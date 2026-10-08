package main

import (
	"net/http"
	"time"

	catalogv1 "plant-nursery/proto/catalog/v1"
)

// plantRequest is the JSON body for creating or replacing a plant.
type plantRequest struct {
	Name       string `json:"name"`
	Species    string `json:"species"`
	PriceCents int64  `json:"price_cents"`
	Stock      int32  `json:"stock"`
}

// plantJSON is the JSON representation returned to clients.
type plantJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Species    string `json:"species"`
	PriceCents int64  `json:"price_cents"`
	Stock      int32  `json:"stock"`
	CreatedAt  string `json:"created_at"`
}

func plantToJSON(p *catalogv1.Plant) plantJSON {
	return plantJSON{
		ID:         p.GetId(),
		Name:       p.GetName(),
		Species:    p.GetSpecies(),
		PriceCents: p.GetPriceCents(),
		Stock:      p.GetStock(),
		CreatedAt:  p.GetCreatedAt(),
	}
}

// PlantHandler exposes the Catalog service as REST endpoints under /plants.
type PlantHandler struct {
	client  catalogv1.CatalogServiceClient
	timeout time.Duration
}

func (h *PlantHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /plants", h.create)
	mux.HandleFunc("GET /plants", h.list)
	mux.HandleFunc("GET /plants/{id}", h.get)
	mux.HandleFunc("PUT /plants/{id}", h.update)
	mux.HandleFunc("DELETE /plants/{id}", h.remove)
}

func (h *PlantHandler) create(w http.ResponseWriter, r *http.Request) {
	var body plantRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	p, err := h.client.CreatePlant(ctx, &catalogv1.CreatePlantRequest{
		Name: body.Name, Species: body.Species, PriceCents: body.PriceCents, Stock: body.Stock,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.Header().Set("Location", "/plants/"+p.GetId())
	writeJSON(w, http.StatusCreated, plantToJSON(p))
}

func (h *PlantHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	resp, err := h.client.ListPlants(ctx, &catalogv1.ListPlantsRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := make([]plantJSON, 0, len(resp.GetPlants())) // non-nil: an empty list encodes as [] not null
	for _, p := range resp.GetPlants() {
		out = append(out, plantToJSON(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *PlantHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	p, err := h.client.GetPlant(ctx, &catalogv1.GetPlantRequest{Id: r.PathValue("id")})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plantToJSON(p))
}

func (h *PlantHandler) update(w http.ResponseWriter, r *http.Request) {
	var body plantRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	p, err := h.client.UpdatePlant(ctx, &catalogv1.UpdatePlantRequest{
		Id: r.PathValue("id"), Name: body.Name, Species: body.Species,
		PriceCents: body.PriceCents, Stock: body.Stock,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plantToJSON(p))
}

func (h *PlantHandler) remove(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	if _, err := h.client.DeletePlant(ctx, &catalogv1.DeletePlantRequest{Id: r.PathValue("id")}); err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

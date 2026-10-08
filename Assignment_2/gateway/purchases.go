package main

import (
	"net/http"
	"time"

	purchasev1 "plant-nursery/proto/purchase/v1"
)

type createPurchaseRequest struct {
	PlantID      string `json:"plant_id"`
	Quantity     int32  `json:"quantity"`
	CustomerName string `json:"customer_name"`
}

type updatePurchaseRequest struct {
	Quantity     int32  `json:"quantity"`
	CustomerName string `json:"customer_name"`
	Status       string `json:"status"`
}

type purchaseJSON struct {
	ID                string `json:"id"`
	PlantID           string `json:"plant_id"`
	PlantName         string `json:"plant_name"`
	UnitPriceCents    int64  `json:"unit_price_cents"`
	Quantity          int32  `json:"quantity"`
	TotalCents        int64  `json:"total_cents"`
	CustomerName      string `json:"customer_name"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
	CurrentPriceCents int64  `json:"current_price_cents"`
	CurrentStock      int32  `json:"current_stock"`
	PlantDetailsStale bool   `json:"plant_details_stale"`
}

func purchaseToJSON(p *purchasev1.Purchase) purchaseJSON {
	return purchaseJSON{
		ID:                p.GetId(),
		PlantID:           p.GetPlantId(),
		PlantName:         p.GetPlantName(),
		UnitPriceCents:    p.GetUnitPriceCents(),
		Quantity:          p.GetQuantity(),
		TotalCents:        p.GetTotalCents(),
		CustomerName:      p.GetCustomerName(),
		Status:            p.GetStatus(),
		CreatedAt:         p.GetCreatedAt(),
		CurrentPriceCents: p.GetCurrentPriceCents(),
		CurrentStock:      p.GetCurrentStock(),
		PlantDetailsStale: p.GetPlantDetailsStale(),
	}
}

// PurchaseHandler exposes the Purchase service as REST endpoints under /purchases.
type PurchaseHandler struct {
	client  purchasev1.PurchaseServiceClient
	timeout time.Duration
}

func (h *PurchaseHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /purchases", h.create)
	mux.HandleFunc("GET /purchases", h.list)
	mux.HandleFunc("GET /purchases/{id}", h.get)
	mux.HandleFunc("PUT /purchases/{id}", h.update)
	mux.HandleFunc("DELETE /purchases/{id}", h.remove)
}

func (h *PurchaseHandler) create(w http.ResponseWriter, r *http.Request) {
	var body createPurchaseRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	p, err := h.client.CreatePurchase(ctx, &purchasev1.CreatePurchaseRequest{
		PlantId: body.PlantID, Quantity: body.Quantity, CustomerName: body.CustomerName,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.Header().Set("Location", "/purchases/"+p.GetId())
	writeJSON(w, http.StatusCreated, purchaseToJSON(p))
}

func (h *PurchaseHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	resp, err := h.client.ListPurchases(ctx, &purchasev1.ListPurchasesRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := make([]purchaseJSON, 0, len(resp.GetPurchases()))
	for _, p := range resp.GetPurchases() {
		out = append(out, purchaseToJSON(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *PurchaseHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	p, err := h.client.GetPurchase(ctx, &purchasev1.GetPurchaseRequest{Id: r.PathValue("id")})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, purchaseToJSON(p))
}

func (h *PurchaseHandler) update(w http.ResponseWriter, r *http.Request) {
	var body updatePurchaseRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	p, err := h.client.UpdatePurchase(ctx, &purchasev1.UpdatePurchaseRequest{
		Id: r.PathValue("id"), Quantity: body.Quantity,
		CustomerName: body.CustomerName, Status: body.Status,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, purchaseToJSON(p))
}

func (h *PurchaseHandler) remove(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := upstreamCtx(r, h.timeout)
	defer cancel()

	if _, err := h.client.DeletePurchase(ctx, &purchasev1.DeletePurchaseRequest{Id: r.PathValue("id")}); err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
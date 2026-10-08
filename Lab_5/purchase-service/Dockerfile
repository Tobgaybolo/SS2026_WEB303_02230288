# syntax=docker/dockerfile:1

# ---- Stage 1: build ----
ARG GO_VERSION=1.26.5
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY proto/ proto/
COPY purchase-service/ purchase-service/

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/purchase-service ./purchase-service

# ---- Stage 2: runtime ----
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/purchase-service /purchase-service
USER nonroot:nonroot
EXPOSE 50052
ENTRYPOINT ["/purchase-service"]
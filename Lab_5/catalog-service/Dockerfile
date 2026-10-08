# syntax=docker/dockerfile:1

# ---- Stage 1: build ----
ARG GO_VERSION=1.26.5
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# Dependencies first: this layer is cached until go.mod / go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Only what this service needs: shared generated proto code + its own source.
COPY proto/ proto/
COPY catalog-service/ catalog-service/

# Static binary (no C libraries), smaller thanks to -s -w and -trimpath.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/catalog-service ./catalog-service

# ---- Stage 2: runtime ----
# Distroless: no shell, no package manager, runs as a non-root user.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/catalog-service /catalog-service
USER nonroot:nonroot
EXPOSE 50051
ENTRYPOINT ["/catalog-service"]
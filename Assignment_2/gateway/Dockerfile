# syntax=docker/dockerfile:1

# ---- Stage 1: build ----
ARG GO_VERSION=1.26.5
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY proto/ proto/
COPY gateway/ gateway/

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway ./gateway

# ---- Stage 2: runtime ----
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/gateway /gateway
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/gateway"]
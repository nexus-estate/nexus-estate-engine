# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.20

# =========================
# Base
# =========================
FROM golang:${GO_VERSION}-alpine AS base

WORKDIR /app

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    git

ENV CGO_ENABLED=0
ENV GO111MODULE=on

# =========================
# Dependencies
# =========================
FROM base AS deps

COPY go.mod go.sum ./
RUN go mod download

# =========================
# Development
# Hot reload with Air
# =========================
FROM base AS development

ARG AIR_VERSION=v1.61.7
RUN go install github.com/air-verse/air@${AIR_VERSION}

COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 50052

CMD ["air", "-c", ".air.toml"]

# =========================
# Builder
# =========================
FROM deps AS builder

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /app/bin/search-service \
    ./cmd/server

# =========================
# Release
# Same runtime style as prod, useful for staging/release testing
# =========================
FROM alpine:${ALPINE_VERSION} AS release

WORKDIR /app

RUN apk add --no-cache \
    ca-certificates \
    tzdata

COPY --from=builder /app/bin/search-service /app/search-service

EXPOSE 50052

CMD ["/app/search-service"]

# =========================
# Production
# Minimal non-root runtime
# =========================
FROM alpine:${ALPINE_VERSION} AS production

WORKDIR /app

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && addgroup -S appgroup \
    && adduser -S appuser -G appgroup

COPY --from=builder /app/bin/search-service /app/search-service

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 50052

CMD ["/app/search-service"]
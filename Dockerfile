# syntax=docker/dockerfile:1.7
# Make/CI derive this build argument from go.mod; direct builds use the same Go minor.
ARG GO_VERSION=1.25.0
FROM golang:${GO_VERSION}-alpine AS base
WORKDIR /app
ENV CGO_ENABLED=0
RUN apk add --no-cache ca-certificates tzdata

FROM base AS deps
COPY go.mod go.sum ./
RUN go mod download

FROM deps AS development
ARG AIR_VERSION=v1.61.7
RUN go install github.com/air-verse/air@${AIR_VERSION}
COPY . .
EXPOSE 50052
CMD ["air", "-c", ".air.search.toml"]

FROM deps AS build-search
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/nexus-search ./cmd/search
FROM deps AS build-engine
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/nexus-engine ./cmd/engine
FROM deps AS build-worker
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/nexus-worker ./cmd/worker

FROM alpine:3.22 AS runtime-base
RUN apk add --no-cache ca-certificates tzdata && addgroup -S app && adduser -S -G app app
WORKDIR /app
USER app

FROM runtime-base AS engine
COPY --from=build-engine /out/nexus-engine /app/nexus-engine
EXPOSE 50051
ENTRYPOINT ["/app/nexus-engine"]

FROM runtime-base AS worker
COPY --from=build-worker /out/nexus-worker /app/nexus-worker
ENTRYPOINT ["/app/nexus-worker"]

# Search remains the default target for compatibility with direct docker builds.
FROM runtime-base AS search
COPY --from=build-search /out/nexus-search /app/nexus-search
EXPOSE 50052
ENTRYPOINT ["/app/nexus-search"]

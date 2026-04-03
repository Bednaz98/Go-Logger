# syntax=docker/dockerfile:1

# Multi-target image: build with --target server | mcp (default final stage is server).

FROM golang:1.25-bookworm AS deps
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

FROM deps AS source
COPY . .

FROM source AS build-server
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/server ./cmd/server

FROM source AS build-mcp
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/mcp ./cmd/mcp

# Stdio MCP tool (publish as ghcr.io/<owner>/<repo>-mcp)
FROM debian:bookworm-slim AS mcp
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build-mcp /out/mcp /app/mcp
ENV DATABASE_URL=file:/data/logger.db?cache=shared
ENTRYPOINT ["/app/mcp"]

# API server: gRPC + HTTPS + MCP streamable HTTP (publish as ghcr.io/<owner>/<repo>-server; default build target)
FROM debian:bookworm-slim AS server
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build-server /out/server /app/server
ENV DATABASE_URL=file:/data/logger.db?cache=shared
EXPOSE 7443 8443 8444
ENTRYPOINT ["/app/server"]

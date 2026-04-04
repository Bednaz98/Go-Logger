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
# Override at build: docker build --build-arg GRPC_PORT=6000 ...
ARG GRPC_PORT=5000
ARG HTTP_PORT=5001
ARG MCP_HTTP_PORT=5002
ARG HTTP_PLAIN_PORT=5003
ENV GRPC_PORT=${GRPC_PORT}
ENV HTTP_PORT=${HTTP_PORT}
ENV MCP_HTTP_PORT=${MCP_HTTP_PORT}
ENV HTTP_PLAIN_PORT=${HTTP_PLAIN_PORT}
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build-server /out/server /app/server
ENV DATABASE_URL=file:/data/logger.db?cache=shared
EXPOSE ${GRPC_PORT} ${HTTP_PORT} ${MCP_HTTP_PORT} ${HTTP_PLAIN_PORT}
ENTRYPOINT ["/app/server"]

# syntax=docker/dockerfile:1

# Must match go.mod `go` directive (1.22 image cannot run go mod download for go 1.25 modules).
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Main API server (gRPC + HTTPS + MCP streamable HTTP)
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/server ./cmd/server

# MCP stdio tool (same DB env as server; use with -i for stdin)
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/mcp ./cmd/mcp

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/mcp /app/mcp
ENV DATABASE_URL=file:/data/logger.db?cache=shared
EXPOSE 7443 8443 8444
# Override with: docker run --entrypoint /app/mcp -i ...
ENTRYPOINT ["/app/server"]

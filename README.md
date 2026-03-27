# Go Logger

Multi-tenant **application / operational logs** and **analytics** with **gRPC** (TLS), **HTTPS JSON**, **MCP** (stdio + streamable HTTPS), and a **Go client SDK** with a pluggable local queue.

- **Specification:** [spec.md](spec.md)
- **Build plan:** [implementation-checklist.md](implementation-checklist.md)
- **HTTPS contract:** [docs/openapi.yaml](docs/openapi.yaml)

## Module path

This repository uses `github.com/joshuabednaz/go-logger`. Fork or rename by updating `go.mod`, `buf.gen.yaml` managed `go_package_prefix`, and regenerating protos (`buf generate`).

## Run the server

```bash
export LOGGER_AUTH_TOKEN="dev-token"          # optional; empty disables bearer check (dev)
export DATABASE_URL="file:logger.db?cache=shared"   # or postgres://...
go run ./cmd/server
```

Defaults: gRPC **TLS** on `0.0.0.0:7443`, HTTPS on `0.0.0.0:8443`, MCP HTTPS on `0.0.0.0:8444`. Without `TLS_*` env vars the server generates a **self-signed** cert and logs a **SHA-256 fingerprint**.

### TLS environment variables

| Variable | Purpose |
| -------- | ------- |
| `TLS_CERT_PATH` / `TLS_CERT_PEM` | Certificate (path wins over PEM) |
| `TLS_KEY_PATH` / `TLS_KEY_PEM` | Private key (path wins over PEM) |
| `TLS_MUST_USE_PROVIDED_CERT` | If `true`, disable auto-TLS when cert/key missing |
| `TLS_EXTRA_SAN_HOSTS` | Comma-separated extra DNS names or IPs for auto-TLS |

### Other server variables

| Variable | Default | Notes |
| -------- | ------- | ----- |
| `LISTEN_BIND_ADDRESS` | `0.0.0.0` | Bind address for all listeners |
| `GRPC_PORT` | `7443` | gRPC over TLS |
| `HTTP_PORT` | `8443` | HTTPS JSON API |
| `MCP_HTTP_PORT` | `8444` | MCP streamable HTTP (disable with `MCP_HTTP_LISTEN=false`) |
| `LOGGER_ENFORCE_METADATA_LIMIT` | `true` | Reject oversize `metadata_json` per record |
| `LOGGER_MAX_METADATA_BYTES` | `262144` | Metadata cap when enforcement on |
| `LOGGER_GRPC_MAX_RECV_BYTES` / `LOGGER_GRPC_MAX_SEND_BYTES` | `4194304` | gRPC message size |
| `MCP_ENABLE_DELETE_LOGS` | `false` | Enables destructive MCP / server delete tooling paths |

## curl (HTTPS)

Health is unauthenticated; other routes require `Authorization: Bearer <token>`.

```bash
curl -sk https://localhost:8443/api/v1/health
curl -sk https://localhost:8443/api/v1/ingest/batch \
  -H "Authorization: Bearer dev-token" -H "Content-Type: application/json" \
  -d '{"application_name":"demo","records":[{"log_id":"1","record_kind":"operational","application_name":"demo","log_message":"hello","event_timestamp":"2026-03-27T12:00:00Z","log_level":"info"}]}'
```

## grpcurl

```bash
grpcurl -insecure -H "authorization: Bearer dev-token" \
  -d '{"application_name":"demo","records":[{"log_id":"2","record_kind":"RECORD_KIND_OPERATIONAL","application_name":"demo","log_message":"hi","event_timestamp":"2026-03-27T12:00:00Z","log_level":"LOG_LEVEL_INFO"}]}' \
  localhost:7443 logger.v1.LoggerService/IngestBatch
```

## MCP (stdio)

The `cmd/mcp` binary speaks MCP over stdin/stdout and opens the database from `DATABASE_URL` (same schema as the server).

Example Cursor snippet: [docs/mcp-cursor-example.json](docs/mcp-cursor-example.json).

## Go client SDK

See `pkg/logger` (constructor `NewClient`) and reference SQLite store `pkg/sqllogstore`. The client requires a host-implemented `LocalLogStore` and uses gRPC `IngestBatch` with TLS (provide CA PEM or set `InsecureSkipVerify` for development only).

## Code generation

```bash
go run github.com/bufbuild/buf/cmd/buf@latest lint
go run github.com/bufbuild/buf/cmd/buf@latest generate
```

## License

See [LICENSE](LICENSE).

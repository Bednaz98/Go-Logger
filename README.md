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

### Docker images

The **`Dockerfile`** has two targets; CI publishes both to GHCR:

| Image | Target | Contents |
| ----- | ------ | -------- |
| **`ghcr.io/<owner>/<repo>-server`** | `server` | gRPC + HTTPS + MCP streamable HTTP |
| **`ghcr.io/<owner>/<repo>-mcp`** | `mcp` | stdio MCP only (smaller) |

Local build: `docker build --target server -t logger-server .` or `--target mcp -t logger-mcp .`

```bash
# API server
docker run --rm -p 7443:7443 -p 8443:8443 -p 8444:8444 \
  -e DATABASE_URL=file:/data/logger.db?cache=shared \
  -v logger-data:/data ghcr.io/joshuabednaz/go-logger-server:latest

# MCP stdio (needs -i)
docker run --rm -i \
  -e DATABASE_URL=file:/data/logger.db?cache=shared \
  -v logger-data:/data ghcr.io/joshuabednaz/go-logger-mcp:latest
```

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

### MCP remote forward (`ingest_batch`)

When **`MCP_REMOTE_GRPC_ADDRESS`** is set, the **`ingest_batch`** MCP tool writes to the **local** database and, by default, **forwards the same batch** to that remote **LoggerService** over gRPC (after local ingest succeeds).

| Variable | Default | Notes |
| -------- | ------- | ----- |
| `MCP_REMOTE_GRPC_ADDRESS` | *(empty)* | Remote target as `host:port` or `grpc://host:port`; empty disables forwarding |
| `MCP_REMOTE_SENDING` | `true` if address set, else ignored | Set to `false` to keep **`ingest_batch`** local-only even when the address is set |
| `MCP_REMOTE_BEARER_TOKEN` | falls back to `LOGGER_AUTH_TOKEN` | gRPC **authorization** metadata |
| `MCP_REMOTE_TLS_CA_PATH` | *(empty)* | PEM file for remote server trust (required unless insecure) |
| `MCP_REMOTE_INSECURE_SKIP_VERIFY` | `false` | Dev-only TLS skip (mutually exclusive with proper CA in production) |
| `MCP_REMOTE_STRICT` | `false` | If `true`, bad remote TLS/address config **exits the process**; if `false`, MCP still runs with local **`ingest_batch`** only |

`grpc://` and `grpcs://` in addresses are **parsed to host:port** only; the forward client **always uses TLS** (CA path or insecure flag), not plaintext gRPC.

Applies to **`cmd/mcp`** (stdio) and streamable **MCP HTTPS** on the main server when `MCP_HTTP_LISTEN=true`.

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

The `cmd/mcp` binary speaks MCP over stdin/stdout and opens the database from `DATABASE_URL` (same schema as the server). Tools include **`ingest_batch`** (local DB, optional remote forward — see **MCP remote forward** above), **`query_logs`**, **`get_log_by_id`**, etc.

Example Cursor snippet: [docs/mcp-cursor-example.json](docs/mcp-cursor-example.json).

## Go client SDK

Package `pkg/logger` implements a buffering client that writes through **`LocalLogStore`** (implemented by the host) and, when remote sending is enabled, uploads batches with gRPC **`IngestBatch`**. TLS is required toward the server when dialing: supply **`TLSCAPEM`** in **`Options`**, or set **`InsecureSkipVerify`** only for local development.

- **`DisableRemote`**: when `true`, the client **never** opens a gRPC connection or uploads; **`Log`** / **`Track`** still append locally (unsent rows remain in the store until you reconfigure or use a different client).
- **`RemoteURL`**: optional; when non-empty, parsed as `host:port` or `grpc://host:port` and used as the dial target **instead of** **`GRPCAddress`**.
- **`Init(*Client)`** does not take options — set **`Options`** on **`NewClient`** before **`Init`**.

### Constructing the client

1. Open or build a store (for example **`sqllogstore.New`** from `pkg/sqllogstore` with GORM).
2. Call **`logger.NewClient(store, opts)`** with non-nil **`store`** and **`Options`** (including **`ApplicationName`**; when remote is enabled, **`GRPCAddress`** or **`RemoteURL`**, plus TLS fields as above).
3. Use **`(*Client).Log`**, **`Track`**, **`Flush`**, **`SetAnalyticsEnabled`**, and **`Close`** on the returned value, **or** register it as the package default (below).

Details, defaults for sync thresholds, and store semantics are in **[spec.md](spec.md)** (Client logger).

### Package default (`Init`)

If the rest of the app should not thread a **`*Client`** pointer everywhere, the parent can register a single instance after construction:

- **`logger.Init(client)`** — sets the default (returns **`ErrAlreadyInitialized`** if one is already set; **`ErrNilClient`** if **`client`** is nil).
- **`logger.Log`**, **`logger.Track`**, **`logger.Flush`**, **`logger.SetAnalyticsEnabled`** — delegate to that client (return **`ErrNotInitialized`** if **`Init`** was never called or after **`Close`** cleared the default).
- **`logger.Default()`** — returns **`(*Client, true)`** when a default is active.
- **`logger.Close()`** — calls **`(*Client).Close`** on the default and clears it **only if shutdown succeeds**; on error the default remains so you can retry. A second **`Close`** with no active default returns **`ErrNotInitialized`**.

Package-level methods hold a **read lock** for the duration of each call so **`Close`** does not run concurrently with **`Log`** / **`Flush`** on the default client.

Example (package default):

```go
import (
	"context"
	"os"

	"github.com/joshuabednaz/go-logger/pkg/logger"
	"github.com/joshuabednaz/go-logger/pkg/sqllogstore"
)

// Assume db is an open *gorm.DB.

store, err := sqllogstore.New(db)
if err != nil { /* ... */ }

client, err := logger.NewClient(store, logger.Options{
	ApplicationName:    "my-app",
	GRPCAddress:        "localhost:7443",
	BearerToken:        os.Getenv("LOGGER_TOKEN"),
	InsecureSkipVerify: true, // dev only; production: set TLSCAPEM to your CA / server cert PEM
})
if err != nil { /* ... */ }

if err := logger.Init(client); err != nil { /* ... */ }
defer func() { _ = logger.Close() }()

ctx := context.Background()
_, _ = logger.Log(ctx, "info", "ready", nil)
```

## Code generation

```bash
go run github.com/bufbuild/buf/cmd/buf@latest lint
go run github.com/bufbuild/buf/cmd/buf@latest generate
```

## License

See [LICENSE](LICENSE).

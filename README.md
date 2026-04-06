# Go Logger

Multi-tenant **application / operational logs** and **analytics** with **gRPC** (TLS), **HTTPS JSON**, **MCP** (stdio + streamable HTTPS), a **Go client SDK** with a pluggable local queue, and a **TypeScript client** for the JSON API.

- **Specification:** [spec.md](spec.md)
- **Build plan:** [implementation-checklist.md](implementation-checklist.md)
- **HTTPS contract:** [docs/openapi.yaml](docs/openapi.yaml)

## Module path

This checkout is **[Bednaz98/Go-Logger](https://github.com/Bednaz98/Go-Logger)**. The Go module path is still **`github.com/joshuabednaz/go-logger`** (forks may keep it or rename via `go.mod`, `buf.gen.yaml` **`go_package_prefix`**, and `buf generate`).

## Run the server

```bash
export LOGGER_AUTH_TOKEN="dev-token"          # optional; empty disables bearer check (dev)
export DATABASE_URL="file:logger.db?cache=shared"   # or postgres://...
go run ./cmd/server
```

Defaults: gRPC **TLS** on `0.0.0.0:5000`, HTTPS on `0.0.0.0:5001`, MCP HTTPS on `0.0.0.0:5002`, optional plain HTTP JSON on `5003` when `HTTP_PLAIN_LISTEN=true`. Without `TLS_*` env vars the server generates a **self-signed** cert and logs a **SHA-256 fingerprint**.

### Docker images

The **`Dockerfile`** has two targets; CI publishes both to GHCR:

| Image | Target | Contents |
| ----- | ------ | -------- |
| **`ghcr.io/<owner>/<repo>-server`** | `server` | gRPC + HTTPS + optional plain HTTP JSON API + MCP streamable HTTPS |
| **`ghcr.io/<owner>/<repo>-mcp`** | `mcp` | stdio MCP only (smaller) |

Local build: `docker build --target server -t logger-server .` or `--target mcp -t logger-mcp .`

The **`server`** image sets **`GRPC_PORT`**, **`HTTP_PORT`**, **`MCP_HTTP_PORT`**, and **`HTTP_PLAIN_PORT`** from Docker **`ARG`** defaults (**5000–5003**). Override at build time (`--build-arg GRPC_PORT=6000`, …) or at run time (`-e GRPC_PORT=6000`, …); map host ports to match.

```bash
# API server
docker run --rm -p 5000:5000 -p 5001:5001 -p 5002:5002 \
  -e DATABASE_URL=file:/data/logger.db?cache=shared \
  -v logger-data:/data ghcr.io/bednaz98/go-logger-server:latest

# Same server with cleartext JSON on 5003 (e.g. curl without -k)
docker run --rm -p 5000:5000 -p 5001:5001 -p 5002:5002 -p 5003:5003 \
  -e HTTP_PLAIN_LISTEN=true \
  -e DATABASE_URL=file:/data/logger.db?cache=shared \
  -v logger-data:/data ghcr.io/bednaz98/go-logger-server:latest

# MCP stdio (needs -i)
docker run --rm -i \
  -e DATABASE_URL=file:/data/logger.db?cache=shared \
  -v logger-data:/data ghcr.io/bednaz98/go-logger-mcp:latest
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
| `GRPC_PORT` | `5000` | gRPC over TLS (must be **1–65535**; invalid values **fail startup**) |
| `HTTP_PORT` | `5001` | HTTPS JSON API (`/api/v1/...`); same port rules |
| `HTTP_PLAIN_LISTEN` | `false` | If `true`, serve the **same** JSON routes on cleartext HTTP (see `HTTP_PLAIN_PORT`); use behind a reverse proxy or trusted networks only |
| `HTTP_PLAIN_PORT` | `5003` | Plain HTTP listener; must differ from `HTTP_PORT` and `MCP_HTTP_PORT`; same port rules |
| `MCP_HTTP_PORT` | `5002` | MCP streamable HTTP (disable with `MCP_HTTP_LISTEN=false`); same port rules |
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
| `MCP_REMOTE_BEARER_TOKEN` | falls back to `LOGGER_AUTH_TOKEN` | gRPC **authorization** metadata toward the **remote**; set explicitly in production so it is not tied to who may call **this** server |
| `MCP_REMOTE_TLS_CA_PATH` | *(empty)* | PEM file for remote server trust (required unless insecure) |
| `MCP_REMOTE_INSECURE_SKIP_VERIFY` | `false` | Dev-only TLS skip (mutually exclusive with proper CA in production) |
| `MCP_REMOTE_STRICT` | `false` | If `true`, bad remote TLS/address config **exits the process**; if `false`, MCP still runs with local **`ingest_batch`** only |

`grpc://` and `grpcs://` in addresses are **parsed to host:port** only; the forward client **always uses TLS** (CA path or insecure flag), not plaintext gRPC.

Applies to **`cmd/mcp`** (stdio) and streamable **MCP HTTPS** on the main server when `MCP_HTTP_LISTEN=true`.

## curl (HTTPS)

Health is unauthenticated; other routes require `Authorization: Bearer <token>`.

With **`HTTP_PLAIN_LISTEN=true`**, the same paths are available on **`http://localhost:5003`** (default **`HTTP_PLAIN_PORT`**) without TLS.

```bash
curl -sk https://localhost:5001/api/v1/health
curl -sk https://localhost:5001/api/v1/ingest/batch \
  -H "Authorization: Bearer dev-token" -H "Content-Type: application/json" \
  -d '{"application_name":"demo","records":[{"log_id":"1","record_kind":"operational","application_name":"demo","log_message":"hello","event_timestamp":"2026-03-27T12:00:00Z","log_level":"info"}]}'
```

## grpcurl

```bash
grpcurl -insecure -H "authorization: Bearer dev-token" \
  -d '{"application_name":"demo","records":[{"log_id":"2","record_kind":"RECORD_KIND_OPERATIONAL","application_name":"demo","log_message":"hi","event_timestamp":"2026-03-27T12:00:00Z","log_level":"LOG_LEVEL_INFO"}]}' \
  localhost:5000 logger.v1.LoggerService/IngestBatch
```

## MCP (stdio)

The `cmd/mcp` binary speaks MCP over stdin/stdout and opens the database from `DATABASE_URL` (same schema as the server). Tools include **`ingest_batch`** (local DB, optional remote forward — see **MCP remote forward** above), **`query_logs`**, **`get_log_by_id`**, etc.

Example Cursor snippet: [docs/mcp-cursor-example.json](docs/mcp-cursor-example.json).

## TypeScript client (Node / browser)

Package **[`clients/ts`](clients/ts)** — **`@bednaz98/go-logger-client`** — talks to the same **`/api/v1`** JSON API as curl (ingest, query, delete, health). CI **publishes to GitHub Packages** on every successful **`main`** push (alongside Docker images); version is **`0.1.0-main.<run_id>.<run_attempt>`** (unique per workflow run / retry). **Maintainers:** CI publishes with **`NODE_AUTH_TOKEN`**: if repository secret **`PUBLISH_TOKEN`** is set (non-empty PAT with **`write:packages`**, from **`bednaz98`**), that is used; otherwise it falls back to **`GITHUB_TOKEN`** with job permission **`packages: write`** (same-repo publish to GitHub Packages only). **`E401`** from npm usually means **`PUBLISH_TOKEN`** was empty, misnamed, expired, or not SSO-authorized — leave **`PUBLISH_TOKEN`** unset to use the default token, or fix the secret.

```typescript
import { LoggerClient } from '@bednaz98/go-logger-client';

const log = new LoggerClient({
  baseUrl: 'https://your-host:5001',
  token: process.env.LOGGER_TOKEN,
});
await log.log({ applicationName: 'app', message: 'hello', level: 'info' });
```

Install and `.npmrc` setup: see **[clients/ts/README.md](clients/ts/README.md)**.

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
	GRPCAddress:        "localhost:5000",
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

# Changelog

## Unreleased

- **Go client SDK:** **`NewServerClient`**: sends each **`Log`** / **`Track`** via gRPC **`IngestBatch`** immediately with **no `LocalLogStore`** (backend services). On RPC failure, **`Log`** / **`Track`** return **`("", err)`** (no log id). **`NewDeviceClient`** is the explicit name for the buffering client; **`NewClient`** remains an alias. **`Init`** / package-level APIs apply only to **`*Client`** (device). **`ErrServerClientDisableRemote`** if **`NewServerClient`** is called with **`DisableRemote: true`**.
- **Breaking — server defaults:** Default listen ports are now **5000** (gRPC), **5001** (HTTPS JSON), **5002** (MCP HTTPS), **5003** (plain HTTP JSON when `HTTP_PLAIN_LISTEN=true`). Override with **`GRPC_PORT`**, **`HTTP_PORT`**, **`MCP_HTTP_PORT`**, **`HTTP_PLAIN_PORT`**, or Docker **`server`** image **`ARG`** / **`ENV`**.
- **Config:** **`GRPC_PORT`**, **`HTTP_PORT`**, **`MCP_HTTP_PORT`**, and **`HTTP_PLAIN_PORT`** must be integers **1–65535** when set; invalid or out-of-range values **fail startup** (no silent fallback).
- **Security:** Bearer token checks for gRPC, HTTPS JSON, and MCP streamable HTTP use **constant-time** comparison when lengths match.
- **Compose:** **`deploy/docker-compose.yml`** no longer publishes **5003** or mounts **`/data`** by default (Postgres DSN only); enable **`HTTP_PLAIN_LISTEN`** and add **`5003:5003`** if you need plain JSON locally.
- **Go client SDK:** optional package default via `logger.Init(*Client)` with package-level `Log`, `Track`, `Flush`, `SetAnalyticsEnabled`, and `Close`; `Default()` for explicit access; exported errors `ErrNotInitialized`, `ErrAlreadyInitialized`, and `ErrNilClient`. The parent still constructs the client with `NewClient`; `Close` clears the default only after a successful `(*Client).Close`.
- **Go client SDK:** `Options.DisableRemote` (no gRPC / no upload), `Options.RemoteURL` (`host:port` or `grpc://…`), and `ErrNoRemoteTarget` when remote is enabled but no target is set.
- **MCP:** `ingest_batch` tool; optional forward to a remote LoggerService via `MCP_REMOTE_GRPC_ADDRESS`, `MCP_REMOTE_SENDING`, `MCP_REMOTE_BEARER_TOKEN`, `MCP_REMOTE_TLS_CA_PATH`, `MCP_REMOTE_INSECURE_SKIP_VERIFY` (stdio MCP and main server MCP HTTPS). Invalid remote config logs a warning and disables forward unless `MCP_REMOTE_STRICT=true`.
- **Go client:** empty `GRPCAddress` and `RemoteURL` with remote enabled now fail fast with `ErrNoRemoteTarget` (clearer than TLS dial errors).
- **Package logger:** `ErrNotInitialized` message is now `logger: no active default client` (still use `errors.Is`).
- **Docker / CI:** Multi-target `Dockerfile` (`server`, `mcp`); GitHub Actions publishes **`ghcr.io/<owner>/<repo>-server`** and **`ghcr.io/<owner>/<repo>-mcp`** (no single combined image).
- **Server:** Optional cleartext HTTP listener for the same **`/api/v1/*`** JSON API as HTTPS: **`HTTP_PLAIN_LISTEN`** (default `false`), **`HTTP_PLAIN_PORT`** (default `5003`); must not collide with **`HTTP_PORT`** or **`MCP_HTTP_PORT`**.
- **TypeScript:** Package **`@bednaz98/go-logger-client`** in **`clients/ts`** (HTTPS JSON API); scope matches GitHub owner **[Bednaz98/Go-Logger](https://github.com/Bednaz98/Go-Logger)** for GitHub Packages. CI **`npm-client`** publishes on **`main`** with versions **`0.1.0-main.<run_id>.<run_attempt>`**.

## 0.1.0 — 2026-03-27

Initial implementation: protobuf API, GORM store (SQLite/Postgres), TLS resolution with optional auto-generated certificates, gRPC `LoggerService`, HTTPS JSON API under `/api/v1`, MCP tools over stdio and streamable HTTPS, and Go client SDK with reference `sqllogstore`.

# Changelog

## Unreleased

- **Breaking — server defaults:** Default listen ports are now **5000** (gRPC), **5001** (HTTPS JSON), **5002** (MCP HTTPS), **5003** (plain HTTP JSON when `HTTP_PLAIN_LISTEN=true`). Override with **`GRPC_PORT`**, **`HTTP_PORT`**, **`MCP_HTTP_PORT`**, **`HTTP_PLAIN_PORT`**, or Docker **`server`** image **`ARG`** / **`ENV`**.
- **Go client SDK:** optional package default via `logger.Init(*Client)` with package-level `Log`, `Track`, `Flush`, `SetAnalyticsEnabled`, and `Close`; `Default()` for explicit access; exported errors `ErrNotInitialized`, `ErrAlreadyInitialized`, and `ErrNilClient`. The parent still constructs the client with `NewClient`; `Close` clears the default only after a successful `(*Client).Close`.
- **Go client SDK:** `Options.DisableRemote` (no gRPC / no upload), `Options.RemoteURL` (`host:port` or `grpc://…`), and `ErrNoRemoteTarget` when remote is enabled but no target is set.
- **MCP:** `ingest_batch` tool; optional forward to a remote LoggerService via `MCP_REMOTE_GRPC_ADDRESS`, `MCP_REMOTE_SENDING`, `MCP_REMOTE_BEARER_TOKEN`, `MCP_REMOTE_TLS_CA_PATH`, `MCP_REMOTE_INSECURE_SKIP_VERIFY` (stdio MCP and main server MCP HTTPS). Invalid remote config logs a warning and disables forward unless `MCP_REMOTE_STRICT=true`.
- **Go client:** empty `GRPCAddress` and `RemoteURL` with remote enabled now fail fast with `ErrNoRemoteTarget` (clearer than TLS dial errors).
- **Package logger:** `ErrNotInitialized` message is now `logger: no active default client` (still use `errors.Is`).
- **Docker / CI:** Multi-target `Dockerfile` (`server`, `mcp`); GitHub Actions publishes **`ghcr.io/<owner>/<repo>-server`** and **`ghcr.io/<owner>/<repo>-mcp`** (no single combined image).
- **Server:** Optional cleartext HTTP listener for the same **`/api/v1/*`** JSON API as HTTPS: **`HTTP_PLAIN_LISTEN`** (default `false`), **`HTTP_PLAIN_PORT`** (default `5003`); must not collide with **`HTTP_PORT`** or **`MCP_HTTP_PORT`**.
- **TypeScript:** New package **`@joshuabednaz/go-logger-client`** in **`clients/ts`** (HTTPS JSON API). CI job **`npm-client`** publishes to **GitHub Packages** on **`main`** with prerelease versions **`0.1.0-main.<run_id>.<run_attempt>`**.

## 0.1.0 — 2026-03-27

Initial implementation: protobuf API, GORM store (SQLite/Postgres), TLS resolution with optional auto-generated certificates, gRPC `LoggerService`, HTTPS JSON API under `/api/v1`, MCP tools over stdio and streamable HTTPS, and Go client SDK with reference `sqllogstore`.

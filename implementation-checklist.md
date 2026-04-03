# Go Logger — implementation checklist

Phased build checklist with **concrete files** (paths relative to repo root). Replace **`github.com/example/go-logger`** with your real **Go module path** when you run `go mod init`.

**Specification:** [spec.md](spec.md)

---

## Phase 0 — Repository and layout

**Goal:** Runnable skeleton, empty commands, CI hook for fmt/vet/test.

### Files to create

| File / directory | Purpose |
| ---------------- | ------- |
| `go.mod` | Module definition (`go 1.22+` or team standard). |
| `.gitignore` | Add `/bin/`, `*.db`, `dist/`, `.env`, generated `gen/` if used. |
| `README.md` | How to run (placeholder), link to `spec.md` and this checklist. |
| `Makefile` *(optional)* | Targets: `proto`, `generate`, `test`, `lint`, `run-server`. |
| `buf.yaml` *(optional)* | Buf module root. |
| `buf.gen.yaml` *(optional)* | `protoc-gen-go`, `protoc-gen-go-grpc` → e.g. `gen/go/...`. |
| `.github/workflows/ci.yml` *(optional)* | `go test ./...`, `buf lint` / `buf breaking` when protos exist. |
| `cmd/server/main.go` | `main` that parses flags/env and exits 0 (stub). |
| `cmd/mcp/main.go` | Stub **stdio** MCP process (or single `cmd/logger` with subcommands—pick one). |
| `internal/config/config.go` | Struct for env/flags (empty struct OK initially). |

### Steps

- [ ] Run `go mod init github.com/example/go-logger` (or your path).
- [ ] Create directories: `cmd/server`, `cmd/mcp`, `internal/config`, `api/logger/v1` (for protos next phase).
- [ ] Add stub `main` files that compile.
- [ ] Add minimal `README.md` (build/run placeholders).
- [ ] *(Optional)* Add `Makefile` + `buf.yaml` / `buf.gen.yaml` scaffolds.
- [ ] *(Optional)* Add GitHub Actions workflow: checkout, setup-go, `go test ./...`.

---

## Phase 1 — API contract (proto + OpenAPI)

**Goal:** Versioned **gRPC** contract and **OpenAPI** draft for HTTPS parity.

### Files to create

| File | Purpose |
| ---- | ------- |
| `api/logger/v1/logger.proto` | `package logger.v1`; `LoggerService`; messages per **spec.md**. |
| `gen/go/logger/v1/*.pb.go` *(or `internal/gen/...`)* | **Generated** — do not hand-edit; path matches `buf.gen.yaml`. |
| `docs/openapi.yaml` | OpenAPI 3: paths under `/api/v1/`, schemas aligned with proto field names. |

### Steps

- [ ] Write `logger.proto`: `LoggerService` with `IngestBatch`, `ListLogs`, `DeleteLogs`; `LogRecord`; `IngestBatchRequest/Response` (`application_name`, `accepted_count`, optional `batch_id`); `ListLogsRequest/Response` (or stream); `DeleteLogsRequest/Response`; enums `RecordKind`, `LogLevel`.
- [ ] Add proto **comments** for multi-tenant rules and idempotency.
- [ ] Configure **Buf** (or `protoc`) and generate Go packages.
- [ ] Add **`buf.lock`** / `buf.yaml` **lint** and **`buf breaking`** against `main` in CI.
- [ ] Create `docs/openapi.yaml`: `POST /ingest/batch`, `GET /logs`, `POST /logs/query`, `DELETE /logs`, `GET /health`; require `application_name` where spec says so.
- [ ] Add CI step: regenerate check (`buf generate` + `git diff --exit-code`) if you commit gen output.

---

## Phase 2 — Server persistence (GORM + repository)

**Goal:** DB schema, **repository** used later by gRPC/HTTP/MCP.

### Files to create

| File | Purpose |
| ---- | ------- |
| `internal/store/model.go` | GORM `Log` model: tenant + `log_id` + payload columns + indexes. |
| `internal/store/db.go` | Open GORM dialector (**SQLite** / **Postgres**) from DSN; `AutoMigrate` or migration runner. |
| `internal/store/repository.go` | `IngestBatch` (tx), `ListLogs`, `DeleteLogs`, `ListApplicationNames` (distinct). |
| `internal/store/errors.go` *(optional)* | Sentinel errors for repo layer. |
| `internal/domain/filter.go` *(optional)* | Shared **query filter** struct for list/delete (used by gRPC + HTTP + MCP). |

### Steps

- [ ] Define model: unique **`(application_name, log_id)`**; index **`(application_name, event_timestamp)`** (or stored time); session index if needed.
- [ ] Implement `OpenDB(cfg) (*gorm.DB, error)` for SQLite file DSN and Postgres URL.
- [ ] Implement `Repository` methods with **no** HTTP/gRPC imports (pure store).
- [ ] Unit-test repo with **SQLite in-memory** (and optionally Postgres integration test).
- [ ] Implement **`ListApplicationNames`** for MCP `list_applications`.

---

## Phase 3 — TLS and listener config

**Goal:** Load cert **path → PEM string → auto-gen**; expose listen **address + ports**.

### Files to create

| File | Purpose |
| ---- | ------- |
| `internal/tlsconfig/tlsconfig.go` | Resolve cert/key; auto-generate self-signed; log fingerprint. |
| `internal/config/server.go` *(or extend `config.go`)* | `TLS_CERT_PATH`, `TLS_CERT_PEM`, `TLS_KEY_*`, `LISTEN_BIND_ADDRESS`, `GRPC_PORT`, `HTTP_PORT`, `MCP_HTTP_PORT`, `TLS_MUST_USE_PROVIDED_CERT`. |

### Steps

- [ ] Implement path-first then PEM for cert and key independently; error if only one of cert/key; auto-gen if both missing (unless `TLS_MUST_USE_PROVIDED_CERT`).
- [ ] Load **same** `tls.Certificate` for gRPC + HTTPS servers.
- [ ] Unit tests: provided PEM, provided files, auto-gen path, partial config fails.
- [ ] Document env vars in `README.md`.

---

## Phase 4 — gRPC server (ingest only)

**Goal:** **`IngestBatch`** end-to-end with **auth**, **tenant checks**, **all-or-nothing** tx.

### Files to create

| File | Purpose |
| ---- | ------- |
| `internal/server/grpc/server.go` | `grpc.NewServer`, TLS creds, register `LoggerService`. |
| `internal/server/grpc/logger_service.go` | `IngestBatch` implementation → `Repository`. |
| `internal/server/grpc/auth.go` | Unary interceptor: validate `authorization` **Bearer** metadata. |
| `internal/server/grpc/validate.go` *(optional)* | Batch: duplicate `log_id` in request, `application_name` match per record. |

### Steps

- [ ] Wire `RegisterLoggerServiceServer` with implementation calling `Repository.IngestBatch` in **one transaction**.
- [ ] Validate: missing/empty `application_name`; mismatched record tenant; duplicate IDs in batch → `InvalidArgument`.
- [ ] Enforce optional **metadata JSON max size** (256 KiB default when enabled).
- [ ] Return `OK` + `accepted_count` for idempotent duplicate **log_id** across requests; **0** if all duplicates.
- [ ] Integration test: `bufconn` or real port; success, rollback, idempotent retry.
- [ ] Update `cmd/server/main.go`: load config, DB, TLS, start gRPC on **`LISTEN:7443`**.

---

## Phase 5 — Go client SDK

**Goal:** **`LocalLogStore`**, reference SQLite store, **sync loop**, gRPC ingest.

### Files to create

| File | Purpose |
| ---- | ------- |
| `pkg/logger/types.go` | `LocalRecord` struct (fields per spec). |
| `pkg/logger/local_store.go` | `LocalLogStore` **interface** definition. |
| `pkg/logger/client.go` | `Client` with `NewClient`, `Log`, `Track`, `Flush`, `Close`, `Options`. |
| `pkg/logger/default_client.go` | Optional package default: `Init`, `Default`, package-level `Log` / `Track` / `Flush` / `SetAnalyticsEnabled` / `Close`. |
| `pkg/logger/errors.go` | Sentinel errors (`ErrNilClient`, etc.). |
| `pkg/logger/sync.go` | Background loop: `CountUnsent`, `OldestUnsentAge`, `ListUnsent`, upload, `MarkSent`, `DeleteSyncedOlderThan`. |
| `pkg/logger/grpc_transport.go` | Build `IngestBatchRequest`, call gRPC, handle TLS for dev. |
| `internal/grpcutil/target.go` | Shared `ParseDialTarget` for `host:port` / `grpc://` / `grpcs://` (SDK + MCP remote). |
| `pkg/sqllogstore/store.go` | Reference **`LocalLogStore`**: payload + outbox (`queued_at`, `server_acked_at`). |
| `pkg/logger/*_test.go` | Tests: fake store, `Init` / default client lifecycle, optional mock gRPC server. |

### Steps

- [ ] Define `LocalRecord` + `LocalLogStore` exactly as **spec.md** (method semantics).
- [ ] Implement `sqllogstore`: migrations/schema for payload + sync columns; **idempotent** `Append` on `log_id` where possible.
- [ ] Implement sync thresholds (defaults from spec): max records per upload, poll interval, count + age triggers, purge after `MarkSent` + on tick.
- [ ] gRPC client: Bearer metadata, TLS trust bundle or insecure dev flag.
- [ ] Document SDK usage in `README.md` (Wails: call from Go only).

---

## Phase 6 — gRPC server (ListLogs + DeleteLogs)

**Goal:** Read/delete paths on gRPC matching **query filters** + **tenant**.

### Files to create / extend

| File | Purpose |
| ---- | ------- |
| `internal/server/grpc/logger_service.go` *(extend)* | Add `ListLogs`, `DeleteLogs`. |
| `internal/store/repository.go` *(extend)* | Query + delete with regex abstraction (SQLite vs Postgres). |

### Steps

- [ ] Implement `ListLogs`: require `application_name`; map filters to SQL; cursor or limit/offset.
- [ ] Implement `DeleteLogs`: require tenant; same filter subset as spec.
- [ ] Tests for cross-tenant isolation (cannot list other app’s logs with wrong name).
- [ ] Update `cmd/server` if new config knobs.

---

## Phase 7 — HTTPS JSON API

**Goal:** **REST** mirror of gRPC using **same repository**.

### Files to create

| File | Purpose |
| ---- | ------- |
| `internal/server/http/router.go` | Chi/Echo/std mux: mount `/api/v1`. |
| `internal/server/http/middleware.go` | Bearer auth, max body bytes, request ID *(optional)*. |
| `internal/server/http/handlers_ingest.go` | `POST /ingest/batch`. |
| `internal/server/http/handlers_logs.go` | `GET /logs`, `POST /logs/query`. |
| `internal/server/http/handlers_delete.go` | `DELETE /logs` JSON body. |
| `internal/server/http/handlers_health.go` | `GET /health`. |
| `internal/server/http/json.go` *(optional)* | Shared encode/decode + error body shape. |

### Steps

- [ ] Start HTTPS server on **8443** with shared TLS from Phase 3.
- [ ] Map JSON ↔ repo calls; same validation as gRPC (tenant, batch rules, metadata size).
- [ ] Contract tests: same inputs → same DB state as gRPC **or** shared handler layer.
- [ ] Keep `docs/openapi.yaml` in sync (or generate from handlers).

---

## Phase 8 — MCP (tools + stdio + HTTP)

**Goal:** Named tools, **direct GORM** (or repo), **stdio** + optional HTTP MCP.

### Files to create

| File | Purpose |
| ---- | ------- |
| `internal/mcp/tools.go` | Register `list_applications`, `get_log_by_id`, `query_logs`, `count_logs`, `delete_logs`. |
| `internal/mcp/schema.go` *(optional)* | JSON Schema for tool inputs (or inline in registration). |
| `internal/mcp/server_stdio.go` | MCP server over stdin/stdout. |
| `internal/mcp/server_http.go` | MCP streamable HTTP on **8444** (reuse TLS). |
| `cmd/mcp/main.go` *(replace stub)* | Load DSN from env, open DB, run stdio MCP. |
| `docs/mcp-cursor-example.json` *(optional)* | Sample Cursor MCP config snippet. |

### Steps

- [ ] Implement each tool: require `application_name` except `list_applications`; call **repository** or GORM (prefer **repository** to avoid drift).
- [ ] Gate `delete_logs` on **`MCP_ENABLE_DELETE_LOGS`** (default off).
- [ ] `stdio` binary: document `DATABASE_URL` / SQLite path for editors.
- [ ] HTTP MCP: bind port, Bearer auth aligned with server.
- [ ] Document tool list in `README.md`.

---

## Phase 9 — Hardening and release

**Goal:** Ops-ready defaults, docs, packaging.

### Files to create / extend

| File | Purpose |
| ---- | ------- |
| `internal/observability/log.go` *(optional)* | slog wiring, gRPC logging interceptor. |
| `Dockerfile` *(optional)* | Multi-stage build; expose 7443/8443/8444. |
| `deploy/docker-compose.yml` *(optional)* | Postgres + logger server. |
| `CHANGELOG.md` | Keep a changelog. |
| `CONTRIBUTING.md` *(optional)* | PR + proto breaking rules. |

### Steps

- [ ] Set **max gRPC recv/send** message size (e.g. 4 MiB); document tuning.
- [ ] Add **structured logs** for listen addresses, TLS mode (provided vs auto), DB dialect.
- [ ] Run **`go vet`**, **`staticcheck`**, **`golangci-lint`** *(optional)* in CI.
- [ ] Expand **README**: TLS env vars, multi-tenant examples, **grpcurl**/**curl** samples, MCP config.
- [ ] Tag **v0.1.0** (or team scheme); publish module if public.

---

## Quick reference — files introduced by phase

| Phase | New / primary paths |
| ----- | ------------------- |
| 0 | `go.mod`, `README.md`, `cmd/server/main.go`, `cmd/mcp/main.go`, `internal/config/config.go`, optional `Makefile`, `buf.yaml`, CI |
| 1 | `api/logger/v1/logger.proto`, generated `gen/...`, `docs/openapi.yaml` |
| 2 | `internal/store/model.go`, `db.go`, `repository.go`, optional `domain/filter.go` |
| 3 | `internal/tlsconfig/tlsconfig.go`, extend `internal/config` |
| 4 | `internal/server/grpc/*.go`, extend `cmd/server/main.go` |
| 5 | `pkg/logger/*.go`, `pkg/sqllogstore/store.go` |
| 6 | extend `logger_service.go`, `repository.go` |
| 7 | `internal/server/http/*.go`, extend `cmd/server/main.go` |
| 8 | `internal/mcp/*.go`, `cmd/mcp/main.go`, optional `docs/mcp-cursor-example.json` |
| 9 | optional `Dockerfile`, `deploy/*`, `CHANGELOG.md`, observability package |

Adjust names if you prefer a single `cmd/logger` with `server` / `mcp` **subcommands** instead of two `main` packages.

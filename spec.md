# Go Logger — Specification

---

## Scope

> One system for **application / operational logs** and **user behavior analytics**. The **server is multi-tenant**: many **applications** share one deployment; **tenant isolation** is by **`application_name`**. Clients use **gRPC** (Go SDK) and/or **HTTPS JSON** (other languages). Industry practice (including [OpenTelemetry](https://opentelemetry.io/docs/specification/otel/logs/event-api/)) is to keep one pipeline and correlate signals with shared IDs (session, trace) instead of running two separate products.

---

## Multi-tenancy

| Rule | Decision |
| ----- | -------- |
| **Tenant key** | **`application_name`** — stable string per product or service (e.g. `billing-api`, `desktop-app`). **Required** on every stored record. |
| **Ingest batch** | **`IngestBatchRequest` includes `application_name`** once; **every `LogRecord` in that batch must match** the same value. **Mismatch** → **`InvalidArgument`** (no DB write). Empty/missing → **`InvalidArgument`**. |
| **Query & delete** | **gRPC**, **HTTPS**, and **MCP** must scope by **`application_name`** (required filter). **No** cross-tenant list/delete by default (prevents accidental data bleed). |
| **Storage** | Index **`(application_name, …)`** for common query patterns (time range, session). |
| **Auth vs tenant** | **Bearer token** proves caller identity; **`application_name`** proves which tenant’s data is accessed. Deployments may issue **per-app tokens** or **shared ops tokens**—either way the **API still requires** an explicit **`application_name`** on ingest and on read/delete unless you add a separate **admin** mode later. |

---

## Architecture

The project has **three parts**:

| Part            | Role                                      |
| --------------- | ----------------------------------------- |
| **Data model**  | Unified record shape for logs + analytics |
| **Client logger** | SDK embedded in applications; **local queue** via host **`LocalLogStore`** |
| **Server**      | **Multi-tenant** **gRPC** + **HTTPS JSON** (**TLS** per **TLS and certificates**); **MCP** (**stdio** + **HTTP**, [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk)); **MCP** direct **GORM**; persistence via **GORM** |

---

## RPC transport (gRPC)

Client ↔ server communication uses **[gRPC](https://grpc.io/)** over **HTTP/2** with **[Protocol Buffers](https://protobuf.dev/)** as the **single source of truth** for request/response shapes.

| Topic | Decision |
| ----- | -------- |
| **Go stack** | [`google.golang.org/grpc`](https://pkg.go.dev/google.golang.org/grpc) + `protoc-gen-go` + `protoc-gen-go-grpc` (or **[Buf](https://buf.build/)** for lint/breaking checks and codegen) |
| **Address** | Configurable **host:port** (no path); **TLS** always on the wire (see **TLS and certificates**) |
| **Auth** | **Bearer token** (or API key) passed via **gRPC metadata** (e.g. `authorization`); same token concept as today’s “endpoint and token” init |
| **Payload** | **Protobuf messages** mirror the **data model** (including **log id**, **record kind**). **Metadata** on the wire: **`bytes`** holding **UTF-8 JSON** (keeps parity with **HTTPS JSON** and is simple to validate); **server default max metadata size** when enforcement is enabled: **256 KiB** per record (see **Server limits**). |
| **Compression** | Optional **gzip** (or other registered compressor) on the gRPC channel for large batches |
| **Errors** | Use **standard gRPC status codes** (`InvalidArgument`, `ResourceExhausted` for metadata size, etc.). **Duplicate `log id` (cross-request / retry):** respond with **`OK`** and the normal **`IngestBatchResponse`** (**idempotent**—client retries must not fail). **Duplicate `log id` within the same request batch:** reject with **`InvalidArgument`** before touching the DB (malformed client). **Batch is only duplicates of already-stored rows:** **`OK`** with **`accepted_count`** reflecting **new** inserts (**0** if all were idempotent no-ops). |
| **Batch ingest** | **All-or-nothing:** one **DB transaction** per batch; on failure **roll back** and return **error**—client **must not** **`MarkSent`**. |
| **Service sketch** | **`logger.v1.LoggerService`**: **`IngestBatch`**, **`ListLogs`** (paginated or server-stream), **`DeleteLogs`**. Exact names in `.proto`; keep stable once shipped. |
| **`IngestBatchResponse`** | Minimal: e.g. **`accepted_count`** (uint32) = rows **newly** inserted this commit; idempotent duplicates do not increment. Optional **`batch_id`** (string) for support correlation. |

**Why gRPC here:** smaller payloads than JSON, generated typed clients for **pure Go** and **Wails** (Go side only dials out), HTTP/2 multiplexing, and a clear **contract** via `.proto` files versioned under `api/` (e.g. `api/v1/logger.proto`).

Administrative **query** and **delete** operations are exposed as **gRPC RPCs** and mirrored on the **HTTPS JSON API** (see below) unless explicitly deferred to a later milestone (see plan below).

---

## HTTPS API (JSON over TLS)

A **REST-style HTTPS** surface lets **non-Go** clients (Node, Python, curl, browser `fetch` behind CORS policy, etc.) use the logger **without** the Go SDK. It is **additive**: **gRPC** remains the **contract source** and preferred path for the **official Go client**.

| Topic | Decision |
| ----- | -------- |
| **Protocol** | **HTTPS** (`HTTP/1.1` or **HTTP/2**); **JSON** request and response bodies unless noted |
| **Paths** | Under **`/api/v1`**: **`POST /ingest/batch`** (body includes **`application_name`** + records), **`GET /logs?application_name=…`** (**`application_name` required**), **`POST /logs/query`** (JSON body includes **`application_name`** + filters), **`DELETE /logs`** with **JSON body** only (criteria + **`application_name`**), **`GET /health`**—exact schemas in **OpenAPI** |
| **Contract parity** | JSON field names and semantics **match** the **Protobuf** messages (same **data model**); avoid divergent DTOs—generate **OpenAPI** from `.proto` (**grpc-gateway**, **buf** `grpc-ecosystem/grpc-gateway`, or hand-maintained spec kept in sync) where feasible |
| **Auth** | **`Authorization: Bearer <token>`** (same tokens as **gRPC** metadata); optional **mTLS** at the edge |
| **Compression** | Support **`Accept-Encoding` / `Content-Encoding`** (**gzip**, **`zstd`** if middleware available) for large batches |
| **Errors** | JSON problem object or stable `{ "code", "message", "details" }` mapping from **gRPC status** / domain errors; use appropriate **HTTP status** (400, 401, 413 for oversize metadata, etc.). **Duplicate `log id` on ingest** maps to **2xx** with the same body shape as success (**idempotent**), matching gRPC. **Batch ingest** is **all-or-nothing** (same transactional rule as gRPC). |
| **Implementation** | **`net/http`** or a light router (**Chi**, **Echo**, etc.); handlers call the **same repository/store** as **gRPC** (no duplicate business logic) |
| **TypeScript clients** | Optional **npm** package (**`@bednaz98/go-logger-client`**, source under **`clients/ts`**) using **`fetch`** against **`/api/v1`**; published to **GitHub Packages** alongside container images from CI |

**Listener layout:** Run **gRPC** and **HTTPS JSON** on **separate ports** by default; see **Default listen ports** under **Server**. Reverse-proxy termination is supported; document upstream ports in config.

**Optional cleartext HTTP (same routes):** When enabled via config (e.g. **`HTTP_PLAIN_LISTEN=true`**), the process may expose a **second** listener on a distinct port with the **same** **`/api/v1/*`** handler stack (**Bearer** auth and limits unchanged) over **HTTP** (no TLS). Default **off**; intended for **reverse proxies** that terminate TLS upstream or **trusted local** integrations. Operators must not expose this listener to untrusted networks without an edge proxy.

---

## TLS and certificates

Certificate and key may each be supplied as a **local file path** and/or an **inline PEM string**. For **each** of cert and key, the server picks **one** source using **fixed precedence**.

**Precedence (cert and key independently)**

| Artifact | Order (first match wins) |
| -------- | ------------------------ |
| **Certificate** | **1.** If **`TLS_CERT_PATH`** is set and non-empty → load PEM from **file**. **2.** Else if **`TLS_CERT_PEM`** is non-empty → use **inline string**. **3.** Else → **no** operator cert. |
| **Private key** | **1.** If **`TLS_KEY_PATH`** is set and non-empty → load from **file**. **2.** Else if **`TLS_KEY_PEM`** is non-empty → use **inline string**. **3.** Else → **no** operator key. |

If **both** path and PEM are set for the same artifact, **the file path is used** and the PEM string is **ignored** for that artifact.

**Startup resolution**

1. If **both** an effective **cert** and an effective **key** are resolved (each by the table above), load them and use them for **gRPC**, **HTTPS JSON**, and **MCP HTTP** (when TLS is in-process).
2. If **exactly one** of cert or key is resolved, **fail startup** (do not fall back to auto-TLS).
3. If **neither** cert nor key is resolved (**both** artifacts missing after applying precedence), **generate a new self-signed certificate at startup** (in-memory or temp file—implementation choice) so listeners still use **TLS**.

**Shared material:** Use **one** loaded or generated key pair for **gRPC** and **HTTPS** in the same process unless **per-listener** overrides are explicitly supported later.

**Self-signed (auto-generated) SANs:** Include **`localhost`**, **`127.0.0.1`**, **`::1`**, and optional **configurable hostnames/IPs**.

**Operator visibility:** When auto-TLS runs, log a **warning**, log the **leaf fingerprint** (SHA-256), and document **`grpcurl -insecure`**, **`curl -k`**, PEM export, or fingerprint pinning for clients.

**Production policy:** Recommend real CA / internal PKI certs via path or PEM. Optional **`TLS_MUST_USE_PROVIDED_CERT=true`**: **fail startup** if cert/key are not both supplied (disables auto-TLS).

**Client behavior:** Auto-generated certs are **not** in the system trust store; the **Go SDK** supports **custom trust** (PEM) or **dev-only** `InsecureSkipVerify` behind an explicit flag.

---

## gRPC adoption plan

Phased rollout so ingest works end-to-end before expanding surface area.

| Phase | Deliverable |
| ----- | ----------- |
| **1. API definition** | `.proto` `package logger.v1`: **`LoggerService`** with **`IngestBatch`**, **`ListLogs`**, **`DeleteLogs`**; **`IngestBatchRequest`** carries **`application_name`** + repeated **`LogRecord`** (each record also carries **`application_name`** and must match); **`IngestBatchResponse`** with **`accepted_count`** (+ optional **`batch_id`**); enums **record kind**, **log level**; **multi-tenancy** and **idempotency** rules in comments. |
| **2. Tooling** | Script or `buf.gen.yaml` to generate Go from protos; CI checks **breaking changes** on `api/` (Buf breaking or equivalent). |
| **3. Server — ingest** | Implement **`LoggerService.IngestBatch`** with **TLS**; validate **`application_name`** + **per-batch uniqueness of `log_id`** + **record/batch tenant match**; **GORM** **one transaction**; **idempotent** cross-retry **`log id`**; optional **metadata** cap; map errors to gRPC status. |
| **4. Client library** | Accept **`LocalLogStore`** in constructor; sync loop uses **`CountUnsent`**, **`OldestUnsentAge`**, **`ListUnsent`**, **`MarkSent`**. gRPC **client connection** (`grpc.NewClient` / `grpc.Dial` per library version), unary (or streaming) **Ingest**; retries with backoff on `Unavailable` / `DeadlineExceeded`; auth **metadata** from init. |
| **5. Server — read/write API** | Add RPCs for **query** (streaming or paginated `ListLogs`) and **delete** matching **query filters**. Implement persistence through a **shared store/repository** (recommended) so **gRPC**, **HTTPS JSON**, and **MCP** (where applicable) do not diverge. Register **MCP tools** per **MCP dual-transport plan** (**direct GORM**; reuse the **same repository/store** as gRPC where queries overlap). |
| **6. Hardening** | Channel compression toggle; max message size limits; observability (grpc stats/logging); document **grpcurl** / debugging for operators; document **TLS** trust for **auto-generated** certs. |

**Out of scope for gRPC-centric work:** **gRPC-Web** (optional later if a browser must speak gRPC natively). **Cross-language and browser-friendly access** is covered by the **HTTPS JSON API**; **Wails/React** should still prefer the **embedded Go client** over **gRPC** or **HTTPS** from the UI tier.

---

## HTTPS API adoption plan

Ship **HTTPS JSON** after (or alongside) the **shared repository** exists so handlers stay thin.

| Phase | Deliverable |
| ----- | ----------- |
| **H1. Spec** | **OpenAPI 3** document for **ingest**, **query**, **delete**, and **health**; JSON schemas aligned with **Protobuf** field names and **data model** enums. |
| **H2. Server wiring** | HTTPS listener (**TLS** shared with gRPC per **TLS and certificates**); route registration under `/api/v1/`; middleware for **Bearer** auth and request size limits (metadata max bytes). |
| **H3. Handlers** | Each handler calls **repository/store** methods used by **gRPC**—**no** parallel SQL in HTTP-only code. |
| **H4. Parity & docs** | Integration tests that assert **gRPC** and **HTTPS** produce the same persistence for the same inputs (where methods compare); publish **OpenAPI** for external integrators. |
| **H5. Optional** | **CORS** allowlist if browser clients are supported; rate limiting per token or IP. |

---

## MCP dual-transport plan

Support **both** MCP transports so local editors and remote/shared access work without duplicating tool logic.

| Phase | Deliverable |
| ----- | ----------- |
| **M1. Tool handlers + DB** | **MCP tools have direct access to the database** via **GORM** (open connection from the same **DSN / config** as the logger server). Implement tool logic with **shared GORM models** and, where practical, a **shared repository/store package** used by **gRPC** and **HTTPS JSON** handlers so queries, filters, and deletes stay consistent—avoid maintaining two independent SQL code paths. |
| **M2. stdio transport** | CLI entry (e.g. `logger mcp stdio` or dedicated subcommand): run **[Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk)** server over **stdin/stdout** for **Cursor / Claude Desktop / VS Code** style configs (`command` + `args` + `env`). The subprocess **must receive DB config** (e.g. `DATABASE_URL` or SQLite **file path**) so it can open GORM directly. Document sample host config JSON. |
| **M3. HTTP transport** | Expose MCP over **HTTPS** using the SDK’s **streamable HTTP** (or current recommended HTTP transport in the SDK version in use); bind **host:port** (or path behind reverse proxy); reuse server **TLS** material (**provided** or **auto-generated**) when MCP terminates TLS in-process. |
| **M4. Auth alignment** | **stdio**: trust **OS user** + optional **env** token/API key loaded at startup. **HTTP**: require **Bearer token** (or mTLS) on MCP HTTP endpoints; align rules with **gRPC** metadata auth where practical. |
| **M5. Operations** | Document when to use which transport; health/logging for HTTP MCP; note **one process** may run **gRPC + HTTP MCP** simultaneously; stdio mode is typically **separate process** spawned by the editor. For **file-backed SQLite**, document **WAL mode**, **busy timeout**, and that **stdio MCP + gRPC server** may both open the same file—operators must understand concurrent access limits. |

### MCP transports (stdio and HTTP)

| Transport | Typical use | Auth / trust | Notes |
| --------- | ----------- | ------------ | ----- |
| **stdio** | Local IDE spawns the binary; JSON-RPC over pipes | Parent process + optional env secret | Requires **DB DSN in environment** (or config file path)—**no gRPC hop** to read logs. |
| **HTTP** | Remote URL, multiple clients, gateway in front | **TLS** + **token** (or OAuth patterns per SDK) | Same **tool definitions** and handler code as stdio; only the **transport adapter** differs. Opens **GORM** with the same DSN as configured for this deployment. |

**Principle:** Tool **names**, **JSON Schema inputs**, and **handler behavior** are **identical** on both transports—only how bytes move (pipes vs HTTP) changes.

**MCP and database access:** MCP tools **do not** call the logger over **gRPC** or the **HTTPS JSON** API; they hit the **database directly** through **GORM**. **Default posture:** tools are **read/query + metadata** unless explicitly documented otherwise; any **delete or destructive** tool must be **named and gated** (config flag) so operators know the risk. The **gRPC** and **HTTPS** servers are for **log clients** and **general integrations** (ingest, query, delete); **MCP** is for **agent/operator** access with **direct DB** semantics.

### MCP tools (named list)

Tool names are **stable** (`snake_case`). Every tool that reads or deletes **log rows** **requires `application_name`** in its input—**except** **`list_applications`**, which is intentionally **cross-tenant** (operator directory). JSON Schemas ship with the server / docs.

| Tool name | Purpose | Destructive? |
| --------- | -------- | ------------ |
| **`list_applications`** | Returns **distinct `application_name`** values seen in storage (tenant directory for operators). | No |
| **`get_log_by_id`** | **`log_id`** + **`application_name`** → one record (or not found). | No |
| **`query_logs`** | **`application_name`** + filters aligned with **Query filters** (time range, session, level, kind, regex/pattern, pagination). Returns a **JSON array** of records (capped page size). | No |
| **`count_logs`** | **`application_name`** + optional filter subset → approximate or exact count (implementation choice; document limits). | No |
| **`delete_logs`** | Same criteria as **DeleteLogs** RPC / **HTTPS DELETE** body; **requires** config **`MCP_ENABLE_DELETE_LOGS=true`** (default **off**). | **Yes** |

**`delete_logs`** must **refuse** if the env/config flag is **false** and log a one-line reason.

---

## Data model

One **unified record** for every event. Analytics rows use the same fields as operational logs; `record kind` and naming conventions tell them apart.

| Field | Description |
| ----- | ----------- |
| **log id** | **Client-assigned** unique identifier for this record (e.g. UUID or ULID). Generated when the event is created; **the same value** is stored locally, sent in batches, and persisted on the server. Used by **frontend (host app)** and **backend** to refer to one specific log without ambiguity (support tickets, UI deep links, deduplicated retries). |
| **record kind** | `operational` (classic logs) or `analytics` (product / UX events)—same storage and ingest for both |
| **analytics event name** | When kind is `analytics`: stable string, e.g. `button_clicked`, `page_viewed`, `form_submitted` (primary label instead of free-form log message) |
| **Event types** (examples) | Legacy / UI-oriented names can map to `analytics` records: UI button click, UI form submission, UI page load, server route execution, database row entry |
| **user / actor id** | Optional: anonymous or authenticated id for analytics; omit or hash per privacy policy |
| **source** | `client-name`, `server-name` |
| **source environment** | `dev`, `prod`, or string |
| **sessionID** | Shared across operational and analytics for one visit |
| **application name** | **Tenant identifier** (required): which application owns this row; must match on **ingest**, **query**, and **delete** (see **Multi-tenancy**) |
| **log message** | Operational: required; analytics: optional detail alongside event name |
| **metadata** | JSON: analytics properties (e.g. `{ "button_id": "save" }`); operational: contextual fields. **Server:** optional **max serialized JSON size** per record; when enabled, default cap **256 KiB**; **reject** oversize records at ingest (**HTTP 413** / `ResourceExhausted`). |
| **event timestamp** | When the event occurred (**RFC 3339** / UTC recommended on the wire) |
| **log level** | Fixed set (aligned with proto enums): `trace`, `debug`, `info`, `warn`, `error`, `fatal`. **Operational:** any of these; **analytics:** default `info` |
| **trace id / span id** | Optional: OpenTelemetry-style correlation if the host provides them |

---

## Client logger

Included in a host program. The **parent application** supplies **local persistence** by implementing **`LocalLogStore`**; the SDK does not choose a database by itself (a **reference** `LocalLogStore` for SQLite may ship as a separate helper package).

### Parent-provided local store (`LocalLogStore`)

The host implements this **Go interface**. Semantics are fixed so batching, the sync loop, and retention stay consistent whether the parent uses SQLite, another DB, or files.

#### `LocalRecord` (payload)

**`LocalRecord`** is the SDK-defined struct (or generated type) holding the **log payload** fields required for persistence and upload—aligned with **data model** / **`LogRecord`** proto:

| Field (conceptual) | Notes |
| ------------------ | ----- |
| **LogID** | Client **log id** (string) |
| **RecordKind** | `operational` \| `analytics` |
| **AnalyticsEventName** | When analytics |
| **UserActorID** | Optional |
| **Source**, **SourceEnvironment** | As in data model |
| **SessionID** | As in data model |
| **ApplicationName** | **Required** — must match SDK init **tenant**; used on every upload batch |
| **LogMessage** | As in data model |
| **MetadataJSON** | Raw JSON bytes or string |
| **EventTimestamp** | `time.Time` |
| **LogLevel** | Enum string |
| **TraceID**, **SpanID** | Optional strings |

Exact names/types ship with the SDK; hosts map to their DB columns.

#### Join / outbox table (sync state)

The **reference client schema** uses a **join (outbox) pattern** so the host can tell **what reached the server successfully**:

- **Payload table:** stores **`LocalRecord`** fields (keyed by **`log_id`**).
- **Outbox / sync join table:** one row per **`log_id`** (or equivalent FK) with **`queued_at`** (set when the row is **successfully committed** as pending upload—typically end of **`Append`**) and **`server_acked_at`** (nullable `time.Time` until a batch containing this id **fully succeeds** on the server).

**`MarkSent`** is called by the SDK **only after** an **all-or-nothing** batch succeeds; the host sets **`server_acked_at`** to **`time.Now()`** (or equivalent) for **every** `log_id` in that batch. **`ListUnsent` / `CountUnsent` / `OldestUnsentAge`** consider only rows with **`server_acked_at` IS NULL** (or equivalent).

**`DeleteSyncedOlderThan(cutoff)`:** delete rows that are **server-acked** and where **`server_acked_at` < `cutoff`** (retention by **ack time**, not **event timestamp**—best practice so in-flight logs are not purged early).

Hosts may merge payload + state into one table with columns instead of a literal join table, as long as semantics match.

```go
// Requires standard library imports: "context", "time".

// LocalLogStore is implemented by the host. Implementations should be safe for concurrent use
// from multiple goroutines unless the SDK documents single-threaded access only.

type LocalLogStore interface {
	// Append persists new records as not-yet-synced (or equivalent).
	Append(ctx context.Context, records []LocalRecord) error

	// ListUnsent returns pending-upload records, oldest first, at most limit rows.
	ListUnsent(ctx context.Context, limit int) ([]LocalRecord, error)

	// MarkSent marks the given log ids as successfully acknowledged by the server.
	MarkSent(ctx context.Context, logIDs []string) error

	// CountUnsent returns how many records are still pending upload.
	CountUnsent(ctx context.Context) (int64, error)

	// OldestUnsentAge returns how long the oldest still-not-server-acked record has been
	// in the outbox (now - queued_at). queued_at is set when the row becomes pending upload.
	// If there are no unsent rows, ok is false (SDK skips age-based triggers).
	OldestUnsentAge(ctx context.Context) (age time.Duration, ok bool, err error)

	// DeleteSyncedOlderThan deletes local rows that are already synced and older than cutoff.
	DeleteSyncedOlderThan(ctx context.Context, cutoff time.Time) error
}
```

**Implementer notes**

- **`Append`:** Prefer **idempotency on `log id`** so SDK retries do not duplicate rows; otherwise return an error.
- **`MarkSent`:** Only called after the server accepts the **entire** batch (**all-or-nothing**); update **`server_acked_at`** (or equivalent) for **all** ids in that batch together.
- **Schema and migrations** are the **parent’s** responsibility; the repo may ship a **reference SQL** for SQLite.
- **Construction:** e.g. **`NewClient(store LocalLogStore, opts Options)`** — **`store` is required** (non-nil). The parent owns the **`*Client`** value.

### Initialization (optional parameters)

- **`LocalLogStore`** (required)
- **`application_name`** (tenant) for this process — included on every outbound batch and must match server expectations
- Environment
- Single instance per application
- Random session ID when initialized
- **Remote sending**: may be disabled (`DisableRemote` / local-only); when enabled, **gRPC server address** as **`host:port`** or optional **`RemoteURL`** (`grpc://host:port`); when matching **Default listen ports**, typical dev target is **`localhost:5000`**
- **Bearer token** (sent as gRPC **metadata** on each call) when remote is enabled
- **TLS**: when the server uses **auto-generated** certificates, the client must use a **custom trust** (server cert/CA PEM) or an **explicit dev-only** insecure flag—document both in SDK examples

### MCP ingest and optional remote forward

The **MCP** server (stdio **`cmd/mcp`** or streamable HTTPS on the main process) exposes **`ingest_batch`** with the same JSON shape as **HTTPS** **`/api/v1/ingest/batch`**. Records are written to the **local** database used by that MCP process. Optionally, environment variables may configure a **second** gRPC **LoggerService** target so the same batch is **forwarded** after local ingest succeeds; **`MCP_REMOTE_SENDING=false`** disables that forward while keeping local ingest.

### Package default (`Init`)

The SDK may expose a **single process-wide default** so call sites can use package-level **`Log` / `Track` / `Flush`** without carrying a **`*Client`**.

- The **parent** still builds the client with **`NewClient`**; it then calls **`Init(client)`** to register that instance (non-nil). **`Init`** must not replace an existing default without **`Close`** on the default first (second **`Init`** returns an **`ErrAlreadyInitialized`**-style error).
- **`Close`** on the package API shuts down the registered client and clears the default **only if** **`(*Client).Close`** succeeds; on failure the default remains for retry.
- Package-level operations that use the default should be **serialized with respect to `Close`** on that default (e.g. hold a read lock for the duration of each **`Log` / `Track` / `Flush` / `SetAnalyticsEnabled`** so **`Close`** cannot run concurrently with them).

Exported sentinel errors should include at least: no active default (**`ErrNotInitialized`**), duplicate **`Init`** (**`ErrAlreadyInitialized`**), **`Init(nil)`** (**`ErrNilClient`**).

### API and behavior

- **Transport**: batches are uploaded with the **generated gRPC client** (Go); React/Wails UIs do not speak gRPC—only the **embedded Go logger** does
- **One client API** for both: e.g. `Log(...)` for operational records and `Track(eventName, properties)` (or `Record(kind, ...)`)—same local queue and batching rules
- Each new record gets a **log id** at creation time; APIs should **return** it (or expose it on the in-memory record) so the host **frontend** can display or pass it along; the **backend** stores and indexes it for lookup and idempotent re-ingest
- Optional **analytics enable** flag (or consent hook): turn off product analytics without disabling error logs
- **Terminal logging**: optional; in production, do not log to terminal by default
- **Local persistence** via **`LocalLogStore`**: SDK uses **`Append`**, **`ListUnsent`**, **`CountUnsent`**, **`OldestUnsentAge`**, **`MarkSent`**, **`DeleteSyncedOlderThan`**
  - Persist to the store on SDK-defined error paths (e.g. after **`Append`** from the in-memory buffer)
  - Configurable **max records per upload** (see defaults below)
  - Function to force a batch flush / sync
- **Background sync**: listener polls on **`background_poll_interval`**; when **count** or **age** thresholds are met, upload (see defaults below)
  - Automatically send logs to the backend
  - Function to force send
  - On error, send all pending logs
  - **All-or-nothing batch:** on **full** server success only, call **`MarkSent`** for **all** ids in that batch; on failure, **do not** mark any
  - Purge **synced** rows via **`DeleteSyncedOlderThan`** on the **default schedule** below

### Client logger — default thresholds

All values are **defaults**; hosts may override via config. Tune for latency vs battery/network.

| Setting | Default | Meaning |
| ------- | ------- | ------- |
| **max_records_per_upload** | **100** | Maximum records per **gRPC** ingest call (and per local “batch slice” when uploading) |
| **background_poll_interval** | **5s** | How often the background listener wakes to evaluate pending **unsent** rows |
| **auto_send_min_unsent_count** | **100** | If **unsent** row count **≥** this, trigger an upload (even if the last poll was recent) |
| **auto_send_max_unsent_age** | **168h** (7 days) | If **`OldestUnsentAge` ≥** this, trigger an upload **even when** count **<** `auto_send_min_unsent_count` (any unsent row waiting **≥ 7 days** in the outbox forces a send; threshold is **wall-clock time since `queued_at`**, not log event age; override with e.g. **`24h`** for more aggressive sync) |
| **local_purge_synced_older_than** | **168h** (7 days) | Delete local rows with **`server_acked_at`** older than **`now - 168h`**; **`0`** means purge as soon as acked (may still batch); **never** compare to **event** time for this rule |
| **local_purge_run_interval** | **same as `background_poll_interval`** (`5s` default) | Run **`DeleteSyncedOlderThan`** on this tick **and** **immediately after** each successful **`MarkSent`** (default schedule) |

**Ordering:** On each poll, if **either** `CountUnsent` **≥** `auto_send_min_unsent_count` **or** `OldestUnsentAge` **≥** `auto_send_max_unsent_age`, run an upload up to **max_records_per_upload** per call until caught up or an error occurs. After a successful batch, **`MarkSent`** then **purge** (per **`local_purge_run_interval`** rules above).

---

## Server

| Capability | Details |
| ---------- | ------- |
| **Ingest (gRPC)** | **`IngestBatch`**: batch **`application_name`** + records (**same tenant**, **unique `log_id` within batch**); **single DB transaction**; **idempotent** existing ids; **`IngestBatchResponse.accepted_count`** |
| **Ingest (HTTPS)** | **`POST /api/v1/ingest/batch`** — same rules as gRPC; **Bearer** auth |
| **Query (gRPC)** | **`ListLogs`**: required **`application_name`** + **Query filters**; paginated or server-streaming |
| **Query (HTTPS)** | **`GET /logs?application_name=…`** (simple params); **`POST /logs/query`** (JSON); pagination **cursor** or **`limit`/`offset`** |
| **Delete (gRPC)** | **`DeleteLogs`**: required **`application_name`** + criteria (time range, sessions, etc.) |
| **Delete (HTTPS)** | **`DELETE /api/v1/logs`** with **JSON body** (include **`application_name`** + same filter shape as gRPC); **no** query-string delete criteria (avoids length limits and keeps parity with gRPC) |
| **MCP** | **[Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk)**; **stdio** + **HTTP**; **direct GORM/DB access** (same DSN as server); **shared models/repository** with gRPC/HTTPS to avoid duplicate query logic |

### Default listen address and ports

| Setting | Default | Notes |
| ------- | ------- | ----- |
| **Bind address** | **`0.0.0.0`** (all IPv4 interfaces) | Config e.g. **`LISTEN_BIND_ADDRESS`**; use **`127.0.0.1`** for **local-only** (laptops, CI). Behind Docker/K8s, **`0.0.0.0`** + firewall / NetworkPolicy is the usual pattern. |

| Listener | Default port | Protocol | Notes |
| -------- | ------------ | -------- | ----- |
| **gRPC** (ingest, query, delete RPCs) | **5000** | **gRPC over TLS** | Primary port for the Go SDK |
| **HTTPS JSON API** | **5001** | **HTTPS** | REST/OpenAPI integrators |
| **JSON API (optional plain HTTP)** | **5003** (default when enabled) | **HTTP** | Same **`/api/v1/*`** as HTTPS; **off** unless **`HTTP_PLAIN_LISTEN=true`**; port via **`HTTP_PLAIN_PORT`** |
| **MCP HTTP** (in-process, if enabled) | **5002** | **HTTPS** (MCP streamable HTTP) | Distinct from **5001** to avoid route clashes |
| **Health (HTTPS)** | *same as 5001* | **GET `/api/v1/health`** on the JSON server | No separate port |

**Example client base URLs:** `https://localhost:5001`, optional `http://localhost:5003` when plain HTTP is enabled, gRPC target `localhost:5000` (adjust host if not local).

### Database configuration

All persistence uses **[GORM](https://gorm.io/)** (models, migrations or auto-migrate, queries, transactions): **gRPC handlers**, **HTTPS JSON handlers**, **MCP tool handlers**, and any **HTTP MCP** listener use the **same schema** and should prefer the **same repository/store code** where possible. Dialect and DSN come from config; application code does not branch on raw SQL drivers except what GORM requires for opening the DB.

The server is **configured** (at startup / deploy) to use **one** persistence backend:

| Backend | Typical use |
| ------- | ----------- |
| **Local SQL** | File-backed or embedded SQL via GORM’s SQLite dialector (single-machine / dev-friendly) |
| **Remote PostgreSQL** | Postgres via GORM’s PostgreSQL dialector (connection URL, credentials, pool limits) |

The same logical schema and GORM models apply to both; configuration only switches **dialector + DSN**.

### Server limits (defaults)

| Limit | Default | Notes |
| ----- | ------- | ----- |
| **Max metadata JSON bytes** (per log record at ingest) | **256 KiB** | Applies when enforcement is **on**; **off** means no dedicated cap beyond DB/driver limits |
| **Max gRPC message size** | **4 MiB** (adjust per deployment) | Ingest batches; tune with expected batch **max_records** |

### Query filters (per application / tenant)

**`application_name` is mandatory** on every **ListLogs** / **query** / **delete** path (gRPC, HTTPS, MCP). It is not optional “filter zero” — missing tenant → **`InvalidArgument`** / **400**.

| Filter | Notes |
| ------ | ----- |
| **application_name** | **Required** tenant scope (see **Multi-tenancy**) |
| **log id** | Exact match (single id); optional “any of” list for bulk lookup |
| **record kind** | `operational`, `analytics`, or both |
| **analytics event name** | Exact or prefix (analytics rows) |
| **Time range** | Start and end |
| **session id** | Session scope |
| **user / actor id** | If stored |
| **Regex / pattern** | Same Go **interface** in the shared **repository/query** layer (used by **gRPC**, **HTTPS JSON**, and **MCP**); **SQLite vs Postgres** implementations differ (see implementation notes in code/docs) |
| **log level** | Level filter |

---

## Related documents

- **[implementation-checklist.md](implementation-checklist.md)** — phased implementation tasks (**Phase 0–9**).

---

## Glossary

| Term | Meaning |
| ---- | ------- |
| **Operational** | Classic application logs (levels, messages, errors) |
| **Analytics** | Product / UX events (named events + structured metadata) |
| **Unified batch** | Single upload containing both kinds of records |
| **Local SQL** | Server persistence via embedded / file SQL (e.g. SQLite) |
| **Remote PostgreSQL** | Server persistence via a Postgres server over the network |
| **GORM** | Go ORM used for all server-side database operations (SQLite or Postgres per config) |
| **log id** | Client-generated unique id for one record; shared reference across host UI, client store, API, and server DB |
| **gRPC** | RPC framework (HTTP/2 + Protobuf); primary wire API for the **official Go logger client** |
| **HTTPS JSON API** | REST-style **HTTPS** endpoints with **JSON** bodies; same behavior as gRPC for integrators without the Go SDK |
| **Protobuf** | Serialization schema for gRPC messages; defined in `.proto` files; **HTTPS JSON** should stay aligned with these shapes |
| **MCP stdio** | MCP over stdin/stdout; editor-spawned process, no listen port |
| **MCP HTTP** | MCP over HTTPS (streamable HTTP per SDK); remote clients and shared deployments |
| **MCP direct DB** | MCP tools use **GORM** against the configured DSN; they do not loop back through **gRPC** |
| **Auto-generated TLS** | Used only when **both** effective cert and effective key are missing after **path-first, then PEM** resolution; **partial** cert/key fails startup |
| **LocalLogStore** | Host-implemented Go **interface**; **outbox/join** pattern tracks **server-acked** rows |
| **All-or-nothing ingest** | Server commits a batch in **one transaction**; client **`MarkSent`** only after full success |
| **application_name** | **Tenant** key; required on ingest, query, delete, and MCP tools |
| **MCP_ENABLE_DELETE_LOGS** | When **false** (default), tool **`delete_logs`** is disabled |

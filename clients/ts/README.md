# `@bednaz98/go-logger-client`

TypeScript client for the [Go Logger](https://github.com/Bednaz98/Go-Logger) **HTTPS JSON API** (`/api/v1`). Uses `fetch` (Node 18+, modern browsers, Bun, Deno with fetch).

**Registry:** This package is **published only to GitHub Packages** ([`npm.pkg.github.com`](https://docs.github.com/packages/working-with-a-github-packages-registry/working-with-the-npm-registry)), not **registry.npmjs.org**. CI runs `npm publish --registry https://npm.pkg.github.com`; `package.json` **`publishConfig.registry`** matches. Installing **`typescript`** during local/CI builds still uses the default npm registry for that dev dependency only.

## Install from GitHub Packages

Create or extend **`.npmrc`** in your project (use a read-only GitHub token with `read:packages`). Point `_authToken` at whatever env var you export (example uses `GITHUB_TOKEN`; `gh auth token` works if you export that name):

```
@bednaz98:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${GITHUB_TOKEN}
```

Then:

```bash
npm install @bednaz98/go-logger-client
```

Published versions look like `0.1.0-main.<run_id>.<run_attempt>` on each successful `main` build. In GitHub → **Packages**, set package visibility to **Public** if you want installs without auth (policy varies; many setups still use a token).

**Other forks:** change the `name` field in `package.json` to your GitHub username/org scope (e.g. `@you/go-logger-client`) and adjust `.npmrc` and CI `scope` before publishing.

Upstream CI publishes using the repository secret **`PUBLISH_TOKEN`** as **`NODE_AUTH_TOKEN`** (not the Actions **`GITHUB_TOKEN`**).

## Usage

```typescript
import { LoggerClient } from '@bednaz98/go-logger-client';

const logger = new LoggerClient({
  baseUrl: 'https://localhost:5001', // or http://localhost:5003 with HTTP_PLAIN_LISTEN
  token: process.env.LOGGER_TOKEN, // Bearer; omit if server auth disabled
});

await logger.health();

await logger.log({
  applicationName: 'my-app',
  message: 'User signed in',
  level: 'info',
  metadata: { userId: 'u1' },
});

await logger.track({
  applicationName: 'my-app',
  eventName: 'button_click',
  metadata: { buttonId: 'save' },
});

await logger.ingestBatch({
  application_name: 'my-app',
  records: [
    /* full LogRecord[] — see types */
  ],
});
```

### TLS (dev)

Self-signed server: use your CA with Node (`NODE_EXTRA_CA_CERTS`) or plain HTTP with `HTTP_PLAIN_LISTEN=true` (not for production on untrusted networks).

### Metadata encoding

The server expects `metadata_json` as **base64(UTF-8 JSON)**. `log()` / `track()` handle this; for raw `ingestBatch`, use `metadataToJsonField()` from this package.

## API surface

| Method | HTTP |
| ------ | ---- |
| `health()` | `GET /api/v1/health` |
| `ingestBatch()` | `POST /api/v1/ingest/batch` |
| `log()` | convenience → ingest operational |
| `track()` | convenience → ingest analytics |
| `queryLogs()` | `POST /api/v1/logs/query` |
| `listLogsQueryParams()` | `GET /api/v1/logs?...` |
| `deleteLogs()` | `DELETE /api/v1/logs` |

Errors throw `LoggerApiError` with `status` and optional `code` / `details` from the server JSON problem body.

OpenAPI: [`../../docs/openapi.yaml`](../../docs/openapi.yaml).

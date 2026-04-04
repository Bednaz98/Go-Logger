import { errorFromResponse } from './errors.js';
import { metadataToJsonField } from './metadata.js';
import type {
  DeleteLogsResponse,
  HealthResponse,
  IngestBatchRequest,
  IngestBatchResponse,
  ListLogsResponse,
  LogLevel,
  LogRecord,
  QueryFilter,
} from './types.js';

export interface LoggerClientOptions {
  /** Base URL with no trailing slash, e.g. `https://localhost:5001` or `http://localhost:5003` */
  baseUrl: string;
  /** `Authorization: Bearer <token>` (omit for dev when server auth is disabled) */
  token?: string;
  /** Override `fetch` (tests or custom agents) */
  fetch?: typeof fetch;
}

function normalizeBaseUrl(url: string): string {
  return url.replace(/\/$/, '');
}

function newLogId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2, 12)}`;
}

function nowRFC3339(): string {
  return new Date().toISOString();
}

export class LoggerClient {
  private readonly baseUrl: string;
  private readonly token?: string;
  private readonly fetchImpl: typeof fetch;

  constructor(options: LoggerClientOptions) {
    this.baseUrl = normalizeBaseUrl(options.baseUrl);
    this.token = options.token;
    this.fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
  }

  private headers(init?: HeadersInit): Headers {
    const h = new Headers(init);
    if (this.token) {
      h.set('Authorization', `Bearer ${this.token}`);
    }
    return h;
  }

  private async request(path: string, init: RequestInit): Promise<Response> {
    const headers = this.headers(init.headers);
    if (init.body != null && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json');
    }
    return this.fetchImpl(`${this.baseUrl}${path}`, {
      ...init,
      headers,
    });
  }

  async health(): Promise<HealthResponse> {
    const res = await this.request('/api/v1/health', { method: 'GET' });
    if (!res.ok) {
      throw await errorFromResponse(res);
    }
    return (await res.json()) as HealthResponse;
  }

  async ingestBatch(body: IngestBatchRequest): Promise<IngestBatchResponse> {
    const res = await this.request('/api/v1/ingest/batch', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      throw await errorFromResponse(res);
    }
    return (await res.json()) as IngestBatchResponse;
  }

  /**
   * Single operational log line (generates `log_id` and `event_timestamp` if omitted).
   */
  async log(params: {
    applicationName: string;
    message: string;
    level?: LogLevel | string;
    logId?: string;
    metadata?: Record<string, unknown>;
    userActorId?: string;
    source?: string;
    sourceEnvironment?: string;
    sessionId?: string;
    traceId?: string;
    spanId?: string;
  }): Promise<IngestBatchResponse> {
    const record: LogRecord = {
      log_id: params.logId ?? newLogId(),
      record_kind: 'operational',
      application_name: params.applicationName,
      log_message: params.message,
      event_timestamp: nowRFC3339(),
      log_level: params.level ?? 'info',
      metadata_json: metadataToJsonField(params.metadata),
    };
    if (params.userActorId) record.user_actor_id = params.userActorId;
    if (params.source) record.source = params.source;
    if (params.sourceEnvironment) record.source_environment = params.sourceEnvironment;
    if (params.sessionId) record.session_id = params.sessionId;
    if (params.traceId) record.trace_id = params.traceId;
    if (params.spanId) record.span_id = params.spanId;

    return this.ingestBatch({
      application_name: params.applicationName,
      records: [record],
    });
  }

  /**
   * Single analytics event (`analytics_event_name` required by server).
   */
  async track(params: {
    applicationName: string;
    eventName: string;
    logId?: string;
    metadata?: Record<string, unknown>;
    detailMessage?: string;
    userActorId?: string;
    source?: string;
    sourceEnvironment?: string;
    sessionId?: string;
    traceId?: string;
    spanId?: string;
  }): Promise<IngestBatchResponse> {
    const record: LogRecord = {
      log_id: params.logId ?? newLogId(),
      record_kind: 'analytics',
      application_name: params.applicationName,
      analytics_event_name: params.eventName,
      log_message: params.detailMessage,
      event_timestamp: nowRFC3339(),
      log_level: 'info',
      metadata_json: metadataToJsonField(params.metadata),
    };
    if (params.userActorId) record.user_actor_id = params.userActorId;
    if (params.source) record.source = params.source;
    if (params.sourceEnvironment) record.source_environment = params.sourceEnvironment;
    if (params.sessionId) record.session_id = params.sessionId;
    if (params.traceId) record.trace_id = params.traceId;
    if (params.spanId) record.span_id = params.spanId;

    return this.ingestBatch({
      application_name: params.applicationName,
      records: [record],
    });
  }

  async queryLogs(filter: QueryFilter): Promise<ListLogsResponse> {
    const res = await this.request('/api/v1/logs/query', {
      method: 'POST',
      body: JSON.stringify(filter),
    });
    if (!res.ok) {
      throw await errorFromResponse(res);
    }
    return (await res.json()) as ListLogsResponse;
  }

  async listLogsQueryParams(params: {
    applicationName: string;
    limit?: number;
    pageToken?: string;
    sessionId?: string;
    messageRegex?: string;
  }): Promise<ListLogsResponse> {
    const q = new URLSearchParams();
    q.set('application_name', params.applicationName);
    if (params.limit != null) q.set('limit', String(params.limit));
    if (params.pageToken) q.set('page_token', params.pageToken);
    if (params.sessionId) q.set('session_id', params.sessionId);
    if (params.messageRegex) q.set('message_regex', params.messageRegex);

    const res = await this.request(`/api/v1/logs?${q.toString()}`, { method: 'GET' });
    if (!res.ok) {
      throw await errorFromResponse(res);
    }
    return (await res.json()) as ListLogsResponse;
  }

  async deleteLogs(filter: QueryFilter): Promise<DeleteLogsResponse> {
    const res = await this.request('/api/v1/logs', {
      method: 'DELETE',
      body: JSON.stringify(filter),
    });
    if (!res.ok) {
      throw await errorFromResponse(res);
    }
    return (await res.json()) as DeleteLogsResponse;
  }
}

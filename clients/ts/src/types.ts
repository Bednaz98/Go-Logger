/** Matches HTTPS `/api/v1` JSON (snake_case). */

export type RecordKind = 'operational' | 'analytics';

export type LogLevel = 'trace' | 'debug' | 'info' | 'warn' | 'error' | 'fatal';

export interface LogRecord {
  log_id: string;
  record_kind: RecordKind;
  application_name: string;
  analytics_event_name?: string;
  user_actor_id?: string;
  source?: string;
  source_environment?: string;
  session_id?: string;
  log_message?: string;
  /**
   * UTF-8 JSON bytes as a **base64 string** (Go `encoding/json` unmarshals `[]byte` from a JSON string).
   * Prefer {@link metadataToJsonField} or the high-level `log` / `track` helpers.
   */
  metadata_json?: string;
  event_timestamp: string;
  log_level?: LogLevel | string;
  trace_id?: string;
  span_id?: string;
}

export interface IngestBatchRequest {
  application_name: string;
  records: LogRecord[];
}

export interface IngestBatchResponse {
  accepted_count: number;
  batch_id?: string;
}

export interface HealthResponse {
  status: string;
}

export interface QueryFilter {
  application_name: string;
  log_ids?: string[];
  record_kinds?: string[];
  analytics_event_name_exact?: string;
  analytics_event_name_prefix?: string;
  time_start_rfc3339?: string;
  time_end_rfc3339?: string;
  session_id?: string;
  user_actor_id?: string;
  message_regex?: string;
  log_levels?: string[];
  limit?: number;
  page_token?: string;
}

export interface ListLogsResponse {
  records: LogRecord[];
  next_page_token?: string;
}

export interface DeleteLogsResponse {
  deleted_count: number;
}

export interface ProblemBody {
  code?: string;
  message?: string;
  details?: string;
}

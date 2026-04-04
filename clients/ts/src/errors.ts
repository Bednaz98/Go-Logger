import type { ProblemBody } from './types.js';

export class LoggerApiError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly details?: string;

  constructor(status: number, body?: ProblemBody, fallbackMessage?: string) {
    const msg = body?.message ?? body?.code ?? fallbackMessage ?? `HTTP ${status}`;
    super(msg);
    this.name = 'LoggerApiError';
    this.status = status;
    this.code = body?.code;
    this.details = body?.details;
  }
}

export async function errorFromResponse(res: Response): Promise<LoggerApiError> {
  let body: ProblemBody | undefined;
  try {
    const ct = res.headers.get('content-type') ?? '';
    if (ct.includes('application/json')) {
      body = (await res.json()) as ProblemBody;
    }
  } catch {
    /* ignore */
  }
  return new LoggerApiError(res.status, body);
}

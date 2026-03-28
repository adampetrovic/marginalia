/**
 * Marginalia API client.
 *
 * Handles all HTTP communication with the Marginalia service.
 * Designed to be testable — accepts a `fetch` implementation so tests
 * can inject a mock without touching globals.
 */

// ── Types ──────────────────────────────────────────────────────────

export interface ExportedDocument {
  id: string;
  type: string; // book | article | podcast | tweet
  title: string;
  author: string;
  /** Fully rendered markdown from the template engine */
  content: string;
  updated_at: string;
  highlight_count: number;
  /** sha256 hash of rendered content – used to skip unchanged docs */
  checksum: string;
}

export interface SyncResult {
  [source: string]: {
    status: string;
    documents?: number;
    highlights?: number;
    error?: string;
  };
}

export interface ApiError {
  error: string;
}

// ── Client ─────────────────────────────────────────────────────────

export type FetchFn = typeof globalThis.fetch;

export class MarginaliaClient {
  private baseUrl: string;
  private token: string;
  private fetchFn: FetchFn;

  constructor(baseUrl: string, token: string, fetchFn?: FetchFn) {
    // Strip trailing slash
    this.baseUrl = baseUrl.replace(/\/+$/, "");
    this.token = token;
    this.fetchFn = fetchFn ?? globalThis.fetch.bind(globalThis);
  }

  // ── Sync ───────────────────────────────────────────────────────

  /** Trigger the service to pull from all configured sources. */
  async triggerSync(): Promise<SyncResult> {
    return this.post<SyncResult>("/api/sync");
  }

  /** Trigger sync for a single source (e.g. "readeck"). */
  async triggerSyncSource(source: string): Promise<SyncResult> {
    return this.post<SyncResult>(`/api/sync/${encodeURIComponent(source)}`);
  }

  // ── Export ─────────────────────────────────────────────────────

  /**
   * Fetch rendered documents for export.
   *
   * @param since  ISO-8601 timestamp — only return docs updated after this time.
   *               Omit to fetch everything.
   */
  async getExport(since?: string): Promise<ExportedDocument[]> {
    const params = since ? `?since=${encodeURIComponent(since)}` : "";
    return this.get<ExportedDocument[]>(`/api/export${params}`);
  }

  /** Fetch a single rendered document by ID. */
  async getExportDocument(id: string): Promise<ExportedDocument> {
    return this.get<ExportedDocument>(`/api/export/documents/${encodeURIComponent(id)}`);
  }

  // ── Health ─────────────────────────────────────────────────────

  /** Quick connectivity / auth check. */
  async healthCheck(): Promise<boolean> {
    try {
      await this.get<{ status: string }>("/api/sources");
      return true;
    } catch {
      return false;
    }
  }

  // ── HTTP helpers ───────────────────────────────────────────────

  private headers(): Record<string, string> {
    return {
      Authorization: `Bearer ${this.token}`,
      "Content-Type": "application/json",
      Accept: "application/json",
    };
  }

  private async get<T>(path: string): Promise<T> {
    const res = await this.fetchFn(`${this.baseUrl}${path}`, {
      method: "GET",
      headers: this.headers(),
    });
    return this.handleResponse<T>(res);
  }

  private async post<T>(path: string, body?: unknown): Promise<T> {
    const res = await this.fetchFn(`${this.baseUrl}${path}`, {
      method: "POST",
      headers: this.headers(),
      body: body ? JSON.stringify(body) : undefined,
    });
    return this.handleResponse<T>(res);
  }

  private async handleResponse<T>(res: Response): Promise<T> {
    if (!res.ok) {
      let message = `HTTP ${res.status}`;
      try {
        const err: ApiError = await res.json();
        if (err.error) message = err.error;
      } catch {
        // response body wasn't JSON — keep the status message
      }
      throw new Error(message);
    }
    return (await res.json()) as T;
  }
}

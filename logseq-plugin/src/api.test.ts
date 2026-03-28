import { describe, it, expect, vi } from "vitest";
import { MarginaliaClient, type ExportedDocument } from "./api";

// ── Helpers ────────────────────────────────────────────────────────

function mockFetch(status: number, body: unknown): typeof fetch {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  });
}

function client(fetchFn: typeof fetch) {
  return new MarginaliaClient("https://m.example.com", "test-token", fetchFn);
}

// ── Tests ──────────────────────────────────────────────────────────

describe("MarginaliaClient", () => {
  describe("constructor", () => {
    it("strips trailing slash from base URL", () => {
      const fn = mockFetch(200, []);
      const c = new MarginaliaClient("https://m.example.com/", "tok", fn);
      c.getExport();
      expect(fn).toHaveBeenCalledWith(
        "https://m.example.com/api/export",
        expect.anything(),
      );
    });
  });

  describe("triggerSync", () => {
    it("sends POST /api/sync with auth header", async () => {
      const fn = mockFetch(200, { readeck: { status: "completed" } });
      const c = client(fn);
      const result = await c.triggerSync();

      expect(fn).toHaveBeenCalledWith("https://m.example.com/api/sync", {
        method: "POST",
        headers: {
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: undefined,
      });
      expect(result.readeck.status).toBe("completed");
    });
  });

  describe("triggerSyncSource", () => {
    it("sends POST /api/sync/{source}", async () => {
      const fn = mockFetch(200, { status: "completed" });
      const c = client(fn);
      await c.triggerSyncSource("readeck");

      expect(fn).toHaveBeenCalledWith(
        "https://m.example.com/api/sync/readeck",
        expect.anything(),
      );
    });
  });

  describe("getExport", () => {
    const docs: ExportedDocument[] = [
      {
        id: "doc-1",
        type: "book",
        title: "Test Book",
        author: "Author",
        content: "title:: Test Book",
        updated_at: "2025-01-01T00:00:00Z",
        highlight_count: 3,
        checksum: "sha256:abc",
      },
    ];

    it("fetches all documents when no since parameter", async () => {
      const fn = mockFetch(200, docs);
      const c = client(fn);
      const result = await c.getExport();

      expect(fn).toHaveBeenCalledWith(
        "https://m.example.com/api/export",
        expect.anything(),
      );
      expect(result).toEqual(docs);
    });

    it("includes since query parameter when provided", async () => {
      const fn = mockFetch(200, docs);
      const c = client(fn);
      await c.getExport("2025-01-01T00:00:00Z");

      expect(fn).toHaveBeenCalledWith(
        "https://m.example.com/api/export?since=2025-01-01T00%3A00%3A00Z",
        expect.anything(),
      );
    });
  });

  describe("getExportDocument", () => {
    it("fetches a single document by ID", async () => {
      const doc: ExportedDocument = {
        id: "doc-1",
        type: "book",
        title: "Test",
        author: "Author",
        content: "rendered",
        updated_at: "2025-01-01T00:00:00Z",
        highlight_count: 1,
        checksum: "sha256:abc",
      };
      const fn = mockFetch(200, doc);
      const c = client(fn);
      const result = await c.getExportDocument("doc-1");

      expect(fn).toHaveBeenCalledWith(
        "https://m.example.com/api/export/documents/doc-1",
        expect.anything(),
      );
      expect(result.title).toBe("Test");
    });
  });

  describe("healthCheck", () => {
    it("returns true on success", async () => {
      const fn = mockFetch(200, []);
      const c = client(fn);
      expect(await c.healthCheck()).toBe(true);
    });

    it("returns false on error", async () => {
      const fn = mockFetch(401, { error: "unauthorized" });
      const c = client(fn);
      expect(await c.healthCheck()).toBe(false);
    });

    it("returns false on network error", async () => {
      const fn = vi.fn().mockRejectedValue(new Error("network error"));
      const c = client(fn);
      expect(await c.healthCheck()).toBe(false);
    });
  });

  describe("error handling", () => {
    it("throws with API error message on non-ok response", async () => {
      const fn = mockFetch(401, { error: "invalid token" });
      const c = client(fn);
      await expect(c.triggerSync()).rejects.toThrow("invalid token");
    });

    it("throws with HTTP status when response has no error field", async () => {
      const fn = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error("not json")),
      });
      const c = client(fn);
      await expect(c.triggerSync()).rejects.toThrow("HTTP 500");
    });
  });
});

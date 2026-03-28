import { describe, it, expect, vi, beforeEach } from "vitest";
import { runSync, formatSyncMessage, filterChanged, type SyncState, type SyncStats, type SyncDeps } from "./sync";
import type { MarginaliaClient, ExportedDocument, SyncResult } from "./api";
import type { LogseqEditorAPI, LogseqBlock } from "./writer";

// ── Fixtures ───────────────────────────────────────────────────────

function makeDoc(overrides?: Partial<ExportedDocument>): ExportedDocument {
  return {
    id: "doc-1",
    type: "book",
    title: "Test Book",
    author: "Author",
    content: "title:: Test Book\nauthor:: Author\n\n- ## Highlights\n\t- > A highlight",
    updated_at: "2025-01-01T00:00:00Z",
    highlight_count: 1,
    checksum: "sha256:abc123",
    ...overrides,
  };
}

function makeClient(docs: ExportedDocument[] = []): MarginaliaClient {
  return {
    triggerSync: vi.fn().mockResolvedValue({ readeck: { status: "completed" } }),
    getExport: vi.fn().mockResolvedValue(docs),
    getExportDocument: vi.fn(),
    healthCheck: vi.fn().mockResolvedValue(true),
    triggerSyncSource: vi.fn(),
  } as unknown as MarginaliaClient;
}

function makeEditor(): LogseqEditorAPI {
  let nextUUID = 1;
  return {
    getPage: vi.fn().mockResolvedValue(null),
    createPage: vi.fn().mockImplementation(async () => ({ uuid: `uuid-${nextUUID++}` })),
    getPageBlocksTree: vi.fn().mockResolvedValue([]),
    insertBlock: vi.fn().mockImplementation(async () => ({ uuid: `uuid-${nextUUID++}` })),
    updateBlock: vi.fn().mockResolvedValue(undefined),
    removeBlock: vi.fn().mockResolvedValue(undefined),
  };
}

const defaultSettings = {
  bookNamespace: "Books",
  articleNamespace: "Articles",
  podcastNamespace: "Podcasts",
};

function emptyState(): SyncState {
  return { checksumCache: {} };
}

// ── Tests ──────────────────────────────────────────────────────────

describe("runSync", () => {
  it("triggers server sync, fetches export, writes documents", async () => {
    const doc = makeDoc();
    const client = makeClient([doc]);
    const editor = makeEditor();
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const { state, stats } = await runSync(deps, emptyState());

    expect(client.triggerSync).toHaveBeenCalledOnce();
    expect(client.getExport).toHaveBeenCalledWith(undefined);
    expect(editor.createPage).toHaveBeenCalled();

    expect(stats.totalDocuments).toBe(1);
    expect(stats.written).toBe(1);
    expect(stats.skipped).toBe(0);
    expect(stats.errors).toBe(0);

    expect(state.checksumCache["doc-1"]).toBe("sha256:abc123");
    expect(state.lastSyncTimestamp).toBeDefined();
  });

  it("uses since parameter from previous sync state", async () => {
    const client = makeClient([]);
    const editor = makeEditor();
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const state: SyncState = {
      lastSyncTimestamp: "2025-01-01T00:00:00Z",
      checksumCache: {},
    };

    await runSync(deps, state);

    expect(client.getExport).toHaveBeenCalledWith("2025-01-01T00:00:00Z");
  });

  it("skips documents with unchanged checksum", async () => {
    const doc = makeDoc({ checksum: "sha256:same" });
    const client = makeClient([doc]);
    const editor = makeEditor();
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const state: SyncState = {
      checksumCache: { "doc-1": "sha256:same" },
    };

    const { stats } = await runSync(deps, state);

    expect(stats.totalDocuments).toBe(1);
    expect(stats.skipped).toBe(1);
    expect(stats.written).toBe(0);
    expect(editor.createPage).not.toHaveBeenCalled();
  });

  it("writes document when checksum changed", async () => {
    const doc = makeDoc({ checksum: "sha256:new" });
    const client = makeClient([doc]);
    const editor = makeEditor();
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const state: SyncState = {
      checksumCache: { "doc-1": "sha256:old" },
    };

    const { state: newState, stats } = await runSync(deps, state);

    expect(stats.written).toBe(1);
    expect(newState.checksumCache["doc-1"]).toBe("sha256:new");
  });

  it("handles multiple documents", async () => {
    const docs = [
      makeDoc({ id: "doc-1", checksum: "sha256:same" }),
      makeDoc({ id: "doc-2", title: "Book Two", checksum: "sha256:new" }),
      makeDoc({ id: "doc-3", title: "Book Three", checksum: "sha256:brand-new" }),
    ];
    const client = makeClient(docs);
    const editor = makeEditor();
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const state: SyncState = {
      checksumCache: {
        "doc-1": "sha256:same",
        "doc-2": "sha256:old",
      },
    };

    const { stats, state: newState } = await runSync(deps, state);

    expect(stats.totalDocuments).toBe(3);
    expect(stats.skipped).toBe(1); // doc-1 unchanged
    expect(stats.written).toBe(2); // doc-2 changed, doc-3 new
    expect(newState.checksumCache["doc-2"]).toBe("sha256:new");
    expect(newState.checksumCache["doc-3"]).toBe("sha256:brand-new");
  });

  it("continues on triggerSync failure", async () => {
    const doc = makeDoc();
    const client = makeClient([doc]);
    (client.triggerSync as any).mockRejectedValue(new Error("sync fail"));
    const editor = makeEditor();
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const { stats } = await runSync(deps, emptyState());

    // Should still fetch and write
    expect(client.getExport).toHaveBeenCalled();
    expect(stats.written).toBe(1);
  });

  it("counts write errors without stopping", async () => {
    const docs = [
      makeDoc({ id: "doc-1" }),
      makeDoc({ id: "doc-2", title: "Book Two" }),
    ];
    const client = makeClient(docs);
    const editor = makeEditor();
    // First createPage succeeds, second fails
    let callCount = 0;
    (editor.createPage as any).mockImplementation(async () => {
      callCount++;
      if (callCount === 2) throw new Error("write fail");
      return { uuid: "uuid-1" };
    });
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const { stats, state } = await runSync(deps, emptyState());

    expect(stats.written).toBe(1);
    expect(stats.errors).toBe(1);
    // Only successful doc should be in checksum cache
    expect(state.checksumCache["doc-1"]).toBeDefined();
    expect(state.checksumCache["doc-2"]).toBeUndefined();
  });

  it("handles empty export (no documents)", async () => {
    const client = makeClient([]);
    const editor = makeEditor();
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const { stats } = await runSync(deps, emptyState());

    expect(stats.totalDocuments).toBe(0);
    expect(stats.written).toBe(0);
    expect(stats.skipped).toBe(0);
  });

  it("records durationMs", async () => {
    const client = makeClient([]);
    const editor = makeEditor();
    const deps: SyncDeps = { client, editor, settings: defaultSettings };

    const { stats } = await runSync(deps, emptyState());

    expect(stats.durationMs).toBeGreaterThanOrEqual(0);
  });
});

// ── formatSyncMessage ──────────────────────────────────────────────

describe("formatSyncMessage", () => {
  it("shows no documents message", () => {
    const stats: SyncStats = { totalDocuments: 0, skipped: 0, written: 0, errors: 0, durationMs: 50 };
    expect(formatSyncMessage(stats)).toBe("No new or updated documents.");
  });

  it("shows written count", () => {
    const stats: SyncStats = { totalDocuments: 3, skipped: 0, written: 3, errors: 0, durationMs: 250 };
    expect(formatSyncMessage(stats)).toBe("3 documents synced (250ms)");
  });

  it("shows singular form", () => {
    const stats: SyncStats = { totalDocuments: 1, skipped: 0, written: 1, errors: 0, durationMs: 100 };
    expect(formatSyncMessage(stats)).toBe("1 document synced (100ms)");
  });

  it("shows skipped and written", () => {
    const stats: SyncStats = { totalDocuments: 5, skipped: 3, written: 2, errors: 0, durationMs: 400 };
    expect(formatSyncMessage(stats)).toBe("2 documents synced, 3 unchanged (400ms)");
  });

  it("shows errors", () => {
    const stats: SyncStats = { totalDocuments: 3, skipped: 1, written: 1, errors: 1, durationMs: 300 };
    expect(formatSyncMessage(stats)).toBe("1 document synced, 1 unchanged, 1 failed (300ms)");
  });

  it("formats seconds for long syncs", () => {
    const stats: SyncStats = { totalDocuments: 1, skipped: 0, written: 1, errors: 0, durationMs: 2500 };
    expect(formatSyncMessage(stats)).toBe("1 document synced (2.5s)");
  });
});

// ── filterChanged ──────────────────────────────────────────────────

describe("filterChanged", () => {
  it("filters out unchanged documents", () => {
    const docs = [
      makeDoc({ id: "1", checksum: "sha256:same" }),
      makeDoc({ id: "2", checksum: "sha256:new" }),
    ];
    const cache = { "1": "sha256:same" };
    const result = filterChanged(docs, cache);

    expect(result).toHaveLength(1);
    expect(result[0].id).toBe("2");
  });

  it("includes all documents when cache is empty", () => {
    const docs = [makeDoc({ id: "1" }), makeDoc({ id: "2" })];
    const result = filterChanged(docs, {});
    expect(result).toHaveLength(2);
  });

  it("returns empty when all unchanged", () => {
    const docs = [makeDoc({ id: "1", checksum: "sha256:a" })];
    const cache = { "1": "sha256:a" };
    expect(filterChanged(docs, cache)).toHaveLength(0);
  });
});

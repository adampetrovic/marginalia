/**
 * Sync engine — orchestrates the full sync cycle.
 *
 * 1. Trigger server-side sync (Marginalia fetches from sources)
 * 2. Fetch exported documents (incremental via `since` timestamp)
 * 3. Skip unchanged documents (checksum comparison)
 * 4. Write new/updated documents to the Logseq graph
 * 5. Update local state (last sync timestamp, checksum cache)
 */

import type { MarginaliaClient, ExportedDocument } from "./api";
import type { LogseqEditorAPI } from "./writer";
import { writeDocument } from "./writer";

// ── Types ──────────────────────────────────────────────────────────

export interface SyncState {
  /** ISO-8601 timestamp of last successful sync. */
  lastSyncTimestamp?: string;
  /** Map of document ID → checksum for skip-unchanged logic. */
  checksumCache: Record<string, string>;
}

export interface SyncStats {
  /** Total documents returned by the API. */
  totalDocuments: number;
  /** Documents skipped because checksum hasn't changed. */
  skipped: number;
  /** Documents written (new or updated). */
  written: number;
  /** Documents that failed to write. */
  errors: number;
  /** Duration of the sync in milliseconds. */
  durationMs: number;
}

export interface SyncDeps {
  client: MarginaliaClient;
  editor: LogseqEditorAPI;
  settings: {
    bookNamespace: string;
    articleNamespace: string;
    podcastNamespace: string;
  };
}

// ── Sync logic ─────────────────────────────────────────────────────

/**
 * Run a full sync cycle.
 *
 * @param deps     Injected dependencies (API client, editor, settings)
 * @param state    Current sync state (timestamps, checksums)
 * @returns        Updated state + stats for the UI
 */
export async function runSync(
  deps: SyncDeps,
  state: SyncState,
): Promise<{ state: SyncState; stats: SyncStats }> {
  const start = Date.now();
  const stats: SyncStats = {
    totalDocuments: 0,
    skipped: 0,
    written: 0,
    errors: 0,
    durationMs: 0,
  };

  // 1. Trigger server-side sync
  try {
    await deps.client.triggerSync();
  } catch (err) {
    // Non-fatal — maybe sources aren't configured. Continue to export.
    console.warn("Server sync trigger failed (continuing):", err);
  }

  // 2. Fetch exported documents
  const documents = await deps.client.getExport(state.lastSyncTimestamp);
  stats.totalDocuments = documents.length;

  // 3. Process each document
  const newChecksumCache = { ...state.checksumCache };

  for (const doc of documents) {
    // Skip if checksum unchanged
    if (state.checksumCache[doc.id] === doc.checksum) {
      stats.skipped++;
      continue;
    }

    try {
      await writeDocument(deps.editor, doc, deps.settings);
      newChecksumCache[doc.id] = doc.checksum;
      stats.written++;
    } catch (err) {
      console.error(`Failed to write document "${doc.title}":`, err);
      stats.errors++;
    }
  }

  stats.durationMs = Date.now() - start;

  // 4. Update state
  const newState: SyncState = {
    lastSyncTimestamp: new Date().toISOString(),
    checksumCache: newChecksumCache,
  };

  return { state: newState, stats };
}

// ── Helpers ────────────────────────────────────────────────────────

/**
 * Build a human-readable summary of the sync for notifications.
 */
export function formatSyncMessage(stats: SyncStats): string {
  if (stats.totalDocuments === 0) {
    return "No new or updated documents.";
  }

  const parts: string[] = [];
  if (stats.written > 0) {
    parts.push(`${stats.written} document${stats.written !== 1 ? "s" : ""} synced`);
  }
  if (stats.skipped > 0) {
    parts.push(`${stats.skipped} unchanged`);
  }
  if (stats.errors > 0) {
    parts.push(`${stats.errors} failed`);
  }

  const duration = stats.durationMs < 1000 ? `${stats.durationMs}ms` : `${(stats.durationMs / 1000).toFixed(1)}s`;

  return `${parts.join(", ")} (${duration})`;
}

/**
 * Filter documents that need writing (checksum changed or new).
 * Exported for testing.
 */
export function filterChanged(
  documents: ExportedDocument[],
  checksumCache: Record<string, string>,
): ExportedDocument[] {
  return documents.filter((doc) => checksumCache[doc.id] !== doc.checksum);
}

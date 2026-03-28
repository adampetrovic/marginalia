/**
 * Marginalia Logseq Plugin — entry point.
 *
 * Registers the toolbar button, slash command, settings schema,
 * and auto-sync interval. All logic is delegated to the sync engine.
 */

import "@logseq/libs";
import { SETTINGS_SCHEMA } from "./settings";
import { MarginaliaClient } from "./api";
import { runSync, formatSyncMessage, type SyncState } from "./sync";
import type { LogseqEditorAPI } from "./writer";

async function main() {
  logseq.useSettingsSchema(SETTINGS_SCHEMA);

  // Toolbar button
  logseq.App.registerUIItem("toolbar", {
    key: "marginalia-sync",
    template: `<a class="button" data-on-click="syncHighlights" title="Sync Marginalia highlights">📚</a>`,
  });

  logseq.provideModel({
    async syncHighlights() {
      await doSync();
    },
  });

  // Slash command
  logseq.Editor.registerSlashCommand("Marginalia Sync", async () => {
    await doSync();
  });

  // Sync on startup
  if (logseq.settings?.syncOnStartup) {
    setTimeout(() => doSync(), 3000);
  }

  // Auto-sync interval
  if (logseq.settings?.autoSync) {
    const intervalMs = (logseq.settings?.autoSyncInterval || 30) * 60 * 1000;
    setInterval(() => doSync(), intervalMs);
  }

  logseq.App.showMsg("📚 Marginalia plugin loaded");
}

/**
 * Run a sync cycle with the Logseq Editor API as the output target.
 */
async function doSync(): Promise<void> {
  const serviceUrl = logseq.settings?.serviceUrl as string;
  const apiToken = logseq.settings?.apiToken as string;

  if (!serviceUrl || !apiToken) {
    logseq.App.showMsg(
      "⚠️ Configure Marginalia service URL and API token in plugin settings",
      "warning",
    );
    return;
  }

  logseq.App.showMsg("🔄 Syncing highlights from Marginalia...");

  try {
    const client = new MarginaliaClient(serviceUrl, apiToken);

    // Build an adapter from logseq.Editor to the LogseqEditorAPI interface
    const editor: LogseqEditorAPI = {
      getPage: (name) => logseq.Editor.getPage(name) as any,
      createPage: (name, props, opts) => logseq.Editor.createPage(name, props, opts) as any,
      getPageBlocksTree: (name) => logseq.Editor.getPageBlocksTree(name) as any,
      insertBlock: (parent, content, opts) => logseq.Editor.insertBlock(parent, content, opts) as any,
      updateBlock: (uuid, content) => logseq.Editor.updateBlock(uuid, content) as any,
      removeBlock: (uuid) => logseq.Editor.removeBlock(uuid) as any,
    };

    // Load persisted state
    const state: SyncState = {
      lastSyncTimestamp: logseq.settings?.lastSyncTimestamp as string | undefined,
      checksumCache: (logseq.settings?.checksumCache as Record<string, string>) ?? {},
    };

    const settings = {
      bookNamespace: (logseq.settings?.bookNamespace as string) || "Books",
      articleNamespace: (logseq.settings?.articleNamespace as string) || "Articles",
      podcastNamespace: (logseq.settings?.podcastNamespace as string) || "Podcasts",
    };

    const result = await runSync({ client, editor, settings }, state);

    // Persist updated state
    logseq.updateSettings({
      lastSyncTimestamp: result.state.lastSyncTimestamp,
      checksumCache: result.state.checksumCache,
    });

    const message = formatSyncMessage(result.stats);
    const level = result.stats.errors > 0 ? "warning" : "success";
    logseq.App.showMsg(`📚 ${message}`, level);
  } catch (err) {
    console.error("Marginalia sync error:", err);
    logseq.App.showMsg(`❌ Sync failed: ${err}`, "error");
  }
}

// Bootstrap
logseq.ready(main).catch(console.error);

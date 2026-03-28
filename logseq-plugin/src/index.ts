import "@logseq/libs";

const SETTINGS_SCHEMA = [
  {
    key: "serviceUrl",
    type: "string" as const,
    title: "Marginalia Service URL",
    description: "URL of your Marginalia instance",
    default: "",
  },
  {
    key: "apiToken",
    type: "string" as const,
    title: "API Token",
    description: "Bearer token for Marginalia API authentication",
    default: "",
  },
  {
    key: "bookNamespace",
    type: "string" as const,
    title: "Book Namespace",
    description: "Logseq namespace prefix for book pages",
    default: "Books",
  },
  {
    key: "articleNamespace",
    type: "string" as const,
    title: "Article Namespace",
    description: "Logseq namespace prefix for article pages",
    default: "Articles",
  },
  {
    key: "syncOnStartup",
    type: "boolean" as const,
    title: "Sync on Startup",
    description: "Automatically sync highlights when Logseq starts",
    default: false,
  },
  {
    key: "autoSync",
    type: "boolean" as const,
    title: "Auto Sync",
    description: "Periodically sync highlights in the background",
    default: false,
  },
  {
    key: "autoSyncInterval",
    type: "number" as const,
    title: "Auto Sync Interval (minutes)",
    description: "How often to auto-sync (if enabled)",
    default: 30,
  },
];

async function main() {
  logseq.useSettingsSchema(SETTINGS_SCHEMA);

  // Register toolbar button
  logseq.App.registerUIItem("toolbar", {
    key: "marginalia-sync",
    template: `<a class="button" data-on-click="syncHighlights" title="Sync Marginalia highlights">📚</a>`,
  });

  // Register model for UI callbacks
  logseq.provideModel({
    async syncHighlights() {
      await syncFromMarginalia();
    },
  });

  // Register slash command
  logseq.Editor.registerSlashCommand("Marginalia Sync", async () => {
    await syncFromMarginalia();
  });

  // Sync on startup if enabled
  if (logseq.settings?.syncOnStartup) {
    setTimeout(() => syncFromMarginalia(), 3000);
  }

  // Auto-sync interval
  if (logseq.settings?.autoSync) {
    const interval = (logseq.settings?.autoSyncInterval || 30) * 60 * 1000;
    setInterval(() => syncFromMarginalia(), interval);
  }

  logseq.App.showMsg("Marginalia plugin loaded");
}

async function syncFromMarginalia() {
  const serviceUrl = logseq.settings?.serviceUrl;
  const apiToken = logseq.settings?.apiToken;

  if (!serviceUrl || !apiToken) {
    logseq.App.showMsg("⚠️ Please configure Marginalia service URL and API token in plugin settings", "warning");
    return;
  }

  try {
    logseq.App.showMsg("🔄 Syncing highlights from Marginalia...");

    // Trigger sync on the server
    await fetch(`${serviceUrl}/api/sync`, {
      method: "POST",
      headers: { Authorization: `Bearer ${apiToken}` },
    });

    // Fetch exported documents
    const lastSync = logseq.settings?.lastSyncTimestamp || "";
    const url = lastSync
      ? `${serviceUrl}/api/export?since=${encodeURIComponent(lastSync)}`
      : `${serviceUrl}/api/export`;

    const response = await fetch(url, {
      headers: { Authorization: `Bearer ${apiToken}` },
    });

    if (!response.ok) {
      throw new Error(`API returned ${response.status}`);
    }

    const documents: ExportedDocument[] = await response.json();

    let synced = 0;
    for (const doc of documents) {
      await writeDocumentToGraph(doc);
      synced++;
    }

    // Save last sync timestamp
    logseq.updateSettings({ lastSyncTimestamp: new Date().toISOString() });

    logseq.App.showMsg(`✅ Synced ${synced} document${synced !== 1 ? "s" : ""} from Marginalia`, "success");
  } catch (err) {
    console.error("Marginalia sync error:", err);
    logseq.App.showMsg(`❌ Sync failed: ${err}`, "error");
  }
}

interface ExportedDocument {
  id: string;
  type: string;
  title: string;
  author: string;
  content: string;
  updated_at: string;
  highlight_count: number;
  checksum: string;
}

function getNamespaceForType(type: string): string {
  switch (type) {
    case "article":
      return logseq.settings?.articleNamespace || "Articles";
    case "podcast":
      return logseq.settings?.podcastNamespace || "Podcasts";
    default:
      return logseq.settings?.bookNamespace || "Books";
  }
}

function sanitizeTitle(title: string): string {
  // Remove characters that are invalid in Logseq page names
  return title.replace(/[/\\:*?"<>|#]/g, "").trim();
}

async function writeDocumentToGraph(doc: ExportedDocument) {
  const namespace = getNamespaceForType(doc.type);
  const pageName = `${namespace}/${sanitizeTitle(doc.title)}`;

  // Check if page exists
  const existing = await logseq.Editor.getPage(pageName);

  if (existing) {
    // Page exists — clear and rewrite with new content
    // (preserves the page but updates all blocks)
    const blocks = await logseq.Editor.getPageBlocksTree(pageName);
    for (const block of blocks) {
      await logseq.Editor.removeBlock(block.uuid);
    }
  } else {
    await logseq.Editor.createPage(pageName, {}, { redirect: false });
  }

  // Insert the rendered content as blocks
  const page = await logseq.Editor.getPage(pageName);
  if (!page) return;

  // Split content into lines and insert as a block tree
  const lines = doc.content.split("\n");

  // First, insert property lines at the page level
  const propertyLines: string[] = [];
  let contentStart = 0;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].match(/^\w+::/)) {
      propertyLines.push(lines[i]);
      contentStart = i + 1;
    } else {
      break;
    }
  }

  // Insert remaining content as blocks
  const remainingContent = lines.slice(contentStart).join("\n").trim();
  if (remainingContent) {
    // Update page properties
    if (propertyLines.length > 0) {
      const propsBlock = await logseq.Editor.insertBlock(page.uuid, propertyLines.join("\n"), { isPageBlock: true });
    }

    // Insert content blocks
    const contentLines = remainingContent.split("\n").filter((l) => l.trim());
    let lastBlock: any = null;
    for (const line of contentLines) {
      const trimmed = line.replace(/^\t*- /, "").replace(/^\t*/, "");
      if (trimmed) {
        const indent = (line.match(/^\t*/)?.[0] || "").length;
        const parent = indent > 0 && lastBlock ? lastBlock.uuid : page.uuid;
        lastBlock = await logseq.Editor.insertBlock(parent, trimmed, {
          sibling: indent === 0,
        });
      }
    }
  }
}

// Bootstrap
logseq.ready(main).catch(console.error);

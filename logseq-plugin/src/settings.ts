import { SettingSchemaDesc } from "@logseq/libs/dist/LSPlugin";

export const SETTINGS_SCHEMA: SettingSchemaDesc[] = [
  {
    key: "serviceUrl",
    type: "string",
    title: "Marginalia Service URL",
    description: "URL of your Marginalia instance (e.g. https://marginalia.example.com)",
    default: "",
  },
  {
    key: "apiToken",
    type: "string",
    title: "API Token",
    description: "Bearer token for Marginalia API authentication",
    default: "",
  },
  {
    key: "bookNamespace",
    type: "string",
    title: "Book Namespace",
    description: "Logseq namespace prefix for book pages (e.g. Books → Books/Title)",
    default: "Books",
  },
  {
    key: "articleNamespace",
    type: "string",
    title: "Article Namespace",
    description: "Logseq namespace prefix for article pages",
    default: "Articles",
  },
  {
    key: "podcastNamespace",
    type: "string",
    title: "Podcast Namespace",
    description: "Logseq namespace prefix for podcast pages",
    default: "Podcasts",
  },
  {
    key: "syncOnStartup",
    type: "boolean",
    title: "Sync on Startup",
    description: "Automatically sync highlights when Logseq starts",
    default: false,
  },
  {
    key: "autoSync",
    type: "boolean",
    title: "Auto Sync",
    description: "Periodically sync highlights in the background",
    default: false,
  },
  {
    key: "autoSyncInterval",
    type: "number",
    title: "Auto Sync Interval (minutes)",
    description: "How often to auto-sync (if enabled)",
    default: 30,
  },
];

export interface PluginSettings {
  serviceUrl: string;
  apiToken: string;
  bookNamespace: string;
  articleNamespace: string;
  podcastNamespace: string;
  syncOnStartup: boolean;
  autoSync: boolean;
  autoSyncInterval: number;
  /** Stored internally — last successful sync ISO timestamp */
  lastSyncTimestamp?: string;
  /** Stored internally — checksum cache per document ID */
  checksumCache?: Record<string, string>;
}

# Logseq Plugin — Marginalia

Syncs highlights from a [Marginalia](../README.md) service into your Logseq graph.

## Features

- **Toolbar sync button** — click 📚 to trigger a manual sync
- **Slash command** — type `/Marginalia Sync` to sync
- **Incremental sync** — tracks last sync timestamp, only fetches new/updated documents
- **Checksum caching** — skips documents that haven't changed since last sync
- **Merge strategy** — updates highlights without losing your own notes
- **Auto-sync** — optional periodic background sync
- **Namespace routing** — books, articles, and podcasts go into configurable namespaces

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| Service URL | | Your Marginalia instance URL |
| API Token | | Bearer token for authentication |
| Book Namespace | `Books` | Page prefix for books (e.g. `Books/Title`) |
| Article Namespace | `Articles` | Page prefix for articles |
| Podcast Namespace | `Podcasts` | Page prefix for podcasts |
| Sync on Startup | `false` | Auto-sync when Logseq starts |
| Auto Sync | `false` | Periodic background sync |
| Auto Sync Interval | `30` min | How often to auto-sync |

## Architecture

```
src/
├── index.ts       # Plugin entry point, registers UI + wiring
├── settings.ts    # Settings schema + types
├── api.ts         # Marginalia API client
├── sync.ts        # Sync engine (orchestration, checksum, state)
└── writer.ts      # Page writer/merger (Logseq-specific)
```

**Separation of concerns:**
- `api.ts` — pure HTTP, no Logseq dependency
- `sync.ts` — orchestration logic, no Logseq dependency (takes an editor interface)
- `writer.ts` — all Logseq-specific page manipulation, accepts an editor interface for testability
- `index.ts` — glue code, only file that imports `@logseq/libs`

## Merge Strategy

When a page already exists in the graph:

1. **Properties** (top of page) — always replaced with latest from Marginalia
2. **Highlights section** (`## Highlights` block + children) — replaced with new rendered content
3. **User blocks** (anything outside the Highlights section) — preserved in place

This means you can safely add your own notes to highlight pages — they won't be overwritten on the next sync.

## Development

```bash
npm install
npm test           # run vitest
npm run dev        # vite dev server
npm run build      # production build → dist/index.js
```

## Installation

1. Build the plugin (`npm run build`)
2. In Logseq: Settings → Plugins → Load unpacked plugin → select this directory
3. Configure the Marginalia service URL and API token in plugin settings

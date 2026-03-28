# Marginalia — Self-Hosted Highlight Sync

> *Marginalia* (noun): notes written in the margins of a book.

A self-hosted Readwise replacement that collects highlights from reading sources, stores them centrally, and serves them to note-taking tools via a plugin architecture.

---

## Architecture Overview

```
                                                          ┌─────────────────┐
  ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐    ┌──────────────┐     │  Output Plugins  │
     Input Sources                  │              │     ├─────────────────┤     ┌────────────┐
  │                           │    │  Marginalia  │     │  Logseq Plugin  │────►│  Logseq    │
    ┌─────────────┐                │  Service     │◄────│  (TypeScript)   │     │  Graph     │
  │ │  Readeck    │◄──── pull  │   │              │     └─────────────────┘     └────────────┘
    └─────────────┘                │  Go + ORM    │
  │                           │    │  SQLite/PG   │      ┌─────────────────┐
    ┌─────────────┐                │              │      │  Future outputs  │
  │ │  KOReader   │────► push  │   │  REST API    │◄─ ─ ─│  (Obsidian,etc) │
    └─────────────┘                └──────────────┘      └─────────────────┘
  │                           │
    ┌ ─ ─ ─ ─ ─ ─ ┐
  │  Kindle txt,               │
     Apple Books,
  │  future inputs ┘          │
  └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
```

### Plugin Architecture

Marginalia is designed around **pluggable input sources and output targets**:

- **Input sources** are adapters that know how to fetch or receive highlights from a specific service (Readeck, KOReader, Kindle clippings file, Apple Books, etc.)
- **Output targets** are client plugins that consume the Marginalia API and write highlights into a specific note-taking tool (Logseq, Obsidian, etc.)
- **The service itself** stores highlights in a normalised schema, renders them via customisable templates, and serves the rendered output via a REST API

The **MVP** implements:
- **Inputs**: Readeck (pull), KOReader (push via Readwise-compatible endpoint)
- **Outputs**: Logseq plugin

Future sources and targets can be added without changing the core service.

### Data Flow

1. **Readeck → Marginalia**: Client triggers sync → Marginalia pulls highlights from Readeck API on-demand
2. **KOReader → Marginalia**: KOReader's exporter plugin pushes highlights to Marginalia's Readwise-compatible API endpoint
3. **Marginalia → Client**: Output plugin requests rendered content from Marginalia → plugin handles writing pages to the target tool (filenames, namespaces, merging)

---

## Components

### 1. Marginalia Service (Go)

A lightweight Go service distributed as a Docker image. Receives and stores highlights from input sources, and serves them as structured JSON to output plugins.

**Database:** Supports **SQLite** (default, zero-config) and **PostgreSQL** via an ORM ([GORM](https://gorm.io/)). The database driver is selected by the `DATABASE_URL` environment variable — a file path for SQLite, a `postgres://` connection string for PostgreSQL.

#### API Endpoints

**Authentication:** Bearer token configured via `MARGINALIA_API_TOKEN` environment variable.

##### Sources & Sync

Sources are configured via environment variables (see Configuration section). The sync API triggers fetching from configured sources.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/sources` | List configured sources and their sync status |
| `POST` | `/api/sync` | Trigger sync from all sources |
| `POST` | `/api/sync/{source}` | Trigger sync from specific source (e.g., `readeck`) |
| `GET` | `/api/sync/status` | Last sync status per source |

##### Documents & Highlights

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/documents` | List all documents (filterable by type, source, updated_since) |
| `GET` | `/api/documents/{id}` | Get document with all highlights |
| `GET` | `/api/highlights` | List all highlights (filterable) |
| `GET` | `/api/highlights/{id}` | Get single highlight |

##### Readwise-Compatible Ingestion (for KOReader)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v2/highlights` | Readwise-format highlight creation (KOReader sends here) |
| `GET` | `/api/v2/auth` | Token validation (KOReader checks this) |

##### Templates

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/templates` | List all templates |
| `GET` | `/api/templates/{id}` | Get template |
| `POST` | `/api/templates` | Create template |
| `PUT` | `/api/templates/{id}` | Update template |
| `GET` | `/api/templates/{id}/preview` | Preview with sample data |
| `POST` | `/api/templates/preview` | Preview arbitrary template with sample data |

##### Export (for output plugins)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/export` | Export all documents as rendered markdown |
| `GET` | `/api/export?since={timestamp}` | Export only documents updated since timestamp |
| `GET` | `/api/export/documents/{id}` | Export single document |

Templates live in Marginalia so that rendering is **consistent across all output plugins** — whether the output is Logseq, Obsidian, or a future tool, the same template produces the same markdown. Output plugins only handle the mechanics of writing (filenames, namespaces, page creation/merging).

The export endpoint returns an array of rendered documents:
```json
[
  {
    "id": "doc_abc123",
    "type": "book",
    "title": "How to Live",
    "author": "Derek Sivers",
    "content": "title:: How to Live\nauthor:: [[Derek Sivers]]\n...",
    "updated_at": "2025-03-28T10:00:00Z",
    "highlight_count": 42,
    "checksum": "sha256:..."
  }
]
```

- `content` — fully rendered markdown from the document's template
- `checksum` — hash of the rendered content, lets plugins skip unchanged documents
- The raw document/highlight JSON is also available via `/api/documents/{id}` for programmatic use

#### Data Model

Managed via [goose](https://github.com/pressly/goose) versioned SQL migrations (supports both SQLite and PostgreSQL). GORM is used as the query layer / ORM but **not** for schema management — migrations are explicit SQL files so schema changes are reviewable, reversible, and predictable across both database backends. Migrations run automatically on startup.

```
┌──────────────┐       ┌──────────────────┐       ┌──────────────────┐
│   Source     │       │    Document      │       │   Highlight      │
├──────────────┤       ├──────────────────┤       ├──────────────────┤
│ id           │──┐    │ id               │──┐    │ id               │
│ type         │  │    │ source_id (FK)   │  │    │ document_id (FK) │
│ name         │  └───►│ source_doc_id    │  └───►│ source_hl_id     │
│ last_synced  │       │ type             │       │ text             │
│ created_at   │       │ title            │       │ note             │
│ updated_at   │       │ author           │       │ color            │
└──────────────┘       │ url              │       │ tags (JSON)      │
                       │ image_url        │       │ location         │
                       │ source_url       │       │ location_type    │
                       │ category         │       │ location_sort    │
                       │ tags (JSON)      │       │ chapter          │
                       │ metadata (JSON)  │       │ page_number      │
                       │ last_highlighted │       │ percentage       │
                       │ last_synced      │       │ highlighted_at   │
                       │ created_at       │       │ synced_at        │
                       │ updated_at       │       │ created_at       │
                       └──────────────────┘       │ updated_at       │
                                                  └──────────────────┘
┌──────────────────┐       ┌──────────────────┐
│   SyncLog        │       │   Template       │
├──────────────────┤       ├──────────────────┤
│ id               │       │ id               │
│ source_id (FK)   │       │ name             │
│ status           │       │ type             │
│ docs_synced      │       │ page_template    │
│ highlights_synced│       │ highlight_tpl    │
│ error            │       │ is_default       │
│ started_at       │       │ created_at       │
│ completed_at     │       │ updated_at       │
└──────────────────┘       └──────────────────┘
```

**Key relationships:**
- `Source` 1→N `Document` — each document belongs to one input source
- `Document` 1→N `Highlight` — each highlight belongs to one document
- `Source` 1→N `SyncLog` — sync history per source
- `Document` has a unique constraint on `(source_id, source_document_id)` for deduplication
- `Highlight` has a unique constraint on `(document_id, source_highlight_id)` for deduplication
- `Document.type` is one of: `book`, `article`, `podcast`, `tweet` (extensible)
- JSON fields (`tags`, `metadata`) use GORM's JSON serializer for cross-DB compatibility
- `Template` stores rendering templates per document type (book, article, etc.)
- `Template.type` matches `Document.type` — each type has a default template

#### Template System

Templates use **Jinja2-compatible syntax** via [pongo2](https://github.com/flosch/pongo2) (Django/Jinja2 template engine for Go). This matches Readwise's template language so users familiar with Readwise can adapt their templates directly.

**Default Book Page Template:**
```
title:: {{ title }}
author:: [[{{ author }}]]
category:: #{{ category }}
source:: {{ source }}
cover:: {{ image_url }}
last_highlighted:: {{ last_highlighted_at|date:"Jan 2, 2006" }}
{% if tags %}tags:: {% for tag in tags %}#{{ tag }} {% endfor %}{% endif %}

- ## Highlights
{% for highlight in highlights %}
	- > {{ highlight.text }}
{% if highlight.note %}		- **Note:** {{ highlight.note }}
{% endif %}{% if highlight.color %}		- color:: {{ highlight.color }}
{% endif %}		- location:: {% if highlight.chapter %}{{ highlight.chapter }}{% endif %}{% if highlight.page_number %} (p. {{ highlight.page_number }}){% endif %}
		- date:: {{ highlight.highlighted_at|date:"Jan 2, 2006" }}
{% endfor %}
```

**Default Article Page Template:**
```
title:: {{ title }}
author:: {{ author }}
category:: #article
url:: {{ url }}
site:: {{ site_name }}
saved:: {{ created_at|date:"Jan 2, 2006" }}
{% if labels %}labels:: {% for label in labels %}#{{ label }} {% endfor %}{% endif %}

- ## Highlights
{% for highlight in highlights %}
	- > {{ highlight.text }}
{% if highlight.note %}		- **Note:** {{ highlight.note }}
{% endif %}		- date:: {{ highlight.highlighted_at|date:"Jan 2, 2006" }}
{% endfor %}
```

**Available Template Variables:**

| Variable | Book | Article | Description |
|----------|------|---------|-------------|
| `title` | ✓ | ✓ | Document title |
| `author` | ✓ | ✓ | Author name(s) |
| `category` | ✓ | ✓ | books / articles / podcasts / tweets |
| `url` | | ✓ | Original article URL |
| `source` | ✓ | ✓ | Source name (KOReader / Readeck) |
| `source_url` | | ✓ | URL in Readeck |
| `site_name` | | ✓ | Website name |
| `image_url` | ✓ | ✓ | Cover / thumbnail URL |
| `tags` | ✓ | ✓ | Array of tags |
| `labels` | | ✓ | Readeck labels |
| `num_highlights` | ✓ | ✓ | Highlight count |
| `last_highlighted_at` | ✓ | ✓ | Most recent highlight date |
| `created_at` | ✓ | ✓ | When first synced |
| `updated_at` | ✓ | ✓ | When last updated |
| `highlights` | ✓ | ✓ | Array of highlight objects |
| `metadata` | ✓ | ✓ | Extra metadata (isbn, asin, etc.) |

| Highlight Variable | Description |
|---|---|
| `highlight.text` | The highlighted text |
| `highlight.note` | User's note/annotation |
| `highlight.color` | Highlight color |
| `highlight.tags` | Highlight-level tags |
| `highlight.chapter` | Chapter title |
| `highlight.page_number` | Page number |
| `highlight.percentage` | Progress through document (0-100) |
| `highlight.location` | Raw location string |
| `highlight.highlighted_at` | When the highlight was created |

#### Configuration

All configuration is via **environment variables**. No config files required.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | No | `./marginalia.db` | SQLite file path or `postgres://` connection string |
| `MARGINALIA_API_TOKEN` | Yes | | Bearer token for API authentication |
| `MARGINALIA_PORT` | No | `8080` | HTTP listen port |
| `MARGINALIA_READECK_URL` | No | | Readeck instance URL (e.g. `https://readeck.example.com`) |
| `MARGINALIA_READECK_TOKEN` | No | | Readeck API token |

Additional source credentials follow the same pattern as more inputs are added (e.g. `MARGINALIA_<SOURCE>_URL`, `MARGINALIA_<SOURCE>_TOKEN`).

#### Deployment

Marginalia is distributed as a **Docker image** (`ghcr.io/adampetrovic/marginalia`) with semver tags.

```yaml
# docker-compose.yml
services:
  marginalia:
    image: ghcr.io/adampetrovic/marginalia:latest
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: /data/marginalia.db
      MARGINALIA_API_TOKEN: ${MARGINALIA_API_TOKEN}
      MARGINALIA_READECK_URL: https://readeck.example.com
      MARGINALIA_READECK_TOKEN: ${READECK_TOKEN}
    volumes:
      - marginalia-data:/data

volumes:
  marginalia-data:
```

Deploy however you like — Docker Compose, Kubernetes, bare metal. The service just needs a volume for the SQLite database (or a PostgreSQL connection string) and env vars for secrets.

### 2. Logseq Plugin (TypeScript)

A Logseq plugin that connects to the Marginalia service, triggers syncs, and writes highlight pages to the graph.

#### Plugin Settings (user-configurable in Logseq)

| Setting | Default | Description |
|---------|---------|-------------|
| `serviceUrl` | `https://marginalia.{domain}` | Marginalia API URL |
| `apiToken` | | Bearer token for authentication |
| `bookNamespace` | `Books` | Logseq namespace for book pages (e.g., `Books/Title`) |
| `articleNamespace` | `Articles` | Logseq namespace for article pages |
| `podcastNamespace` | `Podcasts` | Logseq namespace for podcast pages |
| `autoSync` | `false` | Enable periodic auto-sync |
| `autoSyncInterval` | `30` | Auto-sync interval in minutes |
| `syncOnStartup` | `true` | Sync when Logseq starts |
| `notifyOnSync` | `true` | Show notification after sync |

#### Plugin Features

1. **Toolbar sync button** — click to trigger manual sync
2. **Slash command** — `/marginalia sync` to trigger sync
3. **Settings panel** — configure service URL, token, namespaces
4. **Incremental sync** — tracks last sync timestamp, only fetches new/updated documents
5. **Merge strategy** — when a page exists:
   - Re-render the full page from the template (ensures template changes apply)
   - Preserve any user-added blocks that aren't part of the template output
   - Never delete highlights the user added manually
6. **Sync status** — shows last sync time and result in the plugin panel
7. **Conflict detection** — uses checksums from the API to skip unchanged documents

#### Sync Algorithm

```
1. Plugin calls POST /api/sync (triggers Marginalia to fetch from sources)
2. Plugin calls GET /api/export?since={last_sync_timestamp}
3. For each returned document:
   a. Compute target filename: {namespace}___{sanitized_title}.md
   b. Compare checksum to cached value — skip if unchanged
   c. If new: create page with rendered content from Marginalia
   d. If existing: merge — replace template-managed blocks, preserve user additions
4. Update last_sync_timestamp
5. Show notification: "Synced 3 books, 12 articles (47 new highlights)"
```

The plugin receives **pre-rendered markdown** from Marginalia's export API. It does not do any template rendering — that's Marginalia's responsibility. The plugin only handles Logseq-specific concerns: file naming, namespace prefixing (`Books/`, `Articles/`), page creation, and merge logic.

#### Tech Stack

- TypeScript + `@logseq/libs`
- Vite for bundling
- Published to Logseq Marketplace (optional) or installed manually

### 3. KOReader Exporter — Forked Readwise Target (Lua — minimal)

A fork of KOReader's built-in `readwise.lua` exporter target with one addition: a configurable server URL. The goal is to upstream this change to KOReader later so the official Readwise exporter supports custom endpoints natively.

#### Approach

1. **Fork** `plugins/exporter.koplugin/target/readwise.lua` from KOReader
2. **Add** a "Set server URL" menu item (stored in `self.settings.url`, defaults to `https://readwise.io`)
3. **Replace** the hardcoded URL with `self.settings.url .. "/api/v2/highlights"`
4. **Install** by replacing the original `readwise.lua` on the Kindle
5. **Upstream** — once validated, submit a PR to `koreader/koreader` adding custom URL support to the official readwise exporter

#### Changes from upstream readwise.lua (~10 lines)

```lua
-- ADD: menu item for server URL
{
    text = _("Set server URL"),
    keep_menu_open = true,
    callback = function()
        -- InputDialog for self.settings.url
        -- Default: "https://readwise.io"
    end
},

-- CHANGE: in createHighlights(), replace hardcoded URL
local base_url = self.settings.url or "https://readwise.io"
local result, err = self:makeJsonRequest(base_url .. "/api/v2/highlights", "POST",
     { highlights = highlights }, json_headers)
```

#### Data sent to Marginalia (Readwise-compatible format)

```json
{
  "highlights": [
    {
      "text": "The most rewarding things in life take years",
      "title": "How to Live: 27 Conflicting Answers",
      "author": "Derek Sivers",
      "source_type": "koreader",
      "category": "books",
      "note": "So true — applies to software too",
      "location": 42,
      "location_type": "order",
      "highlighted_at": "2025-03-15T14:30:00Z"
    }
  ]
}
```

#### Installation on Kindle

```
1. Connect Kindle via USB (one-time setup)
2. Navigate to: KOReader/plugins/exporter.koplugin/target/
3. Back up original readwise.lua
4. Replace with forked readwise.lua
5. In KOReader: Settings → Export Highlights → Readwise → Set server URL → enter Marginalia URL
6. Set token → enter Marginalia API token
7. Future exports go to Marginalia instead of Readwise
```

After the upstream PR is merged, users can switch back to the stock KOReader readwise.lua and just configure the URL — no custom file needed.

---

## Phase Plan

### Phase 1: Marginalia Service — Core + Readeck Integration
**Goal:** Docker image running, pulling highlights from Readeck, serving rendered export.

1. **Scaffold Go project** — Go module, GORM (SQLite + PostgreSQL), chi router, structured logging
2. **Database layer** — GORM models, auto-migration, JSON field serialization
3. **Env var configuration** — `DATABASE_URL`, `MARGINALIA_API_TOKEN`, source credentials
4. **Auth middleware** — Bearer token validation from `MARGINALIA_API_TOKEN`
5. **Readeck sync** — fetch bookmarks with annotations from Readeck API, store in DB
6. **Template engine** — pongo2 integration, default templates, rendering pipeline
7. **Export API** — `/api/export` endpoint returning rendered markdown
8. **Template management API** — CRUD for templates with preview
9. **Docker image** — multi-stage build, distroless base, semver tagging via CI
10. **Smoke test** — docker-compose up with Readeck, verify end-to-end sync

**Deliverables:** Published Docker image, Readeck highlights stored and renderable via API.

### Phase 2: Logseq Plugin
**Goal:** Highlights appear as pages in Logseq.

1. **Scaffold Logseq plugin** — TypeScript, Vite, @logseq/libs
2. **Settings panel** — service URL, token, namespaces, auto-sync toggle
3. **Sync engine** — call Marginalia API, track last sync timestamp
4. **Page writer** — create/update Logseq markdown pages from rendered export
5. **Merge logic** — preserve user additions, update template-managed content
6. **UI** — toolbar button, slash command, sync notifications
7. **Testing** — manual testing with real Readeck data in Logseq

**Deliverables:** Working Logseq plugin, Readeck article highlights syncing into Logseq graph.

### Phase 3: KOReader Integration
**Goal:** Book highlights flow wirelessly from Kindle to Logseq.

1. **Readwise-compatible API** — implement `POST /api/v2/highlights` and `GET /api/v2/auth` in Marginalia
2. **Fork readwise.lua** — add `self.settings.url` menu item, replace hardcoded URL (~10 lines changed)
3. **Install on Kindle** — one-time USB copy to replace `target/readwise.lua`
4. **Test end-to-end** — export highlights from KOReader over WiFi → verify in Marginalia → sync to Logseq
5. **Book deduplication** — handle same book exported multiple times (merge by title+author)
6. **Upstream PR** — submit PR to `koreader/koreader` to make URL configurable in the official exporter

**Deliverables:** KOReader highlights flowing to Marginalia → Logseq without plugging in the Kindle.

### Phase 4: Polish & Template Refinement
**Goal:** Feature parity with Readwise's template system.

1. **Web UI for templates** — embedded page to edit/preview templates (templ + htmx, single binary)
2. **Additional template variables** — word count, reading time, progress, etc.
3. **Template validation** — syntax checking, error messages on save
4. **Bulk re-render** — re-export all documents when a template changes
5. **Sync history** — view past syncs, errors, stats in the web UI

---

## Repository Structure

```
marginalia/                     # Monorepo
├── service/                    # Go service
│   ├── cmd/marginalia/         # main.go
│   ├── internal/
│   │   ├── api/                # HTTP handlers
│   │   ├── config/             # Env var configuration
│   │   ├── models/             # GORM models
│   │   ├── sync/               # Input source adapters (readeck, koreader)
│   │   └── render/             # Template rendering (pongo2)
│   ├── migrations/             # Versioned SQL migrations (goose, supports SQLite + PG)
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── logseq-plugin/              # Logseq plugin
│   ├── src/
│   │   ├── index.ts            # Plugin entry point
│   │   ├── api.ts              # Marginalia API client
│   │   ├── sync.ts             # Sync engine
│   │   ├── writer.ts           # Page writer/merger
│   │   └── settings.ts         # Plugin settings
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── koreader-plugin/            # Forked KOReader readwise exporter
│   └── readwise.lua            # Drop-in replacement with custom URL support
└── PROJECT_PLAN.md             # This file
```

---

## Open Decisions

### Settled
- [x] Language: **Go**
- [x] Database: **SQLite default, PostgreSQL supported** (via GORM ORM)
- [x] Configuration: **Environment variables** for all secrets and source credentials
- [x] Distribution: **Docker image** on GHCR with semver tags
- [x] KOReader approach: **Fork readwise.lua with configurable URL, upstream later**
- [x] Logseq integration: **Plugin (not direct file writing)**
- [x] Templates: **Stored in service DB, Jinja2-compatible (pongo2)** — service renders, plugins just write
- [x] Sync model: **On-demand (triggered by output plugin)**
- [x] Merge strategy: **Append-only (never delete highlights from Logseq)**
- [x] Priority: **Readeck → KOReader**
- [x] Architecture: **Pluggable input sources and output targets** — MVP: Readeck + KOReader in, Logseq out
- [x] Repository: **Monorepo** — service/, logseq-plugin/, koreader-plugin/ in one repo
- [x] Logseq graph: **Plugin detects via `logseq.App.getCurrentGraph()`** — works with any graph location
- [x] Readeck auth: **API token already configured** — passed via `MARGINALIA_READECK_TOKEN` env var

### To Decide

All previous open decisions have been resolved. See "Settled" above.

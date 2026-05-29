# Marginalia → Polished, Awesome, Readwise-grade

## Context

Marginalia today is a **well-architected read-only highlights pipeline**: KOReader
(push) + Readeck (pull) → SQLite/Postgres → server-rendered htmx UI → Logseq plugin,
with spaced-repetition review and a pongo2 template engine. The foundations are clean
(Go + chi + GORM, `service/internal/{models,api,sync,render,ui}`), but it stops short
of feeling like Readwise because **the highlight itself is inert**: you can't edit,
tag, favorite, or delete one from the UI or API; search is `LIKE`-only with no filters;
there's no dark mode; sync is manual; and favorites are buried inside `ReviewState`.

Goal (per direction): a **maximal, phased roadmap toward Readwise parity** — deep
polish of the core loop first, then breadth (new sources, Obsidian, export formats),
then multi-user as a final milestone. Each phase is independently shippable and ordered
by leverage. Phases 1–4 deliver the "polished and awesome" feel; 5–7 add breadth; 8 is
the architectural milestone.

Guiding principle (already the project's ethos, see `PROJECT_PLAN.md:111`): **rendering
and logic live in the service**; plugins stay thin. We extend that to editing — the
service is the source of truth and must *preserve user edits across re-sync*, mirroring
the Logseq plugin's note-preserving merge but server-side.

---

## Cross-cutting foundations (do alongside Phase 1)

- **Migrations**: schema changes go through `models.AutoMigrate` in
  `service/internal/models/database.go`. GORM AutoMigrate adds columns/indexes safely;
  for data backfills (e.g. moving `Favorite` off `ReviewState`) add a one-shot migration
  helper invoked from `cmd/marginalia/main.go` after AutoMigrate.
- **Pagination**: add a shared `?limit=&offset=` (default 50, max 200) helper in
  `service/internal/api/api.go`; replace hardcoded `Limit(100)`/`Limit(50)` in
  `handleListHighlights`, `handleListDocuments`, and `uiDashboard` (`ui.go`). Return
  `X-Total-Count` header.
- **Config file support**: `service/internal/config/config.go` is env-only. Add optional
  YAML config (`MARGINALIA_CONFIG=/data/config.yaml`) that env vars override, so
  per-source scheduling and SRS tuning (Phases 5/3) have a home. Keep all existing env
  vars working unchanged.

---

## Phase 1 — Highlights become first-class (edit / tag / favorite / delete)

**The headline gap.** Make documents and highlights mutable, with user edits protected
from sync clobbering.

### Data model (`service/internal/models/models.go`)
- Add to `Highlight`: `Favorite bool` (promote off `ReviewState`), `UserEdited bool`,
  `DeletedAt gorm.DeletedAt` (GORM soft-delete, auto-filters queries).
- Add to `Document`: `Favorite bool`, `DeletedAt gorm.DeletedAt`.
- Keep `ReviewState.Favorite` temporarily; backfill `Highlight.Favorite` from it, then
  drop it in a later cleanup. Update `review.go` reads to use `Highlight.Favorite`.

### Sync must not clobber edits (the correctness crux)
Current upserts overwrite user fields: `koreader.go:113` updates `text,note,tags`;
`readeck.go:219` updates `text`. Change both:
- When `UserEdited == true`, the `OnConflict.DoUpdates` column set must **exclude**
  `text, note, tags, color` — only refresh `synced_at, updated_at` and structural
  fields. Implement by selecting the conflict-update columns based on the existing row's
  `UserEdited` flag (do a lookup, or split into two upsert paths).
- Soft-deleted highlights must **not** resurrect on re-sync: with `gorm.DeletedAt`,
  add `.Unscoped()` lookups in sync to detect a tombstone and skip re-insert.

### API (`service/internal/api/api.go`, new `highlights.go`/`documents.go`)
New authenticated `/api/v1` routes:
- `GET /highlights/{id}`
- `PUT /highlights/{id}` — edits `text, note, color, tags`; sets `UserEdited=true`.
- `DELETE /highlights/{id}` — soft delete.
- `POST /highlights/{id}/favorite` (toggle).
- `PUT /documents/{id}` — edit `title, author, tags, favorite`.
- `DELETE /documents/{id}` — soft delete (cascades highlights).
- `DELETE /templates/{id}` (close the existing CRUD gap).

### UI (`service/internal/ui/templates/document.html`, `partials.html`, `ui.go`)
Reuse the existing htmx fragment pattern (`hx-post`/`hx-target` + `RenderPartial`):
- Per-highlight action row: **favorite** (★ toggle), **edit** (inline textarea for
  text/note via `hx-get` edit fragment → `hx-put` save → swap back to display
  fragment), **tags** (chip input), **color** swatch picker, **delete** (with confirm).
- New `ui.go` handlers: `uiHighlightEdit`, `uiHighlightUpdate`, `uiHighlightDelete`,
  `uiHighlightFavorite`, plus document-level edit. Each returns the re-rendered
  highlight/document partial.
- Document detail gets an editable header (title/author/tags) and a favorite toggle.

---

## Phase 2 — Browsing & discovery (search, filters, views)

Make the library actually navigable.

- **Full-text search**: add SQLite **FTS5** virtual table over highlight `text`+`note`
  and document `title`+`author` (Postgres path: `tsvector` + GIN). Keep a `LIKE`
  fallback for portability. New `GET /api/v1/search?q=` returning ranked highlights and
  documents. Replace the dual `LIKE` forms in `uiDashboard`.
- **Filters** on `/highlights`, `/documents`, and the dashboard: `?tags=`, `?color=`,
  `?favorite=true`, `?source=`, `?type=`, `?from=&to=` (date range), `?sort=`. Compose
  in GORM query builders.
- **Views/tabs** in `layout.html` nav: add **Favorites** and **Tags** (tag cloud →
  filtered list). Surface highlight **color** as a visible swatch in all lists
  (`document.html`, dashboard results) — the data exists but is never shown.
- **Pagination UI**: "Load more" (htmx `hx-get` append) on dashboard + document lists.

---

## Phase 3 — Review delight & SRS correctness (`service/internal/api/review.go`)

The review loop is functional but joyless and slightly wrong.

- **Keyboard shortcuts**: `1/2/3/4` = Again/Hard/Good/Easy, `f` favorite, `e` edit,
  `a` archive, `space`/`enter` next. Small vanilla JS in `review.html` posting the same
  forms (no framework — matches the htmx-only stack).
- **Initial scheduling fix**: new highlights currently get no `DueAt` until first
  review. In `scheduleReview` / card creation, seed `DueAt = now` (or `now+1d`) so the
  new-card queue is well-defined; document the SM-2 variant in a comment.
- **Daily goal + streak**: configurable daily review target (default 20). Add
  `reviewed_today / goal` progress bar and a streak counter to the review card stats
  (extend `reviewCardData`). Persist streak via a tiny `ReviewDay` table or derive from
  `LastReviewedAt` history.
- **Session/history**: `GET /api/v1/review/stats` (due/new/reviewed-today/streak) and
  `GET /api/v1/review/history?from=&to=`. Optional `bury` (skip for today) action
  alongside `archive`.
- **Animations**: subtle card cross-fade on swap (CSS transition on the htmx-swapped
  fragment) so reviewing feels fluid.

---

## Phase 4 — Visual polish & template editor

- **Dark mode**: `layout.html` already uses CSS variables (`--bg,--surface,--text,...`).
  Add a `[data-theme="dark"]` variable block + a header toggle (persist in
  `localStorage`, respect `prefers-color-scheme`). Pure CSS/JS, no rebuild.
- **Color chips** rendered consistently for highlight colors across UI; normalize the
  free-form `Color` string to a small palette on ingest.
- **Template editor** (`template_edit.html`): add CodeMirror (CDN, matching the htmx CDN
  approach) for syntax highlighting; ship 2–3 **preset templates**; fix the unused
  `Template.HighlightTemplate` field — either wire it into `render.RenderDocument`
  (`render/render.go`) as the per-highlight sub-template, or remove it to end the
  inconsistency. Add a **"re-render all"** action (deferred PROJECT_PLAN Phase 5 item).
- **Micro-polish**: empty-state illustrations, toast on save, sticky header.

---

## Phase 5 — Scheduled / automatic sync + robustness

Currently Readeck sync is manual (`POST /api/v1/sync`).

- **Background scheduler**: add `robfig/cron` (or a ticker goroutine) started in
  `cmd/marginalia/main.go`. Per-source interval from config (Phase 0 YAML), e.g. Readeck
  every 6h, RSS every 30m. Surface "next sync at" + last result in the UI.
- **Robustness**: per-item error isolation (one bad highlight shouldn't fail the whole
  sync — wrap the upsert loop), a sync mutex to prevent concurrent runs, and richer
  `SyncLog` (duration, new-vs-updated counts). Backfill these in `sync/*` and
  `handleSyncStatus`.

---

## Phase 6 — New ingestion sources

Generalize the source model so adapters are pluggable (the `Source` table already
supports it; formalize a `Syncer` interface in `service/internal/sync`).

- **Kindle "My Clippings.txt"** (cheapest, highest reach): `POST /api/v1/import/kindle`
  multipart upload + parser → documents/highlights. Add a small upload form in the UI.
- **RSS / newsletters**: feed config (URL list in YAML or a `Feed` table), a parser
  (`gofeed`), per-feed cursor (reuse `Source.LastSyncedAt`), scheduled via Phase 5. Full
  article text → one document; highlights optional (whole-article capture).
- **Pocket / Instapaper**: OAuth + API client per service, mapped into the same
  upsert path. Store per-source credentials (extend config / a `SourceConfig`).

Each new source reuses the dedup/upsert + user-edit-guard machinery from Phase 1.

---

## Phase 7 — Outputs & export

- **Export formats + UI**: extend `handleExport` (`api.go:87`). Add
  `GET /api/v1/export?format=markdown-zip|csv|json` (rendered Markdown bundle as a zip,
  flat CSV of highlights, raw JSON). Add **Export** buttons in the UI (dashboard +
  document detail). CSV/JSON unblock arbitrary downstream tools (the "public-ish API").
- **Obsidian plugin** (`obsidian-plugin/`, new): mirror the Logseq plugin
  (`logseq-plugin/src/{api,sync,writer,settings}.ts`). Reuse the **exact** rendered-page
  pull (`GET /api/v1/export?since=`) and the note-preserving **merge** strategy
  (`writer.ts`), adapted to Obsidian's vault/file API and frontmatter. Same incremental
  checksum-skip logic. This validates the "rendering lives in the service" design with a
  second consumer.

---

## Phase 8 — Multi-user (architectural milestone)

Deferred to last; touches everything.

- **Accounts & auth**: `User` table, password (argon2) or OIDC, session cookies; keep
  the existing single-token mode as "personal mode" for backward compat. Per-user API
  tokens replace the single shared `MARGINALIA_API_TOKEN`.
- **Data isolation**: add `UserID` FK to `Source`, `Document`, `Highlight`, `Template`,
  `ReviewState`; scope every query (a GORM scope/middleware injecting `user_id`).
- **Migration**: existing single-user data assigned to a default user. Plugins
  (Logseq/Obsidian/KOReader) authenticate with per-user tokens — no protocol change.

---

## Critical files

| Area | Files |
|---|---|
| Schema & migration | `service/internal/models/models.go`, `models/database.go` |
| API routes & handlers | `service/internal/api/api.go`, new `highlights.go`/`documents.go`, `review.go` |
| UI handlers | `service/internal/api/ui.go` |
| UI templates | `service/internal/ui/templates/{layout,document,partials,review,template_edit}.html`, `ui/templates.go` |
| Sync (edit-guard, scheduler, new sources) | `service/internal/sync/{readeck,koreader}/*.go`, new adapters |
| Render / templates | `service/internal/render/{render,sample}.go` |
| Config | `service/internal/config/config.go` |
| Entry / scheduler wiring | `service/cmd/marginalia/main.go` |
| Obsidian output | new `obsidian-plugin/` (model on `logseq-plugin/src/*`) |

## Verification

- **Unit/integration**: extend `service/internal/api/*_test.go` and
  `sync/*/*_test.go`. Critical new test: edit a highlight (`UserEdited=true`), re-run
  KOReader/Readeck sync with the same source data, assert the user's `text/note/tags`
  survive and a tombstoned highlight does **not** resurrect.
- **E2E**: extend `e2e/run-e2e.sh` — add cases for edit-then-resync preservation, soft
  delete, FTS search, favorite filter, and export formats (assert zip/CSV shape).
- **Manual** (`/run` skill): build (`cd service && CGO_ENABLED=1 go build ./...`), run
  with `MARGINALIA_API_TOKEN=dev`, walk the loop: ingest → edit/tag/favorite a highlight
  → search/filter → toggle dark mode → review with keyboard → export → confirm Logseq
  (and new Obsidian) sync preserves edits.
- **Per phase**: `cd service && go test ./...` stays green; each phase shippable alone.

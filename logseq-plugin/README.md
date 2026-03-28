# Marginalia Logseq Plugin

Sync highlights from [Marginalia](https://github.com/adampetrovic/marginalia) to your Logseq graph.

## Installation

1. Download `logseq-plugin-marginalia.zip` from the [latest release](https://github.com/adampetrovic/marginalia/releases)
2. In Logseq, go to **Settings → Plugins → Load unpacked plugin**
3. Select the extracted plugin folder

## Configuration

In Logseq plugin settings:

| Setting | Default | Description |
|---------|---------|-------------|
| Service URL | | Your Marginalia instance URL |
| API Token | | Bearer token for authentication |
| Book Namespace | `Books` | Logseq namespace for book pages |
| Article Namespace | `Articles` | Logseq namespace for article pages |
| Sync on Startup | `false` | Auto-sync when Logseq starts |
| Auto Sync | `false` | Periodically sync in the background |
| Auto Sync Interval | `30` | Minutes between auto-syncs |

## Usage

- Click the 📚 toolbar button to sync
- Or use the `/Marginalia Sync` slash command
- Highlights are written to pages under the configured namespaces (e.g. `Books/How to Live`)

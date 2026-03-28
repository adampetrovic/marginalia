# Marginalia KOReader Plugin

A fork of KOReader's built-in Readwise exporter with one addition: a **configurable server URL** so highlights can be sent to [Marginalia](https://github.com/adampetrovic/marginalia) or any Readwise-compatible API.

## Installation

1. Download `readwise.lua` from the [latest release](https://github.com/adampetrovic/marginalia/releases)
2. Connect your Kindle via USB
3. Back up the original file at `KOReader/plugins/exporter.koplugin/target/readwise.lua`
4. Replace it with the downloaded `readwise.lua`

## Configuration

In KOReader:

1. Go to **Settings → Export Highlights → Readwise**
2. **Set server URL** → enter your Marginalia instance URL (e.g. `https://marginalia.example.com`)
3. **Set authorization token** → enter your Marginalia API token
4. Enable **Export to Readwise**

Highlights will now be sent to your Marginalia instance instead of Readwise.

## Changes from upstream

- Added **"Set server URL"** menu item (defaults to `https://readwise.io`)
- Uses `self.settings.url` instead of the hardcoded Readwise URL
- Everything else is identical to [upstream readwise.lua](https://github.com/koreader/koreader/blob/master/plugins/exporter.koplugin/target/readwise.lua)

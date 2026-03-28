/**
 * Page writer / merger for Logseq.
 *
 * Responsible for:
 * - Sanitizing page titles for Logseq compatibility
 * - Computing the full page name (namespace/title)
 * - Parsing rendered markdown into page properties + content blocks
 * - Creating new pages from exported documents
 * - Merging updated content while preserving user-added blocks
 *
 * Merge strategy (from PROJECT_PLAN):
 *   1. Re-render the full page from the template (ensures template changes apply)
 *   2. Preserve any user-added blocks that aren't part of the template output
 *   3. Never delete highlights the user added manually
 *
 * Implementation:
 *   - The rendered content from Marginalia is split into two regions:
 *     (a) Page properties (lines matching `key:: value` at the top)
 *     (b) Content blocks (everything after the properties)
 *   - On update, page properties are always replaced.
 *   - The Highlights section (the `## Highlights` block and all its children)
 *     is replaced wholesale with the new rendered content.
 *   - Any top-level blocks the user added *outside* the Highlights section
 *     are preserved in their original position (before or after Highlights).
 */

import type { ExportedDocument } from "./api";

// ── Types ──────────────────────────────────────────────────────────

export interface ParsedContent {
  /** Page-level properties as key-value pairs. */
  properties: Record<string, string>;
  /** Raw property lines in original order (for first-block creation). */
  propertyLines: string[];
  /** Top-level content blocks parsed from the rendered markdown. */
  blocks: ParsedBlock[];
}

export interface ParsedBlock {
  /** Block content (without leading `- ` or tab indentation). */
  content: string;
  /** Nested child blocks. */
  children: ParsedBlock[];
}

export interface LogseqBlock {
  uuid: string;
  content: string;
  children?: LogseqBlock[];
}

/**
 * Minimal interface for the Logseq Editor API methods we use.
 * Lets us inject a mock in tests without pulling in the full @logseq/libs type.
 */
export interface LogseqEditorAPI {
  getPage(name: string): Promise<{ uuid: string; name: string } | null>;
  createPage(name: string, props?: Record<string, unknown>, opts?: { redirect?: boolean }): Promise<{ uuid: string } | null>;
  getPageBlocksTree(nameOrUUID: string): Promise<LogseqBlock[]>;
  insertBlock(parentUUID: string, content: string, opts?: { sibling?: boolean; isPageBlock?: boolean; before?: boolean }): Promise<{ uuid: string } | null>;
  updateBlock(uuid: string, content: string): Promise<void>;
  removeBlock(uuid: string): Promise<void>;
}

// ── Pure helpers (exported for testing) ────────────────────────────

/**
 * Characters that are invalid in Logseq page names.
 * Slashes are namespace separators, so we strip them from the title part.
 */
const INVALID_CHARS = /[/\\:*?"<>|#^\[\]{}]/g;

/** Remove characters that break Logseq page names. */
export function sanitizeTitle(title: string): string {
  return title.replace(INVALID_CHARS, "").replace(/\s+/g, " ").trim();
}

/** Map a document type to the user-configured namespace. */
export function getNamespaceForType(
  type: string,
  settings: { bookNamespace: string; articleNamespace: string; podcastNamespace: string },
): string {
  switch (type) {
    case "article":
      return settings.articleNamespace || "Articles";
    case "podcast":
      return settings.podcastNamespace || "Podcasts";
    default:
      return settings.bookNamespace || "Books";
  }
}

/** Build the full Logseq page name: `Namespace/Sanitized Title`. */
export function buildPageName(
  doc: Pick<ExportedDocument, "type" | "title">,
  settings: { bookNamespace: string; articleNamespace: string; podcastNamespace: string },
): string {
  const ns = getNamespaceForType(doc.type, settings);
  const title = sanitizeTitle(doc.title);
  return `${ns}/${title}`;
}

// ── Content parsing ────────────────────────────────────────────────

/**
 * Parse the rendered markdown from Marginalia into properties + block tree.
 *
 * The rendered content looks like:
 * ```
 * title:: How to Live
 * author:: [[Derek Sivers]]
 * category:: #book
 * <blank line>
 * - ## Highlights
 *     - > Some highlight
 *         - **Note:** annotation
 * ```
 */
export function parseRenderedContent(content: string): ParsedContent {
  const lines = content.split("\n");
  const properties: Record<string, string> = {};
  const propertyLines: string[] = [];
  let contentStart = 0;

  // Extract property lines from the top
  for (let i = 0; i < lines.length; i++) {
    const match = lines[i].match(/^(\w[\w\s_-]*)::(.*)$/);
    if (match) {
      const key = match[1].trim();
      const value = match[2].trim();
      properties[key] = value;
      propertyLines.push(lines[i]);
      contentStart = i + 1;
    } else if (lines[i].trim() === "") {
      // Skip blank lines between properties and content
      contentStart = i + 1;
    } else {
      break;
    }
  }

  // Parse remaining lines into a block tree
  const remaining = lines.slice(contentStart);
  const blocks = parseBlockTree(remaining);

  return { properties, propertyLines, blocks };
}

/**
 * Parse indented `- ` prefixed lines into a tree of blocks.
 *
 * Indentation is detected by counting leading tabs (or groups of spaces treated as one level).
 * A line like `\t- > highlight text` is depth=1, content=`> highlight text`.
 */
export function parseBlockTree(lines: string[]): ParsedBlock[] {
  const roots: ParsedBlock[] = [];
  const stack: { block: ParsedBlock; depth: number }[] = [];

  for (const line of lines) {
    // Skip empty lines
    if (line.trim() === "") continue;

    // Measure indentation: count leading tabs or 2/4-space groups
    const tabMatch = line.match(/^(\t*)/);
    const depth = tabMatch ? tabMatch[1].length : 0;

    // Strip leading tabs and the optional `- ` bullet
    let content = line.substring(depth);
    content = content.replace(/^- /, "");

    // Skip completely empty content after stripping
    if (content.trim() === "") continue;

    const block: ParsedBlock = { content: content.trimEnd(), children: [] };

    if (depth === 0) {
      roots.push(block);
      stack.length = 0;
      stack.push({ block, depth: 0 });
    } else {
      // Walk back the stack to find the parent at depth-1
      while (stack.length > 0 && stack[stack.length - 1].depth >= depth) {
        stack.pop();
      }
      if (stack.length > 0) {
        stack[stack.length - 1].block.children.push(block);
      } else {
        // Orphan — treat as root
        roots.push(block);
      }
      stack.push({ block, depth });
    }
  }

  return roots;
}

/** Check if a block is the Highlights section header. */
export function isHighlightsHeader(content: string): boolean {
  return /^##?\s*Highlights/i.test(content);
}

// ── Page writer ────────────────────────────────────────────────────

/**
 * Write an exported document to the Logseq graph.
 *
 * If the page doesn't exist, creates it with properties and content blocks.
 * If the page exists, replaces the Highlights section while preserving
 * user-added blocks outside it.
 */
export async function writeDocument(
  editor: LogseqEditorAPI,
  doc: ExportedDocument,
  settings: { bookNamespace: string; articleNamespace: string; podcastNamespace: string },
): Promise<void> {
  const pageName = buildPageName(doc, settings);
  const parsed = parseRenderedContent(doc.content);
  const existing = await editor.getPage(pageName);

  if (!existing) {
    await createNewPage(editor, pageName, parsed);
  } else {
    await mergePage(editor, pageName, parsed);
  }
}

// ── Internal: create page ──────────────────────────────────────────

async function createNewPage(
  editor: LogseqEditorAPI,
  pageName: string,
  parsed: ParsedContent,
): Promise<void> {
  // Create the page with properties
  await editor.createPage(pageName, parsed.properties, { redirect: false });

  const page = await editor.getPage(pageName);
  if (!page) return;

  // Insert content blocks
  await insertBlockTree(editor, page.uuid, parsed.blocks);
}

// ── Internal: merge page ───────────────────────────────────────────

async function mergePage(
  editor: LogseqEditorAPI,
  pageName: string,
  parsed: ParsedContent,
): Promise<void> {
  const page = await editor.getPage(pageName);
  if (!page) return;

  const existingBlocks = await editor.getPageBlocksTree(pageName);

  // 1) Update properties on the first block (or create one)
  //    In Logseq, the first block of a page typically holds properties.
  await updatePageProperties(editor, page.uuid, existingBlocks, parsed);

  // 2) Find and replace the Highlights section, preserving user blocks
  await replaceHighlightsSection(editor, page.uuid, existingBlocks, parsed.blocks);
}

/**
 * Update page properties.
 *
 * If the first block contains property lines (key:: value), update it.
 * Otherwise create a new property block at the top.
 */
async function updatePageProperties(
  editor: LogseqEditorAPI,
  pageUUID: string,
  existingBlocks: LogseqBlock[],
  parsed: ParsedContent,
): Promise<void> {
  if (parsed.propertyLines.length === 0) return;

  const propsContent = parsed.propertyLines.join("\n");

  // Check if first block has properties
  if (existingBlocks.length > 0 && hasProperties(existingBlocks[0].content)) {
    await editor.updateBlock(existingBlocks[0].uuid, propsContent);
  } else {
    // Insert property block at the top
    await editor.insertBlock(pageUUID, propsContent, {
      isPageBlock: true,
      before: true,
    });
  }
}

/** Check if block content contains Logseq property lines. */
function hasProperties(content: string): boolean {
  return content.split("\n").some((line) => /^\w[\w\s_-]*::/.test(line));
}

/**
 * Replace the Highlights section with new blocks while preserving user blocks.
 *
 * Walk the existing top-level blocks:
 *   - Blocks *before* the Highlights header → keep
 *   - The Highlights header + its children → remove, then insert new content
 *   - Blocks *after* the Highlights section → keep
 *
 * If no Highlights section exists, append the new blocks at the end.
 */
async function replaceHighlightsSection(
  editor: LogseqEditorAPI,
  pageUUID: string,
  existingBlocks: LogseqBlock[],
  newBlocks: ParsedBlock[],
): Promise<void> {
  // Skip property block (index 0) when scanning for highlights
  const contentBlocks = existingBlocks.length > 0 && hasProperties(existingBlocks[0].content) ? existingBlocks.slice(1) : existingBlocks;

  // Find the Highlights header among content blocks
  let highlightsIndex = -1;
  for (let i = 0; i < contentBlocks.length; i++) {
    if (isHighlightsHeader(contentBlocks[i].content)) {
      highlightsIndex = i;
      break;
    }
  }

  if (highlightsIndex === -1) {
    // No existing Highlights section — append new blocks at the end
    await insertBlockTree(editor, pageUUID, newBlocks);
    return;
  }

  // Remove the old Highlights block (and all its children via Logseq's tree removal)
  await editor.removeBlock(contentBlocks[highlightsIndex].uuid);

  // Insert new blocks. We need to insert them before the "after" blocks.
  // If there are blocks after the highlights section, insert before the first one.
  const afterBlocks = contentBlocks.slice(highlightsIndex + 1);

  if (afterBlocks.length > 0) {
    // Insert new blocks before the first "after" block
    await insertBlockTreeBefore(editor, pageUUID, afterBlocks[0].uuid, newBlocks);
  } else {
    // Append at the end
    await insertBlockTree(editor, pageUUID, newBlocks);
  }
}

// ── Internal: block insertion ──────────────────────────────────────

/**
 * Insert a tree of parsed blocks as children of a parent.
 */
async function insertBlockTree(
  editor: LogseqEditorAPI,
  parentUUID: string,
  blocks: ParsedBlock[],
): Promise<void> {
  for (const block of blocks) {
    const inserted = await editor.insertBlock(parentUUID, block.content, {
      sibling: false,
    });
    if (inserted && block.children.length > 0) {
      await insertBlockTree(editor, inserted.uuid, block.children);
    }
  }
}

/**
 * Insert blocks before a specific sibling block.
 */
async function insertBlockTreeBefore(
  editor: LogseqEditorAPI,
  parentUUID: string,
  beforeUUID: string,
  blocks: ParsedBlock[],
): Promise<void> {
  for (const block of blocks) {
    const inserted = await editor.insertBlock(beforeUUID, block.content, {
      before: true,
      sibling: true,
    });
    if (inserted && block.children.length > 0) {
      await insertBlockTree(editor, inserted.uuid, block.children);
    }
  }
}

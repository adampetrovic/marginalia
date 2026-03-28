import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  sanitizeTitle,
  getNamespaceForType,
  buildPageName,
  parseRenderedContent,
  parseBlockTree,
  isHighlightsHeader,
  writeDocument,
  type LogseqEditorAPI,
  type LogseqBlock,
} from "./writer";
import type { ExportedDocument } from "./api";

// ── Pure function tests ────────────────────────────────────────────

describe("sanitizeTitle", () => {
  it("removes invalid characters", () => {
    expect(sanitizeTitle('How to Live: 27 Answers')).toBe("How to Live 27 Answers");
  });

  it("removes hash, brackets, pipes", () => {
    expect(sanitizeTitle("Title #1 [draft] {v2} | final")).toBe("Title 1 draft v2 final");
  });

  it("collapses multiple spaces", () => {
    expect(sanitizeTitle("Too   many   spaces")).toBe("Too many spaces");
  });

  it("trims whitespace", () => {
    expect(sanitizeTitle("  Title  ")).toBe("Title");
  });

  it("handles empty string", () => {
    expect(sanitizeTitle("")).toBe("");
  });

  it("preserves unicode", () => {
    expect(sanitizeTitle("Über Bücher — ein Überblick")).toBe("Über Bücher — ein Überblick");
  });

  it("removes slashes and backslashes", () => {
    expect(sanitizeTitle("Path/To\\Book")).toBe("PathToBook");
  });
});

describe("getNamespaceForType", () => {
  const settings = {
    bookNamespace: "Books",
    articleNamespace: "Articles",
    podcastNamespace: "Podcasts",
  };

  it("returns Books for book type", () => {
    expect(getNamespaceForType("book", settings)).toBe("Books");
  });

  it("returns Articles for article type", () => {
    expect(getNamespaceForType("article", settings)).toBe("Articles");
  });

  it("returns Podcasts for podcast type", () => {
    expect(getNamespaceForType("podcast", settings)).toBe("Podcasts");
  });

  it("defaults to Books for unknown type", () => {
    expect(getNamespaceForType("tweet", settings)).toBe("Books");
  });

  it("falls back to defaults when settings are empty", () => {
    const empty = { bookNamespace: "", articleNamespace: "", podcastNamespace: "" };
    expect(getNamespaceForType("article", empty)).toBe("Articles");
    expect(getNamespaceForType("book", empty)).toBe("Books");
    expect(getNamespaceForType("podcast", empty)).toBe("Podcasts");
  });
});

describe("buildPageName", () => {
  const settings = {
    bookNamespace: "Books",
    articleNamespace: "Articles",
    podcastNamespace: "Podcasts",
  };

  it("combines namespace and sanitized title", () => {
    expect(
      buildPageName({ type: "book", title: "How to Live: 27 Answers" }, settings),
    ).toBe("Books/How to Live 27 Answers");
  });

  it("uses article namespace for articles", () => {
    expect(
      buildPageName({ type: "article", title: "Good Article" }, settings),
    ).toBe("Articles/Good Article");
  });
});

// ── Content parsing tests ──────────────────────────────────────────

describe("parseRenderedContent", () => {
  it("extracts properties from the top", () => {
    const content = `title:: How to Live
author:: [[Derek Sivers]]
category:: #book

- ## Highlights
\t- > Some text`;

    const parsed = parseRenderedContent(content);

    expect(parsed.properties).toEqual({
      title: "How to Live",
      author: "[[Derek Sivers]]",
      category: "#book",
    });
    expect(parsed.propertyLines).toEqual([
      "title:: How to Live",
      "author:: [[Derek Sivers]]",
      "category:: #book",
    ]);
  });

  it("parses content blocks after properties", () => {
    const content = `title:: Test
author:: Author

- ## Highlights
\t- > First highlight
\t- > Second highlight`;

    const parsed = parseRenderedContent(content);

    expect(parsed.blocks).toHaveLength(1);
    expect(parsed.blocks[0].content).toBe("## Highlights");
    expect(parsed.blocks[0].children).toHaveLength(2);
    expect(parsed.blocks[0].children[0].content).toBe("> First highlight");
    expect(parsed.blocks[0].children[1].content).toBe("> Second highlight");
  });

  it("handles content with no properties", () => {
    const content = `- ## Highlights
\t- > Only highlights`;

    const parsed = parseRenderedContent(content);

    expect(parsed.properties).toEqual({});
    expect(parsed.blocks).toHaveLength(1);
  });

  it("handles content with only properties", () => {
    const content = `title:: Test
author:: Someone`;

    const parsed = parseRenderedContent(content);

    expect(parsed.properties).toEqual({ title: "Test", author: "Someone" });
    expect(parsed.blocks).toHaveLength(0);
  });

  it("handles multi-word property keys", () => {
    const content = `last_highlighted:: Jan 1, 2025`;
    const parsed = parseRenderedContent(content);
    expect(parsed.properties["last_highlighted"]).toBe("Jan 1, 2025");
  });
});

describe("parseBlockTree", () => {
  it("parses flat blocks", () => {
    const lines = ["- Block 1", "- Block 2", "- Block 3"];
    const blocks = parseBlockTree(lines);

    expect(blocks).toHaveLength(3);
    expect(blocks[0].content).toBe("Block 1");
    expect(blocks[1].content).toBe("Block 2");
    expect(blocks[2].content).toBe("Block 3");
  });

  it("parses nested blocks", () => {
    const lines = [
      "- ## Highlights",
      "\t- > First highlight text",
      "\t\t- **Note:** My annotation",
      "\t\t- color:: yellow",
      "\t- > Second highlight text",
    ];
    const blocks = parseBlockTree(lines);

    expect(blocks).toHaveLength(1);
    expect(blocks[0].content).toBe("## Highlights");
    expect(blocks[0].children).toHaveLength(2);

    const first = blocks[0].children[0];
    expect(first.content).toBe("> First highlight text");
    expect(first.children).toHaveLength(2);
    expect(first.children[0].content).toBe("**Note:** My annotation");
    expect(first.children[1].content).toBe("color:: yellow");

    const second = blocks[0].children[1];
    expect(second.content).toBe("> Second highlight text");
    expect(second.children).toHaveLength(0);
  });

  it("skips empty lines", () => {
    const lines = ["- Block 1", "", "- Block 2"];
    const blocks = parseBlockTree(lines);
    expect(blocks).toHaveLength(2);
  });

  it("handles deeply nested blocks", () => {
    const lines = [
      "- Level 0",
      "\t- Level 1",
      "\t\t- Level 2",
      "\t\t\t- Level 3",
    ];
    const blocks = parseBlockTree(lines);

    expect(blocks).toHaveLength(1);
    expect(blocks[0].children[0].children[0].children[0].content).toBe("Level 3");
  });
});

describe("isHighlightsHeader", () => {
  it("matches ## Highlights", () => {
    expect(isHighlightsHeader("## Highlights")).toBe(true);
  });

  it("matches # Highlights", () => {
    expect(isHighlightsHeader("# Highlights")).toBe(true);
  });

  it("is case-insensitive", () => {
    expect(isHighlightsHeader("## highlights")).toBe(true);
    expect(isHighlightsHeader("## HIGHLIGHTS")).toBe(true);
  });

  it("does not match random text", () => {
    expect(isHighlightsHeader("Some other block")).toBe(false);
    expect(isHighlightsHeader("")).toBe(false);
  });
});

// ── writeDocument integration tests (with mock editor) ─────────────

describe("writeDocument", () => {
  const settings = {
    bookNamespace: "Books",
    articleNamespace: "Articles",
    podcastNamespace: "Podcasts",
  };

  function makeDoc(overrides?: Partial<ExportedDocument>): ExportedDocument {
    return {
      id: "doc-1",
      type: "book",
      title: "How to Live",
      author: "Derek Sivers",
      content: `title:: How to Live
author:: [[Derek Sivers]]
category:: #book

- ## Highlights
\t- > The most rewarding things take years
\t\t- **Note:** So true
\t- > Be bold`,
      updated_at: "2025-01-01T00:00:00Z",
      highlight_count: 2,
      checksum: "sha256:abc",
      ...overrides,
    };
  }

  let editor: LogseqEditorAPI;
  let pages: Map<string, { uuid: string; blocks: LogseqBlock[] }>;
  let nextUUID: number;

  beforeEach(() => {
    pages = new Map();
    nextUUID = 1;

    const genUUID = () => `uuid-${nextUUID++}`;

    editor = {
      getPage: vi.fn(async (name: string) => {
        const page = pages.get(name);
        return page ? { uuid: page.uuid, name } : null;
      }),

      createPage: vi.fn(async (name: string, _props?: Record<string, unknown>) => {
        const uuid = genUUID();
        pages.set(name, { uuid, blocks: [] });
        return { uuid };
      }),

      getPageBlocksTree: vi.fn(async (name: string) => {
        return pages.get(name)?.blocks ?? [];
      }),

      insertBlock: vi.fn(async (_parent: string, content: string) => {
        const uuid = genUUID();
        return { uuid };
      }),

      updateBlock: vi.fn(async () => {}),
      removeBlock: vi.fn(async () => {}),
    };
  });

  it("creates a new page with properties and blocks", async () => {
    await writeDocument(editor, makeDoc(), settings);

    expect(editor.createPage).toHaveBeenCalledWith(
      "Books/How to Live",
      {
        title: "How to Live",
        author: "[[Derek Sivers]]",
        category: "#book",
      },
      { redirect: false },
    );
    // Should insert blocks for highlights
    expect(editor.insertBlock).toHaveBeenCalled();
  });

  it("uses article namespace for articles", async () => {
    await writeDocument(editor, makeDoc({ type: "article", title: "Good Read" }), settings);

    expect(editor.createPage).toHaveBeenCalledWith(
      "Articles/Good Read",
      expect.anything(),
      expect.anything(),
    );
  });

  it("sanitizes page title", async () => {
    await writeDocument(editor, makeDoc({ title: 'Bad: Title "Here"' }), settings);

    expect(editor.createPage).toHaveBeenCalledWith(
      "Books/Bad Title Here",
      expect.anything(),
      expect.anything(),
    );
  });

  describe("merge logic", () => {
    it("updates existing page — replaces properties and highlights", async () => {
      // Set up existing page with a highlights block and a user block
      const pageUUID = "existing-page-uuid";
      pages.set("Books/How to Live", {
        uuid: pageUUID,
        blocks: [
          {
            uuid: "props-block",
            content: "title:: How to Live\nauthor:: [[Derek Sivers]]",
            children: [],
          },
          {
            uuid: "highlights-block",
            content: "## Highlights",
            children: [
              { uuid: "hl-1", content: "> Old highlight", children: [] },
            ],
          },
          {
            uuid: "user-block",
            content: "My personal notes about this book",
            children: [],
          },
        ],
      });

      await writeDocument(editor, makeDoc(), settings);

      // Should NOT create a new page
      expect(editor.createPage).not.toHaveBeenCalled();

      // Should update properties block
      expect(editor.updateBlock).toHaveBeenCalledWith(
        "props-block",
        "title:: How to Live\nauthor:: [[Derek Sivers]]\ncategory:: #book",
      );

      // Should remove old highlights block
      expect(editor.removeBlock).toHaveBeenCalledWith("highlights-block");

      // Should NOT remove the user block
      expect(editor.removeBlock).not.toHaveBeenCalledWith("user-block");
    });

    it("appends highlights when no existing highlights section", async () => {
      const pageUUID = "existing-page-uuid";
      pages.set("Books/How to Live", {
        uuid: pageUUID,
        blocks: [
          {
            uuid: "props-block",
            content: "title:: How to Live\nauthor:: [[Derek Sivers]]",
            children: [],
          },
          {
            uuid: "user-block",
            content: "Just a user note, no highlights yet",
            children: [],
          },
        ],
      });

      await writeDocument(editor, makeDoc(), settings);

      // Should NOT remove the user block
      expect(editor.removeBlock).not.toHaveBeenCalled();

      // Should insert new highlight blocks
      expect(editor.insertBlock).toHaveBeenCalled();
    });
  });
});

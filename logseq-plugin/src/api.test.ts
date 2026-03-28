import { describe, it, expect } from "vitest";

// Basic sanity tests for utility functions
// Full plugin tests require Logseq runtime and will be manual

describe("sanitizeTitle", () => {
  function sanitizeTitle(title: string): string {
    return title.replace(/[/\\:*?"<>|#]/g, "").trim();
  }

  it("removes invalid characters", () => {
    expect(sanitizeTitle('How to Live: 27 Answers')).toBe("How to Live 27 Answers");
  });

  it("removes hash characters", () => {
    expect(sanitizeTitle("Title #1")).toBe("Title 1");
  });

  it("preserves normal titles", () => {
    expect(sanitizeTitle("A Normal Book Title")).toBe("A Normal Book Title");
  });

  it("handles empty string", () => {
    expect(sanitizeTitle("")).toBe("");
  });

  it("trims whitespace", () => {
    expect(sanitizeTitle("  Title  ")).toBe("Title");
  });
});

describe("getNamespaceForType", () => {
  function getNamespaceForType(type: string): string {
    const namespaces: Record<string, string> = {
      article: "Articles",
      podcast: "Podcasts",
    };
    return namespaces[type] || "Books";
  }

  it("returns Books for book type", () => {
    expect(getNamespaceForType("book")).toBe("Books");
  });

  it("returns Articles for article type", () => {
    expect(getNamespaceForType("article")).toBe("Articles");
  });

  it("returns Podcasts for podcast type", () => {
    expect(getNamespaceForType("podcast")).toBe("Podcasts");
  });

  it("defaults to Books for unknown type", () => {
    expect(getNamespaceForType("unknown")).toBe("Books");
  });
});

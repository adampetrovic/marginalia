package koreader

import (
	"testing"
)

func TestParseNote(t *testing.T) {
	tests := []struct {
		input        string
		wantNote     string
		wantConcat   int
		wantHeading  int
	}{
		{"just a note", "just a note", 0, 0},
		{"", "", 0, 0},
		{".c1", "", 1, 0},
		{".c3", "", 3, 0},
		{"good point .c1", "good point", 1, 0},
		{".c2 start of note", "start of note", 2, 0},
		{".h1", "", 0, 1},
		{".h3", "", 0, 3},
		{"chapter title .h2", "chapter title", 0, 2},
		{".c1 .h1", "", 1, 1},
		{"my note .c2 .h3", "my note", 2, 3},
		{"no shortcuts here.c1nope", "no shortcuts here.c1nope", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseNote(tt.input)
			if got.Note != tt.wantNote {
				t.Errorf("Note = %q, want %q", got.Note, tt.wantNote)
			}
			if got.ConcatGroup != tt.wantConcat {
				t.Errorf("ConcatGroup = %d, want %d", got.ConcatGroup, tt.wantConcat)
			}
			if got.HeadingLevel != tt.wantHeading {
				t.Errorf("HeadingLevel = %d, want %d", got.HeadingLevel, tt.wantHeading)
			}
		})
	}
}

func TestProcessShortcuts_NoShortcuts(t *testing.T) {
	hls := []ReadwiseHighlight{
		{Text: "first", Note: "note one", Location: 10, LocationType: "order"},
		{Text: "second", Note: "", Location: 20, LocationType: "order"},
	}

	result := ProcessShortcuts(hls)
	if len(result) != 2 {
		t.Fatalf("expected 2 highlights, got %d", len(result))
	}
	if result[0].Text != "first" {
		t.Errorf("expected 'first', got %q", result[0].Text)
	}
	if result[0].HeadingLevel != 0 {
		t.Errorf("expected heading 0, got %d", result[0].HeadingLevel)
	}
}

func TestProcessShortcuts_Concatenation(t *testing.T) {
	hls := []ReadwiseHighlight{
		{Text: "Part one of the quote", Note: ".c1", Location: 10, LocationType: "order"},
		{Text: "and part two continues", Note: ".c1", Location: 11, LocationType: "order"},
		{Text: "unrelated highlight", Note: "", Location: 20, LocationType: "order"},
	}

	result := ProcessShortcuts(hls)
	if len(result) != 2 {
		t.Fatalf("expected 2 highlights (1 merged + 1 solo), got %d", len(result))
	}

	// Find the merged one (location 10)
	var merged *ProcessedHighlight
	for i := range result {
		if result[i].Location == 10 {
			merged = &result[i]
			break
		}
	}
	if merged == nil {
		t.Fatal("merged highlight not found")
	}

	expected := "Part one of the quote and part two continues"
	if merged.Text != expected {
		t.Errorf("merged text = %q, want %q", merged.Text, expected)
	}
	if merged.Note != "" {
		t.Errorf("merged note should be empty, got %q", merged.Note)
	}
}

func TestProcessShortcuts_ConcatWithNotes(t *testing.T) {
	hls := []ReadwiseHighlight{
		{Text: "Part A", Note: "important .c1", Location: 5, LocationType: "order"},
		{Text: "Part B", Note: ".c1", Location: 6, LocationType: "order"},
	}

	result := ProcessShortcuts(hls)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged highlight, got %d", len(result))
	}
	if result[0].Text != "Part A Part B" {
		t.Errorf("text = %q", result[0].Text)
	}
	if result[0].Note != "important" {
		t.Errorf("note = %q, want 'important'", result[0].Note)
	}
}

func TestProcessShortcuts_MultipleConcatGroups(t *testing.T) {
	hls := []ReadwiseHighlight{
		{Text: "Group 1 part A", Note: ".c1", Location: 1, LocationType: "order"},
		{Text: "Group 2 part A", Note: ".c2", Location: 2, LocationType: "order"},
		{Text: "Group 1 part B", Note: ".c1", Location: 3, LocationType: "order"},
		{Text: "Group 2 part B", Note: ".c2", Location: 4, LocationType: "order"},
	}

	result := ProcessShortcuts(hls)
	if len(result) != 2 {
		t.Fatalf("expected 2 merged highlights, got %d", len(result))
	}

	// Results sorted by location — group 1 (loc 1) before group 2 (loc 2)
	if result[0].Text != "Group 1 part A Group 1 part B" {
		t.Errorf("group 1 text = %q", result[0].Text)
	}
	if result[1].Text != "Group 2 part A Group 2 part B" {
		t.Errorf("group 2 text = %q", result[1].Text)
	}
}

func TestProcessShortcuts_Headings(t *testing.T) {
	hls := []ReadwiseHighlight{
		{Text: "Chapter One", Note: ".h1", Location: 1, LocationType: "order"},
		{Text: "A regular highlight", Note: "", Location: 5, LocationType: "order"},
		{Text: "Section Two", Note: "subtitle .h2", Location: 10, LocationType: "order"},
	}

	result := ProcessShortcuts(hls)
	if len(result) != 3 {
		t.Fatalf("expected 3 highlights, got %d", len(result))
	}

	if result[0].HeadingLevel != 1 {
		t.Errorf("first highlight heading = %d, want 1", result[0].HeadingLevel)
	}
	if result[0].Note != "" {
		t.Errorf("first highlight note = %q, want empty", result[0].Note)
	}

	if result[1].HeadingLevel != 0 {
		t.Errorf("second highlight heading = %d, want 0", result[1].HeadingLevel)
	}

	if result[2].HeadingLevel != 2 {
		t.Errorf("third highlight heading = %d, want 2", result[2].HeadingLevel)
	}
	if result[2].Note != "subtitle" {
		t.Errorf("third highlight note = %q, want 'subtitle'", result[2].Note)
	}
}

func TestProcessShortcuts_ConcatWithHeading(t *testing.T) {
	hls := []ReadwiseHighlight{
		{Text: "Long chapter", Note: ".c1 .h1", Location: 1, LocationType: "order"},
		{Text: "title continues", Note: ".c1", Location: 2, LocationType: "order"},
	}

	result := ProcessShortcuts(hls)
	if len(result) != 1 {
		t.Fatalf("expected 1 highlight, got %d", len(result))
	}
	if result[0].Text != "Long chapter title continues" {
		t.Errorf("text = %q", result[0].Text)
	}
	if result[0].HeadingLevel != 1 {
		t.Errorf("heading = %d, want 1", result[0].HeadingLevel)
	}
}

func TestProcessShortcuts_LocationOrder(t *testing.T) {
	// Out of order input — should sort by location
	hls := []ReadwiseHighlight{
		{Text: "second", Note: ".c1", Location: 20, LocationType: "order"},
		{Text: "first", Note: ".c1", Location: 10, LocationType: "order"},
	}

	result := ProcessShortcuts(hls)
	if len(result) != 1 {
		t.Fatalf("expected 1 highlight, got %d", len(result))
	}
	if result[0].Text != "first second" {
		t.Errorf("text = %q, want 'first second'", result[0].Text)
	}
}

func TestHeadingTag(t *testing.T) {
	tests := []struct {
		level int
		want  string
	}{
		{0, ""}, {1, "h1"}, {3, "h3"}, {5, "h5"}, {6, ""}, {-1, ""},
	}
	for _, tt := range tests {
		if got := HeadingTag(tt.level); got != tt.want {
			t.Errorf("HeadingTag(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

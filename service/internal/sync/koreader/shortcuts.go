package koreader

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Readwise-compatible note shortcuts:
//   .c1–.c5  — concatenation group (merge highlights sharing the same group)
//   .h1–.h5  — heading level (highlight text is a section header)

var (
	// Match .c1-.c5 and .h1-.h5 as whole "words" — preceded by space/start, followed by space/end.
	concatRe  = regexp.MustCompile(`(?:^|\s)(\.c([1-5]))(?:\s|$)`)
	headingRe = regexp.MustCompile(`(?:^|\s)(\.h([1-5]))(?:\s|$)`)
	// Simpler cleanup: match the literal shortcode tokens to remove them.
	shortcodeCleanRe = regexp.MustCompile(`\s*\.[ch][1-5](?:\s|$)`)
)

// ParsedNote holds the cleaned note text and any parsed shortcodes.
type ParsedNote struct {
	// Note is the cleaned text with shortcodes removed.
	Note string
	// ConcatGroup is 0 if no concat shortcode, or 1–5.
	ConcatGroup int
	// HeadingLevel is 0 if no heading shortcode, or 1–5.
	HeadingLevel int
}

// ParseNote extracts shortcodes from a note string.
func ParseNote(note string) ParsedNote {
	p := ParsedNote{}

	if m := concatRe.FindStringSubmatch(note); m != nil {
		p.ConcatGroup = int(m[2][0] - '0')
	}

	if m := headingRe.FindStringSubmatch(note); m != nil {
		p.HeadingLevel = int(m[2][0] - '0')
	}

	// Strip all shortcode tokens from the note
	cleaned := shortcodeCleanRe.ReplaceAllString(note, " ")
	p.Note = strings.TrimSpace(cleaned)
	return p
}

// ProcessedHighlight is a ReadwiseHighlight with shortcodes applied.
type ProcessedHighlight struct {
	ReadwiseHighlight
	// HeadingLevel is 0 for normal highlights, 1–5 for headings.
	HeadingLevel int
	// SourceIDKey is a stable identifier for deduplication.
	// For regular highlights: "loc:{location}".
	// For concat groups: "concat:{group}:{loc1},{loc2},...".
	SourceIDKey string
}

// ProcessShortcuts takes a flat list of highlights for a single document,
// parses note shortcuts, merges concatenation groups, and returns
// processed highlights ready for storage.
//
// Concatenation: all highlights with the same .cN are merged into one,
// texts joined by " " in location order.
//
// Headings: .hN is parsed and stored as HeadingLevel; the shortcode is
// stripped from the note.
func ProcessShortcuts(highlights []ReadwiseHighlight) []ProcessedHighlight {
	type parsed struct {
		hl   ReadwiseHighlight
		note ParsedNote
	}

	var ungrouped []parsed
	groups := make(map[int][]parsed) // concatGroup → highlights

	for _, hl := range highlights {
		pn := ParseNote(hl.Note)
		p := parsed{hl: hl, note: pn}
		if pn.ConcatGroup > 0 {
			groups[pn.ConcatGroup] = append(groups[pn.ConcatGroup], p)
		} else {
			ungrouped = append(ungrouped, p)
		}
	}

	var result []ProcessedHighlight

	// Emit ungrouped highlights
	for _, p := range ungrouped {
		hl := p.hl
		hl.Note = p.note.Note
		result = append(result, ProcessedHighlight{
			ReadwiseHighlight: hl,
			HeadingLevel:      p.note.HeadingLevel,
			SourceIDKey:       fmt.Sprintf("loc:%d", hl.Location),
		})
	}

	// Merge each concat group
	for groupNum, members := range groups {
		sort.Slice(members, func(i, j int) bool {
			return members[i].hl.Location < members[j].hl.Location
		})

		var texts []string
		var notes []string
		var headingLevel int
		for _, m := range members {
			texts = append(texts, m.hl.Text)
			if m.note.Note != "" {
				notes = append(notes, m.note.Note)
			}
			if m.note.HeadingLevel > 0 {
				headingLevel = m.note.HeadingLevel
			}
		}

		// Stable source ID: based on group number, not text content.
		// This way adding a new highlight to the group still matches the existing record.
		sourceKey := fmt.Sprintf("concat:%d", groupNum)

		merged := members[0].hl
		merged.Text = strings.Join(texts, " ")
		merged.Note = strings.Join(notes, " ")
		merged.Location = members[0].hl.Location
		result = append(result, ProcessedHighlight{
			ReadwiseHighlight: merged,
			HeadingLevel:      headingLevel,
			SourceIDKey:       sourceKey,
		})
	}

	// Sort by location
	sort.Slice(result, func(i, j int) bool {
		return result[i].Location < result[j].Location
	})

	return result
}

// HeadingTag returns the tag string for a heading level (e.g. "h1", "h2").
// Returns empty string for level 0.
func HeadingTag(level int) string {
	if level < 1 || level > 5 {
		return ""
	}
	return fmt.Sprintf("h%d", level)
}

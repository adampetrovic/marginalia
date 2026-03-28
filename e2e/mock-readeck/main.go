// Mock Readeck server for e2e testing.
// Returns fixture bookmarks and annotations that exercise
// the full Marginalia sync pipeline.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// Fixture data — two bookmarks, each with annotations.

type Resource struct {
	Src string `json:"src"`
}

type Bookmark struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Title     string   `json:"title"`
	SiteName  string   `json:"site_name"`
	Authors   []string `json:"authors"`
	Labels    []string `json:"labels"`
	Created   string   `json:"created"`
	Updated   string   `json:"updated"`
	Type      string   `json:"type"`
	Resources struct {
		Image     *Resource `json:"image"`
		Thumbnail *Resource `json:"thumbnail"`
	} `json:"resources"`
}

type Annotation struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Created string `json:"created"`
}

var bookmarks = []Bookmark{
	{
		ID:       "bm-001",
		URL:      "https://example.com/article-one",
		Title:    "The Art of Focused Reading",
		SiteName: "Example Blog",
		Authors:  []string{"Alice Writer"},
		Labels:   []string{"reading", "focus"},
		Created:  "2025-03-01T10:00:00Z",
		Updated:  "2025-03-01T10:00:00Z",
		Type:     "article",
		Resources: struct {
			Image     *Resource `json:"image"`
			Thumbnail *Resource `json:"thumbnail"`
		}{
			Image: &Resource{Src: "https://example.com/art-cover.jpg"},
		},
	},
	{
		ID:       "bm-002",
		URL:      "https://example.com/article-two",
		Title:    "Why Self-Hosting Matters",
		SiteName: "Tech Today",
		Authors:  []string{"Bob Builder"},
		Labels:   []string{"tech", "self-hosting"},
		Created:  "2025-03-10T14:00:00Z",
		Updated:  "2025-03-10T14:00:00Z",
		Type:     "article",
		Resources: struct {
			Image     *Resource `json:"image"`
			Thumbnail *Resource `json:"thumbnail"`
		}{
			Thumbnail: &Resource{Src: "https://example.com/tech-thumb.jpg"},
		},
	},
}

// bm-001 has 2 annotations, bm-002 has 1
var annotations = map[string][]Annotation{
	"bm-001": {
		{ID: "ann-001", Text: "Deep reading requires removing all distractions", Created: "2025-03-01T10:30:00Z"},
		{ID: "ann-002", Text: "The brain needs at least 20 minutes to enter a flow state", Created: "2025-03-01T11:00:00Z"},
	},
	"bm-002": {
		{ID: "ann-003", Text: "Owning your data means owning your workflow", Created: "2025-03-10T15:00:00Z"},
	},
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/bookmarks", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[mock-readeck] GET /api/bookmarks")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bookmarks)
	})

	mux.HandleFunc("/api/bookmarks/", func(w http.ResponseWriter, r *http.Request) {
		// Match /api/bookmarks/{id}/annotations
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/bookmarks/"), "/")
		if len(parts) == 2 && parts[1] == "annotations" {
			bmID := parts[0]
			log.Printf("[mock-readeck] GET /api/bookmarks/%s/annotations", bmID)
			anns, ok := annotations[bmID]
			if !ok {
				anns = []Annotation{}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(anns)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Println("[mock-readeck] listening on :9090")
	log.Fatal(http.ListenAndServe(":9090", mux))
}

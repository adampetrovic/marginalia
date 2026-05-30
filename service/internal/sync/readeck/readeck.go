package readeck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/adampetrovic/marginalia/service/internal/models"
)

const sourceType = "readeck"

// Bookmark is the Readeck API bookmark response.
type Bookmark struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	SiteName  string    `json:"site_name"`
	Authors   []string  `json:"authors"`
	Labels    []string  `json:"labels"`
	Published *string   `json:"published"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Type      string    `json:"type"` // article, photo, video
	Resources struct {
		Image *struct {
			Src string `json:"src"`
		} `json:"image"`
		Thumbnail *struct {
			Src string `json:"src"`
		} `json:"thumbnail"`
	} `json:"resources"`
}

// Annotation is the Readeck API annotation/highlight response.
type Annotation struct {
	ID      string    `json:"id"`
	Text    string    `json:"text"`
	Created time.Time `json:"created"`
}

// Client talks to the Readeck API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a Readeck API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetHTTPClient allows overriding the HTTP client (for testing).
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.httpClient = hc
}

func (c *Client) doRequest(method, path string) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetBookmarks fetches all bookmarks from Readeck.
func (c *Client) GetBookmarks() ([]Bookmark, error) {
	body, err := c.doRequest("GET", "/api/bookmarks")
	if err != nil {
		return nil, err
	}

	var bookmarks []Bookmark
	if err := json.Unmarshal(body, &bookmarks); err != nil {
		return nil, fmt.Errorf("decoding bookmarks: %w", err)
	}
	return bookmarks, nil
}

// GetAnnotations fetches annotations for a specific bookmark.
func (c *Client) GetAnnotations(bookmarkID string) ([]Annotation, error) {
	body, err := c.doRequest("GET", "/api/bookmarks/"+bookmarkID+"/annotations")
	if err != nil {
		return nil, err
	}

	var annotations []Annotation
	if err := json.Unmarshal(body, &annotations); err != nil {
		return nil, fmt.Errorf("decoding annotations: %w", err)
	}
	return annotations, nil
}

// SyncResult holds the outcome of a sync operation.
type SyncResult struct {
	DocumentsSynced  int
	HighlightsSynced int
}

// Syncer handles syncing Readeck data into the local database.
type Syncer struct {
	client *Client
	db     *gorm.DB
}

// NewSyncer creates a new Readeck syncer.
func NewSyncer(client *Client, db *gorm.DB) *Syncer {
	return &Syncer{client: client, db: db}
}

// Sync fetches bookmarks and annotations from Readeck and stores them into the
// given user's source.
func (s *Syncer) Sync(src *models.Source) (*SyncResult, error) {
	result := &SyncResult{}
	sourceID := src.ID
	userID := src.UserID

	bookmarks, err := s.client.GetBookmarks()
	if err != nil {
		return nil, fmt.Errorf("fetching bookmarks: %w", err)
	}

	for _, bm := range bookmarks {
		annotations, err := s.client.GetAnnotations(bm.ID)
		if err != nil {
			return nil, fmt.Errorf("fetching annotations for %s: %w", bm.ID, err)
		}

		// Skip bookmarks with no annotations
		if len(annotations) == 0 {
			continue
		}

		author := ""
		if len(bm.Authors) > 0 {
			author = bm.Authors[0]
		}

		imageURL := ""
		if bm.Resources.Image != nil {
			imageURL = bm.Resources.Image.Src
		} else if bm.Resources.Thumbnail != nil {
			imageURL = bm.Resources.Thumbnail.Src
		}

		doc := models.Document{
			ID:               fmt.Sprintf("%s-%s", sourceID, bm.ID),
			UserID:           userID,
			SourceID:         sourceID,
			SourceDocumentID: bm.ID,
			Type:             "article",
			Title:            bm.Title,
			Author:           author,
			URL:              bm.URL,
			ImageURL:         imageURL,
			SourceURL:        bm.URL,
			Tags:             models.JSONStringArray(bm.Labels),
			Metadata: models.JSONMap{
				"site_name": bm.SiteName,
			},
		}

		// Upsert document
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_id"}, {Name: "source_document_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "author", "url", "image_url", "tags", "metadata", "updated_at"}),
		}).Create(&doc).Error; err != nil {
			return nil, fmt.Errorf("upserting document %s: %w", bm.ID, err)
		}
		result.DocumentsSynced++

		// Sync highlights
		var lastHighlightedAt *time.Time
		for _, ann := range annotations {
			created := ann.Created
			if lastHighlightedAt == nil || created.After(*lastHighlightedAt) {
				lastHighlightedAt = &created
			}

			hl := models.Highlight{
				ID:                fmt.Sprintf("%s-%s-%s", sourceID, bm.ID, ann.ID),
				UserID:            userID,
				DocumentID:        doc.ID,
				SourceHighlightID: ann.ID,
				Text:              ann.Text,
				HighlightedAt:     &created,
				SyncedAt:          time.Now(),
			}

			if err := models.UpsertHighlight(s.db, &hl, []string{"text", "synced_at", "updated_at"}); err != nil {
				return nil, fmt.Errorf("upserting highlight %s: %w", ann.ID, err)
			}
			result.HighlightsSynced++
		}

		// Update last highlighted time
		if lastHighlightedAt != nil {
			s.db.Model(&doc).Update("last_highlighted_at", lastHighlightedAt)
		}
	}

	// Update source sync time
	now := time.Now()
	s.db.Model(&models.Source{}).Where("id = ?", sourceID).Update("last_synced_at", &now)

	return result, nil
}

// EnsureSource creates or returns the Readeck source record for a user.
func EnsureSource(db *gorm.DB, userID string) (*models.Source, error) {
	id := fmt.Sprintf("readeck-%s", userID)
	src := models.Source{
		ID:     id,
		UserID: userID,
		Type:   sourceType,
		Name:   "Readeck",
	}

	result := db.Where("id = ?", id).FirstOrCreate(&src)
	if result.Error != nil {
		return nil, fmt.Errorf("ensuring source: %w", result.Error)
	}
	return &src, nil
}

package models

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// userOwnedColumns are the highlight fields a user can edit from the UI/API.
// Once a highlight is UserEdited, these are never overwritten by a re-sync so
// the user's corrections, notes, tags, and colours survive future syncs.
var userOwnedColumns = map[string]bool{
	"text":  true,
	"note":  true,
	"tags":  true,
	"color": true,
}

// UpsertHighlight inserts or updates a synced highlight while respecting user intent.
//
//   - If the highlight was previously soft-deleted by the user (a tombstone), it is
//     left deleted and NOT resurrected, even if it still exists in the source.
//   - If the existing highlight was edited by the user (UserEdited), the user-owned
//     columns (text, note, tags, color) are preserved and only the remaining columns
//     in updateColumns are refreshed.
//   - Otherwise the highlight is inserted, or updated using updateColumns on conflict.
//
// updateColumns is the set of columns a clean re-sync would refresh (e.g.
// "text", "synced_at", "updated_at").
func UpsertHighlight(db *gorm.DB, hl *Highlight, updateColumns []string) error {
	var existing Highlight
	err := db.Unscoped().
		Select("id", "user_edited", "deleted_at").
		Where("id = ?", hl.ID).
		First(&existing).Error

	switch {
	case err == nil:
		// Tombstone: the user deleted this highlight; honour that across re-sync.
		if existing.DeletedAt.Valid {
			return nil
		}
		cols := updateColumns
		if existing.UserEdited {
			cols = make([]string, 0, len(updateColumns))
			for _, c := range updateColumns {
				if !userOwnedColumns[c] {
					cols = append(cols, c)
				}
			}
		}
		conflict := clause.OnConflict{Columns: []clause.Column{{Name: "id"}}}
		if len(cols) == 0 {
			conflict.DoNothing = true
		} else {
			conflict.DoUpdates = clause.AssignmentColumns(cols)
		}
		return db.Clauses(conflict).Create(hl).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		return db.Create(hl).Error
	default:
		return err
	}
}

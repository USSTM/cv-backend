package notifications

import (
	"context"
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/google/uuid"
)

func NewEmailLookupFunc(queries *db.Queries) EmailLookupFunc {
	return func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
		rows, err := queries.GetUsersByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[uuid.UUID]string, len(rows))
		for _, row := range rows {
			result[row.ID] = row.Email
		}
		return result, nil
	}
}

// each .html file must define {{define "name:subject"}} and {{define "name:body"}} blocks,
// where name matches the filename without extension.
func LoadTemplates(dir string) (*template.Template, error) {
	pattern := filepath.Join(dir, "*.html")
	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to load email templates from %s: %w", dir, err)
	}
	return tmpl, nil
}

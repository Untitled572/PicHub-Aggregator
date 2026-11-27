package store

import (
	"database/sql"
	"errors"
)

// A nil PageIndex means the source matched Curator but did not identify its page.
type CuratorImageIdentity struct {
	Origin    string
	IllustID  string
	PageIndex *int
}

func (s *Store) GetCuratorImageIdentity(imageID int64) (*CuratorImageIdentity, error) {
	var identity CuratorImageIdentity
	err := s.db.QueryRow("SELECT origin, illust_id, page_index FROM curator_image_identities WHERE image_id = ?", imageID).
		Scan(&identity.Origin, &identity.IllustID, &identity.PageIndex)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

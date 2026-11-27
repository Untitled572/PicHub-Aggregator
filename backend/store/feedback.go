package store

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrFeedbackUnavailable = errors.New("feedback is unavailable for this image")

type FeedbackTarget struct {
	HistoryID int64
	ImageID   int64
	SourceURL string
	Ref       string
	Vote      int
}

// GetFeedbackTarget resolves either an authenticated history row or the
// exact App distribution reference. Only legacy clients omit both.
func (s *Store) GetFeedbackTarget(historyID, imageID int64, ref string) (FeedbackTarget, error) {
	var target FeedbackTarget
	if s.batcher != nil && (historyID > 0 || ref != "") {
		if err := s.batcher.flush(); err != nil {
			return target, err
		}
	}
	var err error
	if historyID > 0 || ref != "" {
		err = s.db.QueryRow(`SELECT h.id,COALESCE(h.image_id,0),COALESCE(src.url,''),h.distribution_ref,h.app_feedback
			FROM image_history h LEFT JOIN sources src ON src.id=h.source_id
			WHERE h.id=? OR (? != '' AND h.distribution_ref=? AND h.image_id=?)`,
			historyID, ref, ref, imageID).Scan(&target.HistoryID, &target.ImageID, &target.SourceURL, &target.Ref, &target.Vote)
	} else {
		target.ImageID = imageID
		err = s.db.QueryRow(`SELECT COALESCE(src.url,'') FROM images img
			LEFT JOIN sources src ON src.id=img.source_id WHERE img.id=?`, imageID).Scan(&target.SourceURL)
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrFeedbackImageGone
	}
	return target, err
}

func (s *Store) LatestImageHistoryID(imageID int64) (int64, error) {
	if s.batcher != nil {
		if err := s.batcher.flush(); err != nil {
			return 0, err
		}
	}
	var id int64
	err := s.db.QueryRow("SELECT COALESCE(MAX(id),0) FROM image_history WHERE image_id=?", imageID).Scan(&id)
	return id, err
}

func (s *Store) CuratorVote(identity *CuratorImageIdentity) (int, error) {
	var vote int
	err := s.db.QueryRow("SELECT vote FROM curator_feedback_votes WHERE origin=? AND illust_id=? AND page_index=?",
		identity.Origin, identity.IllustID, *identity.PageIndex).Scan(&vote)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return vote, err
}

// CheckFeedbackReceipt is used before an external write. The handler serializes
// the external write and its local receipt to keep retries in order.
func (s *Store) CheckFeedbackReceipt(requestID string, target FeedbackTarget, vote int) (bool, error) {
	var previousID int64
	var previousRef string
	var previousVote int
	err := s.db.QueryRow("SELECT image_id, distribution_ref, vote FROM app_feedback_receipts WHERE request_id=?", requestID).
		Scan(&previousID, &previousRef, &previousVote)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	ref := target.Ref
	if ref == "" && target.HistoryID > 0 {
		ref = fmt.Sprintf("history:%d", target.HistoryID)
	}
	if previousID != target.ImageID || previousRef != ref || previousVote != vote {
		return false, ErrFeedbackConflict
	}
	return true, nil
}

func (s *Store) RecordCuratorFeedback(requestID string, target FeedbackTarget, identity *CuratorImageIdentity, vote int) error {
	return s.recordFeedback("", requestID, target.ImageID, vote, target.Ref, target.HistoryID, identity)
}

func FeedbackSupport(config CuratorFeedbackConfig, sourceURL string, identity *CuratorImageIdentity) (string, bool) {
	if identity != nil || (config.Address != "" && CuratorFeedbackOrigin(sourceURL) == config.Address) {
		return "curator", identity != nil && identity.IllustID != "" && identity.PageIndex != nil &&
			config.Enabled && config.Token != "" && config.Address == identity.Origin
	}
	return "source", sourceURL != ""
}

func FeedbackValue(vote int) string {
	switch vote {
	case 1:
		return "like"
	case -1:
		return "dislike"
	default:
		return "none"
	}
}

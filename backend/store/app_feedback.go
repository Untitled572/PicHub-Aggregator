package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrInvalidAppKey     = errors.New("invalid app key")
	ErrFeedbackConflict  = errors.New("request id already used for a different vote")
	ErrFeedbackImageGone = errors.New("image is no longer available for feedback")
)

const appKeySetting = "app_feedback_key_hash"

func appKeyHash(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func (s *Store) HasAppKey() (bool, error) {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM settings WHERE key = ?)", appKeySetting).Scan(&exists)
	return exists, err
}

func (s *Store) SetAppKey(key string) error {
	if key == "" {
		_, err := s.db.Exec("DELETE FROM settings WHERE key = ?", appKeySetting)
		return err
	}
	_, err := s.db.Exec("INSERT INTO settings(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", appKeySetting, appKeyHash(key))
	return err
}

func (s *Store) CheckAppKey(key string) error {
	var valid bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM settings WHERE key = ? AND value = ?)", appKeySetting, appKeyHash(key)).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidAppKey
	}
	return nil
}

func (s *Store) RecordAppFeedback(key, requestID string, imageID int64, vote int, ref string) error {
	return s.recordFeedback(key, requestID, imageID, vote, ref, 0, nil)
}

func (s *Store) RecordWebFeedback(requestID string, target FeedbackTarget, vote int) error {
	return s.recordFeedback("", requestID, target.ImageID, vote, target.Ref, target.HistoryID, nil)
}

func (s *Store) recordFeedback(key, requestID string, imageID int64, vote int, ref string, historyID int64, curator *CuratorImageIdentity) error {
	if ref != "" && s.batcher != nil {
		if key != "" {
			if err := s.CheckAppKey(key); err != nil {
				return err
			}
		}
		// The viewer can react before the normal stats flush.
		if err := s.batcher.flush(); err != nil {
			return err
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	receiptRef := ref
	if ref == "" && historyID > 0 {
		receiptRef = fmt.Sprintf("history:%d", historyID)
	}
	// Acquire the write lock before checking the key so rotation and a vote are serialized.
	result, err := tx.Exec(`
		INSERT INTO app_feedback_receipts (request_id, image_id, vote, distribution_ref)
		SELECT ?, ?, ?, ? WHERE ? = '' OR EXISTS (SELECT 1 FROM settings WHERE key = ? AND value = ?)
		ON CONFLICT (request_id) DO NOTHING`,
		requestID, imageID, vote, receiptRef, key, appKeySetting, appKeyHash(key))
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if inserted == 0 {
		var previousImageID int64
		var previousVote int
		var previousRef string
		err := tx.QueryRow(`
			SELECT image_id, vote, distribution_ref FROM app_feedback_receipts
			WHERE request_id = ? AND (? = '' OR EXISTS (SELECT 1 FROM settings WHERE key = ? AND value = ?))`,
			requestID, key, appKeySetting, appKeyHash(key)).Scan(&previousImageID, &previousVote, &previousRef)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidAppKey
		}
		if err != nil {
			return err
		}
		if previousImageID != imageID || previousVote != vote || previousRef != receiptRef {
			return ErrFeedbackConflict
		}
		return nil
	}

	if curator != nil {
		_, err := tx.Exec(`INSERT INTO curator_feedback_votes(origin,illust_id,page_index,vote) VALUES (?,?,?,?)
			ON CONFLICT(origin,illust_id,page_index) DO UPDATE SET vote=excluded.vote`, curator.Origin, curator.IllustID, *curator.PageIndex, vote)
		if err != nil {
			return err
		}
	} else if ref != "" || historyID > 0 {
		if err := updateDistributionFeedback(tx, ref, imageID, historyID, vote); err != nil {
			return err
		}
	} else {
		result, err := tx.Exec(`
			UPDATE sources SET weight = MAX(30, MIN(70, weight + ?)), updated_at = CURRENT_TIMESTAMP
			WHERE id = (SELECT source_id FROM images WHERE id = ?)`, vote, imageID)
		if err != nil {
			return err
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if updated == 0 {
			return ErrFeedbackImageGone
		}
	}
	if _, err := tx.Exec("DELETE FROM app_feedback_receipts WHERE created_at < datetime('now', '-7 days')"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if ref != "" || historyID > 0 || curator != nil {
		s.notifyHistoryChanged()
	}
	return nil
}

func updateDistributionFeedback(tx *sql.Tx, ref string, imageID, id int64, vote int) error {
	var historyID, sourceID int64
	var previousVote, previousDelta, weight int
	err := tx.QueryRow(`
		SELECT h.id, h.source_id, h.app_feedback, h.app_feedback_delta, s.weight
		FROM image_history h JOIN sources s ON s.id = h.source_id
		WHERE h.id = ? OR (? != '' AND h.distribution_ref = ? AND h.image_id = ?)`, id, ref, ref, imageID).
		Scan(&historyID, &sourceID, &previousVote, &previousDelta, &weight)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrFeedbackImageGone
	}
	if err != nil || previousVote == vote {
		return err
	}

	// Undo the actual change, including no-op votes at the weight limits.
	base := max(1, min(100, weight-previousDelta))
	next := base
	if vote != 0 {
		next = max(30, min(70, base+vote))
	}
	if next != weight {
		if _, err := tx.Exec("UPDATE sources SET weight = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", next, sourceID); err != nil {
			return err
		}
	}
	_, err = tx.Exec("UPDATE image_history SET app_feedback = ?, app_feedback_delta = ? WHERE id = ?",
		vote, next-base, historyID)
	return err
}

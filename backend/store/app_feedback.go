package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
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

func (s *Store) RecordAppFeedback(key, requestID string, imageID int64, vote int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Acquire the write lock before checking the key so rotation and a vote are serialized.
	result, err := tx.Exec(`
		INSERT INTO app_feedback_receipts (request_id, image_id, vote)
		SELECT ?, ?, ? FROM settings WHERE key = ? AND value = ?
		ON CONFLICT (request_id) DO NOTHING`,
		requestID, imageID, vote, appKeySetting, appKeyHash(key))
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
		err := tx.QueryRow(`
			SELECT image_id, vote FROM app_feedback_receipts
			WHERE request_id = ? AND EXISTS (SELECT 1 FROM settings WHERE key = ? AND value = ?)`,
			requestID, appKeySetting, appKeyHash(key)).Scan(&previousImageID, &previousVote)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidAppKey
		}
		if err != nil {
			return err
		}
		if previousImageID != imageID || previousVote != vote {
			return ErrFeedbackConflict
		}
		return nil
	}

	result, err = tx.Exec(`
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
	if _, err := tx.Exec("DELETE FROM app_feedback_receipts WHERE created_at < datetime('now', '-7 days')"); err != nil {
		return err
	}
	return tx.Commit()
}

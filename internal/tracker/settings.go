package tracker

import (
	"database/sql"
	"errors"
)

// GetSetting returns the stored value for key, or def if unset.
func (t *Tracker) GetSetting(key, def string) (string, error) {
	var v string
	err := t.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

// SetSetting stores a key/value pair, replacing any existing value.
func (t *Tracker) SetSetting(key, value string) error {
	_, err := t.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

package tracker

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ListActivities returns activities ordered presets-first, then by name.
func (t *Tracker) ListActivities(includeArchived bool) ([]Activity, error) {
	q := `SELECT id, name, is_break, is_preset, archived FROM activities`
	if !includeArchived {
		q += ` WHERE archived = 0`
	}
	q += ` ORDER BY is_preset DESC, name COLLATE NOCASE`
	rows, err := t.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var acts []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.Name, &a.IsBreak, &a.IsPreset, &a.Archived); err != nil {
			return nil, err
		}
		acts = append(acts, a)
	}
	return acts, rows.Err()
}

// EnsureActivity returns the activity with the given name (case-insensitive,
// unarchiving it if needed), creating a non-preset one if it doesn't exist.
// This backs the free-text activity input.
func (t *Tracker) EnsureActivity(name string) (Activity, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Activity{}, ErrEmptyName
	}
	var act Activity
	err := t.withTx(func(tx *sql.Tx) error {
		err := tx.QueryRow(
			`SELECT id, name, is_break, is_preset, archived FROM activities WHERE name = ? COLLATE NOCASE`,
			name).Scan(&act.ID, &act.Name, &act.IsBreak, &act.IsPreset, &act.Archived)
		switch {
		case err == nil:
			if act.Archived {
				if _, err := tx.Exec(`UPDATE activities SET archived = 0 WHERE id = ?`, act.ID); err != nil {
					return err
				}
				act.Archived = false
			}
			return nil
		case errors.Is(err, sql.ErrNoRows):
			res, err := tx.Exec(`INSERT INTO activities (name) VALUES (?)`, name)
			if err != nil {
				return err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return err
			}
			act = Activity{ID: id, Name: name}
			return nil
		default:
			return err
		}
	})
	return act, err
}

// SetActivityPreset pins (or unpins) an activity: presets show as one-click
// buttons in the switch panels.
func (t *Tracker) SetActivityPreset(id int64, preset bool) error {
	return t.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE activities SET is_preset = ? WHERE id = ?`, preset, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("activity %d: %w", id, ErrNotFound)
		}
		return nil
	})
}

// SetActivityArchived hides (or restores) an activity in pickers. Archived
// activities keep their historic boundaries.
func (t *Tracker) SetActivityArchived(id int64, archived bool) error {
	return t.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE activities SET archived = ? WHERE id = ?`, archived, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("activity %d: %w", id, ErrNotFound)
		}
		return nil
	})
}

// activityMap loads all activities keyed by id, for resolving boundaries.
func activityMap(q queryer) (map[int64]Activity, error) {
	rows, err := q.Query(`SELECT id, name, is_break, is_preset, archived FROM activities`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[int64]Activity{}
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.Name, &a.IsBreak, &a.IsPreset, &a.Archived); err != nil {
			return nil, err
		}
		m[a.ID] = a
	}
	return m, rows.Err()
}

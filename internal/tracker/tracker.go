package tracker

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Tracker implements every operation on days and boundaries. All mutations
// run inside a transaction and re-validate the whole day before committing,
// so an invalid state can never be persisted even if an individual check is
// missed.
type Tracker struct {
	db *sql.DB
}

func New(db *sql.DB) *Tracker {
	return &Tracker{db: db}
}

// --- time encoding -----------------------------------------------------

// Timestamps are stored as RFC3339 UTC strings with second precision.
// Sub-second precision is dropped on write so equality and ordering checks
// behave the same in Go and in the database.

func toDB(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func fromDB(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// --- transactions ------------------------------------------------------

func (t *Tracker) withTx(fn func(tx *sql.Tx) error) error {
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// --- loading -----------------------------------------------------------

// queryer lets the same helpers run on *sql.DB and *sql.Tx.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

func getDay(q queryer, dayID int64) (Day, error) {
	var (
		day     Day
		endedAt sql.NullString
	)
	err := q.QueryRow(`SELECT id, date, ended_at FROM days WHERE id = ?`, dayID).
		Scan(&day.ID, &day.Date, &endedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Day{}, fmt.Errorf("day %d: %w", dayID, ErrNotFound)
	}
	if err != nil {
		return Day{}, err
	}
	if endedAt.Valid {
		at, err := fromDB(endedAt.String)
		if err != nil {
			return Day{}, err
		}
		day.EndedAt = &at
	}
	day.Boundaries, err = getBoundaries(q, dayID)
	return day, err
}

func getBoundaries(q queryer, dayID int64) ([]Boundary, error) {
	rows, err := q.Query(
		`SELECT id, day_id, at, activity_id FROM boundaries WHERE day_id = ? ORDER BY at`, dayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bs []Boundary
	for rows.Next() {
		var (
			b  Boundary
			at string
		)
		if err := rows.Scan(&b.ID, &b.DayID, &at, &b.ActivityID); err != nil {
			return nil, err
		}
		if b.At, err = fromDB(at); err != nil {
			return nil, err
		}
		bs = append(bs, b)
	}
	return bs, rows.Err()
}

func getBoundary(q queryer, boundaryID int64) (Boundary, error) {
	var (
		b  Boundary
		at string
	)
	err := q.QueryRow(`SELECT id, day_id, at, activity_id FROM boundaries WHERE id = ?`, boundaryID).
		Scan(&b.ID, &b.DayID, &at, &b.ActivityID)
	if errors.Is(err, sql.ErrNoRows) {
		return Boundary{}, fmt.Errorf("boundary %d: %w", boundaryID, ErrNotFound)
	}
	if err != nil {
		return Boundary{}, err
	}
	b.At, err = fromDB(at)
	return b, err
}

// validate checks the day invariants: at least one boundary, strictly
// increasing timestamps, and ended_at (if set) after the last boundary.
func validate(d Day) error {
	if len(d.Boundaries) == 0 {
		return fmt.Errorf("day %s: %w", d.Date, errors.New("day must have at least one boundary"))
	}
	for i := 1; i < len(d.Boundaries); i++ {
		if !d.Boundaries[i].At.After(d.Boundaries[i-1].At) {
			return ErrBoundaryOrder
		}
	}
	if d.EndedAt != nil && !d.EndedAt.After(d.Boundaries[len(d.Boundaries)-1].At) {
		return ErrEndBeforeLast
	}
	return nil
}

// revalidate reloads the day inside the transaction and checks invariants;
// called as the last step of every mutation.
func revalidate(tx *sql.Tx, dayID int64) error {
	day, err := getDay(tx, dayID)
	if err != nil {
		return err
	}
	return validate(day)
}

// --- day queries --------------------------------------------------------

// DayByID returns a day with its boundaries.
func (t *Tracker) DayByID(dayID int64) (Day, error) {
	return getDay(t.db, dayID)
}

// DayByDate returns the day for a local date string, or ErrNotFound.
func (t *Tracker) DayByDate(date string) (Day, error) {
	var id int64
	err := t.db.QueryRow(`SELECT id FROM days WHERE date = ?`, date).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Day{}, fmt.Errorf("day %s: %w", date, ErrNotFound)
	}
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, id)
}

// OpenDay returns the most recent day that has not been ended, or
// ErrNotFound if every day is closed.
func (t *Tracker) OpenDay() (Day, error) {
	var id int64
	err := t.db.QueryRow(
		`SELECT id FROM days WHERE ended_at IS NULL ORDER BY date DESC LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Day{}, ErrNotFound
	}
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, id)
}

// ListDays returns days in [from, to] (inclusive local date strings),
// newest first, without boundaries.
func (t *Tracker) ListDays(from, to string) ([]Day, error) {
	rows, err := t.db.Query(
		`SELECT id, date, ended_at FROM days WHERE date >= ? AND date <= ? ORDER BY date DESC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []Day
	for rows.Next() {
		var (
			d       Day
			endedAt sql.NullString
		)
		if err := rows.Scan(&d.ID, &d.Date, &endedAt); err != nil {
			return nil, err
		}
		if endedAt.Valid {
			at, err := fromDB(endedAt.String)
			if err != nil {
				return nil, err
			}
			d.EndedAt = &at
		}
		days = append(days, d)
	}
	return days, rows.Err()
}

// --- day mutations -------------------------------------------------------

// StartDay creates a day for the given local date with its first boundary.
func (t *Tracker) StartDay(date string, at time.Time, activityID int64) (Day, error) {
	var day Day
	err := t.withTx(func(tx *sql.Tx) error {
		var existing int64
		err := tx.QueryRow(`SELECT id FROM days WHERE date = ?`, date).Scan(&existing)
		if err == nil {
			return fmt.Errorf("%s: %w", date, ErrDayExists)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		res, err := tx.Exec(`INSERT INTO days (date) VALUES (?)`, date)
		if err != nil {
			return err
		}
		dayID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO boundaries (day_id, at, activity_id) VALUES (?, ?, ?)`,
			dayID, toDB(at), activityID); err != nil {
			return err
		}
		if err := revalidate(tx, dayID); err != nil {
			return err
		}
		day, err = getDay(tx, dayID)
		return err
	})
	return day, err
}

// EndDay closes a running day. Fails if it is already ended.
func (t *Tracker) EndDay(dayID int64, at time.Time) (Day, error) {
	var day Day
	err := t.withTx(func(tx *sql.Tx) error {
		d, err := getDay(tx, dayID)
		if err != nil {
			return err
		}
		if d.EndedAt != nil {
			return ErrDayEnded
		}
		return setDayEnd(tx, dayID, at)
	})
	if err != nil {
		return Day{}, err
	}
	day, err = getDay(t.db, dayID)
	return day, err
}

// SetDayEnd edits the end time of an already-ended day.
func (t *Tracker) SetDayEnd(dayID int64, at time.Time) (Day, error) {
	err := t.withTx(func(tx *sql.Tx) error {
		d, err := getDay(tx, dayID)
		if err != nil {
			return err
		}
		if d.EndedAt == nil {
			return ErrDayNotEnded
		}
		return setDayEnd(tx, dayID, at)
	})
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, dayID)
}

func setDayEnd(tx *sql.Tx, dayID int64, at time.Time) error {
	if _, err := tx.Exec(`UPDATE days SET ended_at = ? WHERE id = ?`, toDB(at), dayID); err != nil {
		return err
	}
	return revalidate(tx, dayID)
}

// ReopenDay clears ended_at so the day is running again.
func (t *Tracker) ReopenDay(dayID int64) (Day, error) {
	err := t.withTx(func(tx *sql.Tx) error {
		if _, err := getDay(tx, dayID); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE days SET ended_at = NULL WHERE id = ?`, dayID)
		return err
	})
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, dayID)
}

// DeleteDay removes a day and (via ON DELETE CASCADE) its boundaries.
func (t *Tracker) DeleteDay(dayID int64) error {
	return t.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM days WHERE id = ?`, dayID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("day %d: %w", dayID, ErrNotFound)
		}
		return nil
	})
}

// --- boundary mutations ---------------------------------------------------

// AddBoundary inserts a boundary (switch activity live, or split a segment
// when editing). The time must not be before the day start, must not
// collide with an existing boundary, and must be before ended_at on a
// closed day.
func (t *Tracker) AddBoundary(dayID int64, at time.Time, activityID int64) (Day, error) {
	err := t.withTx(func(tx *sql.Tx) error {
		day, err := getDay(tx, dayID)
		if err != nil {
			return err
		}
		at := at.UTC().Truncate(time.Second)
		if at.Before(day.Start()) {
			return ErrBeforeDayStart
		}
		for _, b := range day.Boundaries {
			if b.At.Equal(at) {
				return ErrDuplicateAt
			}
		}
		if day.EndedAt != nil && !at.Before(*day.EndedAt) {
			return ErrEndBeforeLast
		}
		if _, err := tx.Exec(
			`INSERT INTO boundaries (day_id, at, activity_id) VALUES (?, ?, ?)`,
			dayID, toDB(at), activityID); err != nil {
			return err
		}
		return revalidate(tx, dayID)
	})
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, dayID)
}

// MoveBoundary changes a boundary's time. It must stay strictly between its
// neighbours (this is the "drag" operation: it resizes the two adjacent
// segments and can never create a gap or overlap).
func (t *Tracker) MoveBoundary(boundaryID int64, at time.Time) (Day, error) {
	var dayID int64
	err := t.withTx(func(tx *sql.Tx) error {
		b, err := getBoundary(tx, boundaryID)
		if err != nil {
			return err
		}
		dayID = b.DayID
		day, err := getDay(tx, dayID)
		if err != nil {
			return err
		}
		at := at.UTC().Truncate(time.Second)
		for _, other := range day.Boundaries {
			if other.ID != boundaryID && other.At.Equal(at) {
				return ErrDuplicateAt
			}
		}
		// A moved boundary must stay strictly between its current
		// neighbours. Boundaries are reloaded sorted by time, so without
		// this check a move past a neighbour would silently reorder
		// segments instead of failing validation.
		idx := -1
		for i, other := range day.Boundaries {
			if other.ID == boundaryID {
				idx = i
				break
			}
		}
		if idx > 0 && !at.After(day.Boundaries[idx-1].At) {
			return ErrBoundaryOrder
		}
		if idx < len(day.Boundaries)-1 && !at.Before(day.Boundaries[idx+1].At) {
			return ErrBoundaryOrder
		}
		if _, err := tx.Exec(`UPDATE boundaries SET at = ? WHERE id = ?`, toDB(at), boundaryID); err != nil {
			return err
		}
		// Ordering, collision and ended_at rules all fall out of the
		// full-day invariant check.
		return revalidate(tx, dayID)
	})
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, dayID)
}

// DeleteBoundary removes a boundary, merging its segment into the previous
// one (or, for the first boundary, moving the day start to the second).
func (t *Tracker) DeleteBoundary(boundaryID int64) (Day, error) {
	var dayID int64
	err := t.withTx(func(tx *sql.Tx) error {
		b, err := getBoundary(tx, boundaryID)
		if err != nil {
			return err
		}
		dayID = b.DayID
		day, err := getDay(tx, dayID)
		if err != nil {
			return err
		}
		if len(day.Boundaries) == 1 {
			return ErrOnlyBoundary
		}
		if _, err := tx.Exec(`DELETE FROM boundaries WHERE id = ?`, boundaryID); err != nil {
			return err
		}
		return revalidate(tx, dayID)
	})
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, dayID)
}

// SetBoundaryActivity changes which activity a segment is tagged with.
func (t *Tracker) SetBoundaryActivity(boundaryID, activityID int64) (Day, error) {
	var dayID int64
	err := t.withTx(func(tx *sql.Tx) error {
		b, err := getBoundary(tx, boundaryID)
		if err != nil {
			return err
		}
		dayID = b.DayID
		_, err = tx.Exec(`UPDATE boundaries SET activity_id = ? WHERE id = ?`, activityID, boundaryID)
		return err
	})
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, dayID)
}

// --- back from break --------------------------------------------------------

// BackFromBreak retroactively records a break that ends now: it inserts a
// break boundary at now-dur and a boundary at now resuming the last
// non-break activity. Boundaries that fall inside the break window are
// removed — the user was away, so switches recorded there are void.
//
// Edge cases:
//   - day started less than dur ago: the break is clamped to the day start
//     (the whole day so far becomes break; the day start is unchanged)
//   - the current segment is already a break: the resumed activity is the
//     last non-break one before it
//   - no non-break activity exists at all: ErrNoResume
func (t *Tracker) BackFromBreak(dayID int64, now time.Time, dur time.Duration) (Day, error) {
	err := t.withTx(func(tx *sql.Tx) error {
		day, err := getDay(tx, dayID)
		if err != nil {
			return err
		}
		if day.EndedAt != nil {
			return ErrDayEnded
		}
		now := now.UTC().Truncate(time.Second)
		if !now.After(day.Start()) {
			return ErrBeforeDayStart
		}

		acts, err := activityMap(tx)
		if err != nil {
			return err
		}

		// The activity to resume: last boundary whose activity is not a
		// break. Found before any deletion so switches inside the break
		// window still count as "what I was doing".
		var resumeID int64
		found := false
		for i := len(day.Boundaries) - 1; i >= 0; i-- {
			if !acts[day.Boundaries[i].ActivityID].IsBreak {
				resumeID = day.Boundaries[i].ActivityID
				found = true
				break
			}
		}
		if !found {
			return ErrNoResume
		}

		breakID, err := breakActivityID(tx)
		if err != nil {
			return err
		}

		breakStart := now.Add(-dur)
		if breakStart.Before(day.Start()) {
			breakStart = day.Start()
		}

		// Delete every boundary inside [breakStart, now]. If the break
		// covers the whole day this includes the first boundary, but the
		// break boundary inserted at breakStart takes over the exact same
		// timestamp, so the day start is preserved.
		for _, b := range day.Boundaries {
			if !b.At.Before(breakStart) {
				if _, err := tx.Exec(`DELETE FROM boundaries WHERE id = ?`, b.ID); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO boundaries (day_id, at, activity_id) VALUES (?, ?, ?)`,
			dayID, toDB(breakStart), breakID); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO boundaries (day_id, at, activity_id) VALUES (?, ?, ?)`,
			dayID, toDB(now), resumeID); err != nil {
			return err
		}
		return revalidate(tx, dayID)
	})
	if err != nil {
		return Day{}, err
	}
	return getDay(t.db, dayID)
}

// breakActivityID returns the preset break activity (lowest id wins if the
// user ever creates more break activities).
func breakActivityID(q queryer) (int64, error) {
	var id int64
	err := q.QueryRow(
		`SELECT id FROM activities WHERE is_break = 1 ORDER BY is_preset DESC, id LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("no break activity configured")
	}
	return id, err
}

// sortTotals orders per-activity totals largest first for display.
func sortTotals(totals []ActivityTotal) {
	sort.Slice(totals, func(i, j int) bool {
		if totals[i].Seconds != totals[j].Seconds {
			return totals[i].Seconds > totals[j].Seconds
		}
		return totals[i].Activity.Name < totals[j].Activity.Name
	})
}

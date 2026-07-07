// Package tracker contains the domain model and all business rules for
// gapless day tracking. It works directly on *sql.DB and knows nothing
// about Wails; the service layer wraps it.
package tracker

import (
	"errors"
	"time"
)

// Sentinel errors. Callers (and tests) can match them with errors.Is; the
// service layer turns them into user-facing messages.
var (
	ErrNotFound       = errors.New("not found")
	ErrDayExists      = errors.New("a day for this date already exists")
	ErrDayEnded       = errors.New("day has already ended")
	ErrDayNotEnded    = errors.New("day has not ended")
	ErrBoundaryOrder  = errors.New("boundaries must be strictly increasing")
	ErrEndBeforeLast  = errors.New("day end must be after the last boundary")
	ErrBeforeDayStart = errors.New("time must not be before the day start")
	ErrOnlyBoundary   = errors.New("cannot delete the only boundary of a day")
	ErrDuplicateAt    = errors.New("a boundary already exists at this time")
	ErrNoResume       = errors.New("no previous non-break activity to resume")
	ErrEmptyName      = errors.New("activity name must not be empty")
)

type Activity struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsBreak  bool   `json:"isBreak"`
	IsPreset bool   `json:"isPreset"`
	Archived bool   `json:"archived"`
}

// Boundary marks the moment the tagged activity starts. The segment it opens
// ends at the next boundary, or at Day.EndedAt for the last one.
type Boundary struct {
	ID         int64     `json:"id"`
	DayID      int64     `json:"dayId"`
	At         time.Time `json:"at"`
	ActivityID int64     `json:"activityId"`
}

type Day struct {
	ID         int64      `json:"id"`
	Date       string     `json:"date"` // local calendar date, YYYY-MM-DD
	EndedAt    *time.Time `json:"endedAt"`
	Boundaries []Boundary `json:"boundaries"` // ordered by At ascending
}

// Start returns the day's first boundary time (the zero time if the day has
// no boundaries, which validate() rules out for persisted days).
func (d Day) Start() time.Time {
	if len(d.Boundaries) == 0 {
		return time.Time{}
	}
	return d.Boundaries[0].At
}

// Segment is a derived view: the span between a boundary and its successor.
type Segment struct {
	BoundaryID int64      `json:"boundaryId"`
	Activity   Activity   `json:"activity"`
	Start      time.Time  `json:"start"`
	End        *time.Time `json:"end"` // nil while this segment is still running
}

// ActivityTotal aggregates time per activity. Seconds instead of
// time.Duration because it crosses the JSON boundary to the frontend.
type ActivityTotal struct {
	Activity Activity `json:"activity"`
	Seconds  int64    `json:"seconds"`
}

type DaySummary struct {
	DayID      int64           `json:"dayId"`
	Date       string          `json:"date"`
	Start      time.Time       `json:"start"`
	End        *time.Time      `json:"end"`
	TotalSecs  int64           `json:"totalSecs"`
	BreakSecs  int64           `json:"breakSecs"`
	WorkSecs   int64           `json:"workSecs"`
	ByActivity []ActivityTotal `json:"byActivity"`
	Segments   []Segment       `json:"segments"`
}

// RangeSummary aggregates several days (e.g. a week).
type RangeSummary struct {
	From       string          `json:"from"`
	To         string          `json:"to"`
	Days       []DaySummary    `json:"days"`
	TotalSecs  int64           `json:"totalSecs"`
	BreakSecs  int64           `json:"breakSecs"`
	WorkSecs   int64           `json:"workSecs"`
	ByActivity []ActivityTotal `json:"byActivity"`
}

package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"timetrack/internal/tracker"
)

// BreakDuration is how much time "Back from break" covers.
const BreakDuration = 45 * time.Minute

const settingDefaultActivity = "default_activity"

// DayChangedEvent is emitted after every mutation so all windows (main +
// quick popup) refresh from the backend instead of holding their own state.
const DayChangedEvent = "day:changed"

// TrackerService is the Wails-bound facade over the domain layer. It does
// edge conversion only: ISO timestamp strings <-> time.Time, local calendar
// dates, and change events. All rules live in internal/tracker.
type TrackerService struct {
	t *tracker.Tracker
	// app is set in main after application.New; nil in tests.
	app *application.App
}

func NewTrackerService(t *tracker.Tracker) *TrackerService {
	return &TrackerService{t: t}
}

func (s *TrackerService) emitChanged() {
	if s.app != nil {
		s.app.Event.Emit(DayChangedEvent, "")
	}
}

// parseISO accepts RFC3339 timestamps (what JS Date.toISOString produces,
// including fractional seconds). All timestamps are aligned to whole
// minutes: booking granularity is minutes, and mixing second-precision
// boundaries (live switches) with minute-precision ones (time inputs)
// produces confusing sub-minute durations like "18:54–18:55 = 0:00".
func parseISO(iso string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp %q: %w", iso, err)
	}
	return t.Truncate(time.Minute), nil
}

// nowMinute is the live-tracking clock, minute-aligned like parseISO.
func nowMinute() time.Time {
	return time.Now().Truncate(time.Minute)
}

// localDate converts a moment to the local calendar date string.
func localDate(t time.Time) string {
	return t.Local().Format("2006-01-02")
}

// --- view types ------------------------------------------------------------

// DayView is what the frontend renders: the raw day (boundaries carry the
// ids needed for edits) plus the derived summary (segments and totals).
type DayView struct {
	Day     tracker.Day        `json:"day"`
	Summary tracker.DaySummary `json:"summary"`
}

// HomeState is everything the main and quick windows need on load.
type HomeState struct {
	Today           string             `json:"today"` // local date YYYY-MM-DD
	OpenDay         *DayView           `json:"openDay"`
	TodayDay        *DayView           `json:"todayDay"` // may be the same day as OpenDay, or an ended day
	Activities      []tracker.Activity `json:"activities"`
	DefaultActivity string             `json:"defaultActivity"`
	BreakMinutes    int                `json:"breakMinutes"`
}

func (s *TrackerService) dayView(dayID int64) (DayView, error) {
	day, err := s.t.DayByID(dayID)
	if err != nil {
		return DayView{}, err
	}
	sum, err := s.t.Summary(dayID, time.Now())
	if err != nil {
		return DayView{}, err
	}
	return DayView{Day: day, Summary: sum}, nil
}

func (s *TrackerService) dayViewPtr(dayID int64) (*DayView, error) {
	v, err := s.dayView(dayID)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// --- queries -----------------------------------------------------------------

func (s *TrackerService) HomeState() (HomeState, error) {
	state := HomeState{
		Today:        localDate(time.Now()),
		BreakMinutes: int(BreakDuration / time.Minute),
	}

	var err error
	if state.Activities, err = s.t.ListActivities(false); err != nil {
		return HomeState{}, err
	}
	if state.DefaultActivity, err = s.t.GetSetting(settingDefaultActivity, "Development"); err != nil {
		return HomeState{}, err
	}

	open, err := s.t.OpenDay()
	switch {
	case err == nil:
		if state.OpenDay, err = s.dayViewPtr(open.ID); err != nil {
			return HomeState{}, err
		}
	case !errors.Is(err, tracker.ErrNotFound):
		return HomeState{}, err
	}

	today, err := s.t.DayByDate(state.Today)
	switch {
	case err == nil:
		if state.TodayDay, err = s.dayViewPtr(today.ID); err != nil {
			return HomeState{}, err
		}
	case !errors.Is(err, tracker.ErrNotFound):
		return HomeState{}, err
	}
	return state, nil
}

// Activities lists selectable activities for autocomplete and presets.
func (s *TrackerService) Activities(includeArchived bool) ([]tracker.Activity, error) {
	return s.t.ListActivities(includeArchived)
}

// DayByDate returns the day for a local date, or null if none exists.
func (s *TrackerService) DayByDate(date string) (*DayView, error) {
	day, err := s.t.DayByDate(date)
	if errors.Is(err, tracker.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.dayViewPtr(day.ID)
}

// ListDays returns day rows (without boundaries) in [from, to], newest first.
func (s *TrackerService) ListDays(from, to string) ([]tracker.Day, error) {
	return s.t.ListDays(from, to)
}

// RangeSummary aggregates the days in [from, to], e.g. a week.
func (s *TrackerService) RangeSummary(from, to string) (tracker.RangeSummary, error) {
	return s.t.RangeSummary(from, to, time.Now())
}

// --- live tracking mutations --------------------------------------------------

// StartDay starts the day whose local date contains atISO.
func (s *TrackerService) StartDay(atISO, activityName string) (*DayView, error) {
	at, err := parseISO(atISO)
	if err != nil {
		return nil, err
	}
	act, err := s.t.EnsureActivity(activityName)
	if err != nil {
		return nil, err
	}
	day, err := s.t.StartDay(localDate(at), at, act.ID)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

// SwitchActivity records that the given activity starts now on the open day.
// Unknown names create a new activity (free-text input).
func (s *TrackerService) SwitchActivity(activityName string) (*DayView, error) {
	open, err := s.t.OpenDay()
	if err != nil {
		return nil, fmt.Errorf("no running day: %w", err)
	}
	act, err := s.t.EnsureActivity(activityName)
	if err != nil {
		return nil, err
	}
	at := nowMinute()
	day, err := s.t.AddBoundary(open.ID, at, act.ID)
	if errors.Is(err, tracker.ErrDuplicateAt) {
		// Switched again within the same minute: the user changed their
		// mind, so retag that boundary instead of failing.
		for _, b := range open.Boundaries {
			if b.At.Equal(at) {
				day, err = s.t.SetBoundaryActivity(b.ID, act.ID)
				break
			}
		}
	}
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

// BackFromBreak books the last 45 minutes of the open day as Break and
// resumes the previous activity now.
func (s *TrackerService) BackFromBreak() (*DayView, error) {
	open, err := s.t.OpenDay()
	if err != nil {
		return nil, fmt.Errorf("no running day: %w", err)
	}
	day, err := s.t.BackFromBreak(open.ID, nowMinute(), BreakDuration)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

// EndDay closes the open day at the given time.
func (s *TrackerService) EndDay(atISO string) (*DayView, error) {
	open, err := s.t.OpenDay()
	if err != nil {
		return nil, fmt.Errorf("no running day: %w", err)
	}
	at, err := parseISO(atISO)
	if err != nil {
		return nil, err
	}
	day, err := s.t.EndDay(open.ID, at)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

// --- review / edit mutations ---------------------------------------------------

func (s *TrackerService) AddBoundary(dayID int64, atISO, activityName string) (*DayView, error) {
	at, err := parseISO(atISO)
	if err != nil {
		return nil, err
	}
	act, err := s.t.EnsureActivity(activityName)
	if err != nil {
		return nil, err
	}
	day, err := s.t.AddBoundary(dayID, at, act.ID)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

func (s *TrackerService) MoveBoundary(boundaryID int64, atISO string) (*DayView, error) {
	at, err := parseISO(atISO)
	if err != nil {
		return nil, err
	}
	day, err := s.t.MoveBoundary(boundaryID, at)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

func (s *TrackerService) DeleteBoundary(boundaryID int64) (*DayView, error) {
	day, err := s.t.DeleteBoundary(boundaryID)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

func (s *TrackerService) SetBoundaryActivity(boundaryID int64, activityName string) (*DayView, error) {
	act, err := s.t.EnsureActivity(activityName)
	if err != nil {
		return nil, err
	}
	day, err := s.t.SetBoundaryActivity(boundaryID, act.ID)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

// SetDayEnd edits the end time of an ended day.
func (s *TrackerService) SetDayEnd(dayID int64, atISO string) (*DayView, error) {
	at, err := parseISO(atISO)
	if err != nil {
		return nil, err
	}
	day, err := s.t.SetDayEnd(dayID, at)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

// ReopenDay clears a day's end so tracking continues.
func (s *TrackerService) ReopenDay(dayID int64) (*DayView, error) {
	day, err := s.t.ReopenDay(dayID)
	if err != nil {
		return nil, err
	}
	defer s.emitChanged()
	return s.dayViewPtr(day.ID)
}

func (s *TrackerService) DeleteDay(dayID int64) error {
	if err := s.t.DeleteDay(dayID); err != nil {
		return err
	}
	s.emitChanged()
	return nil
}

// --- export ---------------------------------------------------------------------

// ExportCSV writes one row per segment for all days in [from, to] to a
// location chosen via the native save dialog. Returns the written path, or
// "" if the user cancelled.
func (s *TrackerService) ExportCSV(from, to string) (string, error) {
	rs, err := s.t.RangeSummary(from, to, time.Now())
	if err != nil {
		return "", err
	}

	path, err := s.app.Dialog.SaveFile().
		SetFilename(fmt.Sprintf("timetrack_%s_%s.csv", from, to)).
		AddFilter("CSV files", "*.csv").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // user cancelled
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"date", "start", "end", "minutes", "hours", "activity", "is_break"})
	for _, d := range rs.Days {
		for _, seg := range d.Segments {
			end := time.Now()
			endStr := "" // still running
			if seg.End != nil {
				end = *seg.End
				endStr = end.Local().Format("15:04")
			}
			mins := end.Sub(seg.Start).Minutes()
			_ = w.Write([]string{
				d.Date,
				seg.Start.Local().Format("15:04"),
				endStr,
				strconv.Itoa(int(mins)),
				strconv.FormatFloat(mins/60, 'f', 2, 64),
				seg.Activity.Name,
				strconv.FormatBool(seg.Activity.IsBreak),
			})
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// --- settings -------------------------------------------------------------------

func (s *TrackerService) SetDefaultActivity(name string) error {
	act, err := s.t.EnsureActivity(name)
	if err != nil {
		return err
	}
	return s.t.SetSetting(settingDefaultActivity, act.Name)
}

func (s *TrackerService) SetActivityArchived(id int64, archived bool) error {
	if err := s.t.SetActivityArchived(id, archived); err != nil {
		return err
	}
	s.emitChanged()
	return nil
}

// CreateActivity adds (or revives) an activity without switching to it.
func (s *TrackerService) CreateActivity(name string) (tracker.Activity, error) {
	act, err := s.t.EnsureActivity(name)
	if err != nil {
		return tracker.Activity{}, err
	}
	s.emitChanged()
	return act, nil
}

// SetActivityPreset pins/unpins an activity as a one-click button.
func (s *TrackerService) SetActivityPreset(id int64, preset bool) error {
	if err := s.t.SetActivityPreset(id, preset); err != nil {
		return err
	}
	s.emitChanged()
	return nil
}

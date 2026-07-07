package tracker

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"timetrack/internal/store"
)

// ts returns a fixed test timestamp offset by min minutes from 09:00 UTC.
func ts(min int) time.Time {
	return time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC).Add(time.Duration(min) * time.Minute)
}

func newTestTracker(t *testing.T) *Tracker {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db)
}

// actID resolves a seeded/created activity by name.
func actID(t *testing.T, tr *Tracker, name string) int64 {
	t.Helper()
	act, err := tr.EnsureActivity(name)
	if err != nil {
		t.Fatalf("ensure activity %q: %v", name, err)
	}
	return act.ID
}

// boundarySpec describes one boundary as (minutes after 09:00, activity name).
type boundarySpec struct {
	min      int
	activity string
}

// buildDay creates a day starting with specs[0] and adds the rest.
func buildDay(t *testing.T, tr *Tracker, specs []boundarySpec, endedMin *int) Day {
	t.Helper()
	if len(specs) == 0 {
		t.Fatal("buildDay needs at least one boundary")
	}
	day, err := tr.StartDay("2026-07-06", ts(specs[0].min), actID(t, tr, specs[0].activity))
	if err != nil {
		t.Fatalf("start day: %v", err)
	}
	for _, s := range specs[1:] {
		if day, err = tr.AddBoundary(day.ID, ts(s.min), actID(t, tr, s.activity)); err != nil {
			t.Fatalf("add boundary %+v: %v", s, err)
		}
	}
	if endedMin != nil {
		if day, err = tr.EndDay(day.ID, ts(*endedMin)); err != nil {
			t.Fatalf("end day: %v", err)
		}
	}
	return day
}

// assertBoundaries checks the day's boundaries against expected specs.
func assertBoundaries(t *testing.T, tr *Tracker, day Day, want []boundarySpec) {
	t.Helper()
	got, err := tr.DayByID(day.ID)
	if err != nil {
		t.Fatalf("reload day: %v", err)
	}
	if len(got.Boundaries) != len(want) {
		t.Fatalf("boundary count = %d, want %d (%+v)", len(got.Boundaries), len(want), got.Boundaries)
	}
	for i, w := range want {
		b := got.Boundaries[i]
		if !b.At.Equal(ts(w.min)) {
			t.Errorf("boundary[%d].At = %v, want %v", i, b.At, ts(w.min))
		}
		if b.ActivityID != actID(t, tr, w.activity) {
			t.Errorf("boundary[%d] activity = %d, want %q", i, b.ActivityID, w.activity)
		}
	}
}

func intPtr(v int) *int { return &v }

func TestStartDay(t *testing.T) {
	tr := newTestTracker(t)
	day, err := tr.StartDay("2026-07-06", ts(0), actID(t, tr, "Development"))
	if err != nil {
		t.Fatalf("start day: %v", err)
	}
	if len(day.Boundaries) != 1 || !day.Start().Equal(ts(0)) {
		t.Errorf("unexpected day after start: %+v", day)
	}
	if _, err := tr.StartDay("2026-07-06", ts(60), actID(t, tr, "Development")); !errors.Is(err, ErrDayExists) {
		t.Errorf("duplicate StartDay err = %v, want ErrDayExists", err)
	}
}

func TestAddBoundary(t *testing.T) {
	tests := []struct {
		name    string
		setup   []boundarySpec
		ended   *int
		atMin   int
		wantErr error
		want    []boundarySpec // checked only when wantErr == nil
	}{
		{
			name:  "switch appends",
			setup: []boundarySpec{{0, "Development"}},
			atMin: 60,
			want:  []boundarySpec{{0, "Development"}, {60, "Meeting"}},
		},
		{
			name:  "split existing segment",
			setup: []boundarySpec{{0, "Development"}, {120, "Development"}},
			atMin: 60,
			want:  []boundarySpec{{0, "Development"}, {60, "Meeting"}, {120, "Development"}},
		},
		{
			name:    "before day start",
			setup:   []boundarySpec{{60, "Development"}},
			atMin:   0,
			wantErr: ErrBeforeDayStart,
		},
		{
			name:    "duplicate timestamp",
			setup:   []boundarySpec{{0, "Development"}},
			atMin:   0,
			wantErr: ErrDuplicateAt,
		},
		{
			name:    "at or after day end",
			setup:   []boundarySpec{{0, "Development"}},
			ended:   intPtr(480),
			atMin:   480,
			wantErr: ErrEndBeforeLast,
		},
		{
			name:  "inside ended day",
			setup: []boundarySpec{{0, "Development"}},
			ended: intPtr(480),
			atMin: 240,
			want:  []boundarySpec{{0, "Development"}, {240, "Meeting"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newTestTracker(t)
			day := buildDay(t, tr, tc.setup, tc.ended)
			_, err := tr.AddBoundary(day.ID, ts(tc.atMin), actID(t, tr, "Meeting"))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			assertBoundaries(t, tr, day, tc.want)
		})
	}
}

func TestMoveBoundary(t *testing.T) {
	// Fixed setup: Dev@0, Meeting@180, Dev@240, ended@480.
	setup := []boundarySpec{{0, "Development"}, {180, "Meeting"}, {240, "Development"}}
	tests := []struct {
		name    string
		index   int // which boundary to move
		toMin   int
		wantErr error
	}{
		{name: "resize adjacent segments", index: 1, toMin: 200},
		{name: "onto previous boundary", index: 1, toMin: 0, wantErr: ErrDuplicateAt},
		{name: "before previous boundary", index: 1, toMin: -30, wantErr: ErrBoundaryOrder},
		{name: "past next boundary", index: 1, toMin: 300, wantErr: ErrBoundaryOrder},
		{name: "last past day end", index: 2, toMin: 500, wantErr: ErrEndBeforeLast},
		{name: "move day start earlier", index: 0, toMin: -60},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newTestTracker(t)
			day := buildDay(t, tr, setup, intPtr(480))
			id := day.Boundaries[tc.index].ID
			_, err := tr.MoveBoundary(id, ts(tc.toMin))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				// The failed move must not have been persisted.
				assertBoundaries(t, tr, day, setup)
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			got, _ := tr.DayByID(day.ID)
			if !got.Boundaries[tc.index].At.Equal(ts(tc.toMin)) {
				t.Errorf("boundary not moved: %+v", got.Boundaries[tc.index])
			}
		})
	}
}

func TestDeleteBoundary(t *testing.T) {
	tests := []struct {
		name    string
		setup   []boundarySpec
		index   int
		wantErr error
		want    []boundarySpec
	}{
		{
			name:  "merge into previous segment",
			setup: []boundarySpec{{0, "Development"}, {60, "Meeting"}, {120, "Development"}},
			index: 1,
			want:  []boundarySpec{{0, "Development"}, {120, "Development"}},
		},
		{
			name:  "delete first moves day start",
			setup: []boundarySpec{{0, "Development"}, {60, "Meeting"}},
			index: 0,
			want:  []boundarySpec{{60, "Meeting"}},
		},
		{
			name:    "only boundary",
			setup:   []boundarySpec{{0, "Development"}},
			index:   0,
			wantErr: ErrOnlyBoundary,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newTestTracker(t)
			day := buildDay(t, tr, tc.setup, nil)
			_, err := tr.DeleteBoundary(day.Boundaries[tc.index].ID)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			assertBoundaries(t, tr, day, tc.want)
		})
	}
}

func TestEndDay(t *testing.T) {
	tests := []struct {
		name    string
		endMin  int
		wantErr error
	}{
		{name: "valid", endMin: 480},
		{name: "before last boundary", endMin: 30, wantErr: ErrEndBeforeLast},
		{name: "equal to last boundary", endMin: 60, wantErr: ErrEndBeforeLast},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newTestTracker(t)
			day := buildDay(t, tr, []boundarySpec{{0, "Development"}, {60, "Meeting"}}, nil)
			_, err := tr.EndDay(day.ID, ts(tc.endMin))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
		})
	}

	t.Run("already ended", func(t *testing.T) {
		tr := newTestTracker(t)
		day := buildDay(t, tr, []boundarySpec{{0, "Development"}}, intPtr(480))
		if _, err := tr.EndDay(day.ID, ts(490)); !errors.Is(err, ErrDayEnded) {
			t.Fatalf("err = %v, want ErrDayEnded", err)
		}
	})

	t.Run("SetDayEnd on open day", func(t *testing.T) {
		tr := newTestTracker(t)
		day := buildDay(t, tr, []boundarySpec{{0, "Development"}}, nil)
		if _, err := tr.SetDayEnd(day.ID, ts(480)); !errors.Is(err, ErrDayNotEnded) {
			t.Fatalf("err = %v, want ErrDayNotEnded", err)
		}
	})

	t.Run("SetDayEnd edits ended day", func(t *testing.T) {
		tr := newTestTracker(t)
		day := buildDay(t, tr, []boundarySpec{{0, "Development"}}, intPtr(480))
		got, err := tr.SetDayEnd(day.ID, ts(450))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.EndedAt == nil || !got.EndedAt.Equal(ts(450)) {
			t.Errorf("EndedAt = %v, want %v", got.EndedAt, ts(450))
		}
	})
}

func TestBackFromBreak(t *testing.T) {
	const dur = 45 * time.Minute
	tests := []struct {
		name    string
		setup   []boundarySpec
		ended   *int
		nowMin  int
		wantErr error
		want    []boundarySpec
	}{
		{
			name:   "simple: away for 45min while on Development",
			setup:  []boundarySpec{{0, "Development"}},
			nowMin: 120,
			want:   []boundarySpec{{0, "Development"}, {75, "Break"}, {120, "Development"}},
		},
		{
			name:   "day started less than 45min ago: break clamps to day start",
			setup:  []boundarySpec{{0, "Development"}},
			nowMin: 20,
			want:   []boundarySpec{{0, "Break"}, {20, "Development"}},
		},
		{
			name:   "day is exactly 45min old",
			setup:  []boundarySpec{{0, "Development"}},
			nowMin: 45,
			want:   []boundarySpec{{0, "Break"}, {45, "Development"}},
		},
		{
			name:   "switch inside break window is voided, latest activity resumes",
			setup:  []boundarySpec{{0, "Development"}, {110, "Meeting"}},
			nowMin: 120,
			want:   []boundarySpec{{0, "Development"}, {75, "Break"}, {120, "Meeting"}},
		},
		{
			name:   "currently on a manual break: resumes last non-break activity",
			setup:  []boundarySpec{{0, "Development"}, {90, "Break"}},
			nowMin: 120,
			want:   []boundarySpec{{0, "Development"}, {75, "Break"}, {120, "Development"}},
		},
		{
			name:    "day consists only of break",
			setup:   []boundarySpec{{0, "Break"}},
			nowMin:  60,
			wantErr: ErrNoResume,
		},
		{
			name:    "day already ended",
			setup:   []boundarySpec{{0, "Development"}},
			ended:   intPtr(480),
			nowMin:  490,
			wantErr: ErrDayEnded,
		},
		{
			name:    "now equals day start",
			setup:   []boundarySpec{{0, "Development"}},
			nowMin:  0,
			wantErr: ErrBeforeDayStart,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newTestTracker(t)
			day := buildDay(t, tr, tc.setup, tc.ended)
			_, err := tr.BackFromBreak(day.ID, ts(tc.nowMin), dur)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				// Failed operation must leave the day untouched.
				assertBoundaries(t, tr, day, tc.setup)
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			assertBoundaries(t, tr, day, tc.want)
		})
	}
}

func TestSummary(t *testing.T) {
	tr := newTestTracker(t)
	// 09:00 Dev, 12:00 Break, 12:45 Dev, 15:00 Meeting, ended 17:00.
	day := buildDay(t, tr, []boundarySpec{
		{0, "Development"}, {180, "Break"}, {225, "Development"}, {360, "Meeting"},
	}, intPtr(480))

	s, err := tr.Summary(day.ID, ts(600))
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if s.TotalSecs != 8*3600 {
		t.Errorf("TotalSecs = %d, want %d", s.TotalSecs, 8*3600)
	}
	if s.BreakSecs != 45*60 {
		t.Errorf("BreakSecs = %d, want %d", s.BreakSecs, 45*60)
	}
	if s.WorkSecs != 8*3600-45*60 {
		t.Errorf("WorkSecs = %d, want %d", s.WorkSecs, 8*3600-45*60)
	}
	if len(s.Segments) != 4 {
		t.Fatalf("segments = %d, want 4", len(s.Segments))
	}
	if s.Segments[3].End == nil || !s.Segments[3].End.Equal(ts(480)) {
		t.Errorf("last segment end = %v, want %v", s.Segments[3].End, ts(480))
	}
	// Per-activity: Dev 3h+2h15m, Meeting 2h, Break 45m — sorted desc.
	wantTotals := map[string]int64{
		"Development": 5*3600 + 15*60,
		"Meeting":     2 * 3600,
		"Break":       45 * 60,
	}
	if len(s.ByActivity) != len(wantTotals) {
		t.Fatalf("ByActivity = %+v, want %d entries", s.ByActivity, len(wantTotals))
	}
	for _, at := range s.ByActivity {
		if at.Seconds != wantTotals[at.Activity.Name] {
			t.Errorf("%s = %d secs, want %d", at.Activity.Name, at.Seconds, wantTotals[at.Activity.Name])
		}
	}
}

func TestSummaryRunningDay(t *testing.T) {
	tr := newTestTracker(t)
	day := buildDay(t, tr, []boundarySpec{{0, "Development"}}, nil)

	s, err := tr.Summary(day.ID, ts(90))
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if s.TotalSecs != 90*60 {
		t.Errorf("TotalSecs = %d, want %d", s.TotalSecs, 90*60)
	}
	if s.Segments[0].End != nil {
		t.Errorf("running segment must have nil End, got %v", *s.Segments[0].End)
	}
}

func TestEnsureActivity(t *testing.T) {
	tr := newTestTracker(t)

	a, err := tr.EnsureActivity("Code Review")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Case-insensitive + trimmed lookup must return the same activity.
	b, err := tr.EnsureActivity("  code review ")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if a.ID != b.ID {
		t.Errorf("expected same activity, got %d and %d", a.ID, b.ID)
	}
	// Presets resolve to their seeded rows.
	dev, err := tr.EnsureActivity("development")
	if err != nil {
		t.Fatalf("preset lookup: %v", err)
	}
	if !dev.IsPreset {
		t.Errorf("expected seeded preset, got %+v", dev)
	}
	if _, err := tr.EnsureActivity("   "); !errors.Is(err, ErrEmptyName) {
		t.Errorf("blank name err = %v, want ErrEmptyName", err)
	}
	// Unarchive on reuse.
	if err := tr.SetActivityArchived(a.ID, true); err != nil {
		t.Fatalf("archive: %v", err)
	}
	c, err := tr.EnsureActivity("Code Review")
	if err != nil {
		t.Fatalf("reuse archived: %v", err)
	}
	if c.Archived {
		t.Error("activity should be unarchived on reuse")
	}
}

func TestRangeSummary(t *testing.T) {
	tr := newTestTracker(t)
	dev := actID(t, tr, "Development")

	day1, err := tr.StartDay("2026-07-06", ts(0), dev)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tr.EndDay(day1.ID, ts(8*60)); err != nil {
		t.Fatal(err)
	}
	day2, err := tr.StartDay("2026-07-07", ts(24*60), dev)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tr.EndDay(day2.ID, ts(24*60+6*60)); err != nil {
		t.Fatal(err)
	}

	rs, err := tr.RangeSummary("2026-07-06", "2026-07-07", ts(48*60))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Days) != 2 || rs.Days[0].Date != "2026-07-06" {
		t.Fatalf("days = %+v, want oldest first", rs.Days)
	}
	if rs.TotalSecs != 14*3600 || rs.WorkSecs != 14*3600 || rs.BreakSecs != 0 {
		t.Errorf("totals = %d/%d/%d, want 14h/14h/0", rs.TotalSecs, rs.WorkSecs, rs.BreakSecs)
	}
}

package tracker

import (
	"time"
)

// Segments derives the segment list from a day's boundaries. For a running
// day the last segment has End == nil; totals treat it as ending at "now",
// which the caller supplies so elapsed time is always recomputed from
// timestamps (never accumulated).
func segments(day Day, acts map[int64]Activity) []Segment {
	segs := make([]Segment, 0, len(day.Boundaries))
	for i, b := range day.Boundaries {
		seg := Segment{
			BoundaryID: b.ID,
			Activity:   acts[b.ActivityID],
			Start:      b.At,
		}
		if i+1 < len(day.Boundaries) {
			end := day.Boundaries[i+1].At
			seg.End = &end
		} else if day.EndedAt != nil {
			seg.End = day.EndedAt
		}
		segs = append(segs, seg)
	}
	return segs
}

// Summary computes the full day summary. now is only used as the open end
// of the last segment on a still-running day.
func (t *Tracker) Summary(dayID int64, now time.Time) (DaySummary, error) {
	day, err := getDay(t.db, dayID)
	if err != nil {
		return DaySummary{}, err
	}
	acts, err := activityMap(t.db)
	if err != nil {
		return DaySummary{}, err
	}
	return summarize(day, acts, now), nil
}

func summarize(day Day, acts map[int64]Activity, now time.Time) DaySummary {
	now = now.UTC().Truncate(time.Second)
	segs := segments(day, acts)

	s := DaySummary{
		DayID:    day.ID,
		Date:     day.Date,
		Start:    day.Start(),
		End:      day.EndedAt,
		Segments: segs,
	}

	perActivity := map[int64]int64{}
	for _, seg := range segs {
		end := now
		if seg.End != nil {
			end = *seg.End
		}
		secs := int64(end.Sub(seg.Start) / time.Second)
		if secs < 0 { // running day viewed with a stale "now"
			secs = 0
		}
		perActivity[seg.Activity.ID] += secs
		s.TotalSecs += secs
		if seg.Activity.IsBreak {
			s.BreakSecs += secs
		}
	}
	s.WorkSecs = s.TotalSecs - s.BreakSecs

	for id, secs := range perActivity {
		s.ByActivity = append(s.ByActivity, ActivityTotal{Activity: acts[id], Seconds: secs})
	}
	sortTotals(s.ByActivity)
	return s
}

// RangeSummary aggregates all days within [from, to] (inclusive local date
// strings), e.g. one week. Days are ordered oldest first.
func (t *Tracker) RangeSummary(from, to string, now time.Time) (RangeSummary, error) {
	days, err := t.ListDays(from, to)
	if err != nil {
		return RangeSummary{}, err
	}
	acts, err := activityMap(t.db)
	if err != nil {
		return RangeSummary{}, err
	}

	rs := RangeSummary{From: from, To: to}
	perActivity := map[int64]int64{}
	// ListDays returns newest first; walk backwards for oldest-first output.
	for i := len(days) - 1; i >= 0; i-- {
		day, err := getDay(t.db, days[i].ID)
		if err != nil {
			return RangeSummary{}, err
		}
		ds := summarize(day, acts, now)
		rs.Days = append(rs.Days, ds)
		rs.TotalSecs += ds.TotalSecs
		rs.BreakSecs += ds.BreakSecs
		rs.WorkSecs += ds.WorkSecs
		for _, at := range ds.ByActivity {
			perActivity[at.Activity.ID] += at.Seconds
		}
	}
	for id, secs := range perActivity {
		rs.ByActivity = append(rs.ByActivity, ActivityTotal{Activity: acts[id], Seconds: secs})
	}
	sortTotals(rs.ByActivity)
	return rs, nil
}

import { useCallback, useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { TrackerService, type DayView } from "../../bindings/timetrack";
import type { RangeSummary } from "../../bindings/timetrack/internal/tracker/models";
import { useApp } from "../lib/app";
import { activitySolid } from "../lib/colors";
import { ActivityDot, DayBar } from "../components/DayBar";
import { DaySummary } from "../components/DaySummary";
import { InlineActivity, Timeline } from "../components/Timeline";
import {
  addDays,
  dateAtTime,
  fmtDate,
  fmtDecimalHours,
  fmtHM,
  startOfWeek,
  toISO,
  toTimeInput,
} from "../lib/time";

/** Commit-on-change time input bound to an ISO timestamp. */
function TimeField({
  iso,
  onCommit,
  disabled,
}: {
  iso: string;
  onCommit: (hhmm: string) => void;
  disabled?: boolean;
}) {
  // Keyed remount on iso keeps this simple: local value until committed.
  const [value, setValue] = useState(() => toTimeInput(iso));
  return (
    <input
      type="time"
      value={value}
      disabled={disabled}
      onChange={(e) => setValue(e.target.value)}
      onBlur={() => value && value !== toTimeInput(iso) && onCommit(value)}
      onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()}
      className="rounded-lg border border-zinc-700 bg-zinc-900 px-2 py-1 text-sm outline-none focus:border-indigo-500 disabled:opacity-40"
    />
  );
}

function DayEditor({ view, reload }: { view: DayView; reload: () => void }) {
  const { state, run } = useApp();
  const day = view.day;
  const first = (day.boundaries ?? [])[0];

  const setStart = (hhmm: string) =>
    void run(TrackerService.MoveBoundary(first.id, toISO(dateAtTime(day.date, hhmm))));
  const setEnd = (hhmm: string) =>
    void run(TrackerService.SetDayEnd(day.id, toISO(dateAtTime(day.date, hhmm))));

  return (
    <div className="space-y-6">
      <section className="rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">{fmtDate(day.date)}</h2>
          <div className="flex items-center gap-3 text-sm">
            <label className="text-zinc-400">Start</label>
            {first && <TimeField key={`s-${first.at}`} iso={first.at} onCommit={setStart} />}
            <label className="ml-2 text-zinc-400">End</label>
            {day.endedAt ? (
              <TimeField key={`e-${day.endedAt}`} iso={day.endedAt} onCommit={setEnd} />
            ) : (
              <span className="text-emerald-400">running</span>
            )}
            {day.endedAt && (
              <button
                onClick={() => void run(TrackerService.ReopenDay(day.id))}
                className="rounded-lg border border-zinc-700 px-3 py-1 hover:border-zinc-500"
              >
                Reopen
              </button>
            )}
            <button
              onClick={() => {
                if (confirm(`Delete ${day.date} entirely?`)) {
                  void run(TrackerService.DeleteDay(day.id)).then(reload);
                }
              }}
              className="rounded-lg border border-rose-900 px-3 py-1 text-rose-400 hover:border-rose-600"
            >
              Delete day
            </button>
          </div>
        </div>
        <p className="mb-3 text-xs text-zinc-500">
          Drag the line between two segments to move that boundary · split adds a boundary ·
          merge ↑ removes one · click an activity name to retag the segment.
        </p>
        <Timeline view={view} activities={state?.activities ?? []} />
        <BoundaryEditor view={view} />
      </section>

      <section className="rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6">
        <h2 className="mb-4 text-lg font-semibold">Booking summary</h2>
        <DaySummary view={view} />
      </section>
    </div>
  );
}

/** Exact-time editing of every segment, complementing the drag timeline. */
function BoundaryEditor({ view }: { view: DayView }) {
  const { state, run } = useApp();
  const day = view.day;
  const boundaries = day.boundaries ?? [];
  const segments = view.summary.segments ?? [];

  return (
    <div className="mt-6">
      <h3 className="mb-2 text-sm font-medium text-zinc-400">Exact times</h3>
      <table className="w-full text-sm">
        <tbody>
          {segments.map((seg, i) => {
            const b = boundaries[i];
            return (
              <tr key={b.id} className="border-t border-zinc-800">
                <td className="w-28 py-1.5 pr-3">
                  <TimeField
                    key={b.at}
                    iso={b.at}
                    onCommit={(hhmm) =>
                      void run(
                        TrackerService.MoveBoundary(b.id, toISO(dateAtTime(day.date, hhmm))),
                      )
                    }
                  />
                </td>
                <td className="py-1.5">
                  <InlineActivity
                    key={`${b.id}-${seg.activity.name}`}
                    name={seg.activity.name}
                    activities={state?.activities ?? []}
                    onChange={(name) => void run(TrackerService.SetBoundaryActivity(b.id, name))}
                  />
                </td>
                <td className="w-24 py-1.5 text-right">
                  {boundaries.length > 1 && (
                    <button
                      title={
                        i === 0
                          ? "Remove — the day then starts at the next boundary"
                          : "Remove — the previous segment absorbs this one"
                      }
                      onClick={() => void run(TrackerService.DeleteBoundary(b.id))}
                      className="rounded border border-zinc-700 px-2 py-0.5 text-xs text-zinc-400 hover:border-rose-700 hover:text-rose-400"
                    >
                      remove
                    </button>
                  )}
                </td>
              </tr>
            );
          })}
          <tr className="border-t border-zinc-800">
            <td className="py-1.5 pr-3">
              {day.endedAt ? (
                <TimeField
                  key={`end-${day.endedAt}`}
                  iso={day.endedAt}
                  onCommit={(hhmm) =>
                    void run(TrackerService.SetDayEnd(day.id, toISO(dateAtTime(day.date, hhmm))))
                  }
                />
              ) : (
                <span className="text-emerald-400">running</span>
              )}
            </td>
            <td className="py-1.5 text-zinc-500" colSpan={2}>
              day end
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}

function WeekSummary({ range }: { range: RangeSummary }) {
  const { run } = useApp();
  return (
    <section className="rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold">Week summary</h2>
        <button
          onClick={() => void run(TrackerService.ExportCSV(range.from, range.to))}
          className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm hover:border-zinc-500"
        >
          Export CSV
        </button>
      </div>
      <div className="grid grid-cols-2 gap-8">
        <table className="selectable w-full self-start text-sm">
          <thead>
            <tr className="text-left text-xs uppercase text-zinc-500">
              <th className="pb-2 font-medium">Day</th>
              <th className="pb-2 font-medium"></th>
              <th className="pb-2 text-right font-medium">Total</th>
              <th className="pb-2 text-right font-medium">Break</th>
              <th className="pb-2 text-right font-medium">Net</th>
            </tr>
          </thead>
          <tbody>
            {(range.days ?? []).map((d) => (
              <tr key={d.dayId} className="border-t border-zinc-800">
                <td className="py-1.5 pr-2 whitespace-nowrap">{fmtDate(d.date)}</td>
                <td className="w-24 py-1.5 pr-2 align-middle">
                  <DayBar segments={d.segments ?? []} className="h-1.5!" />
                </td>
                <td className="py-1.5 text-right tabular-nums">{fmtHM(d.totalSecs)}</td>
                <td className="py-1.5 text-right tabular-nums text-zinc-400">{fmtHM(d.breakSecs)}</td>
                <td className="py-1.5 text-right font-medium tabular-nums">{fmtHM(d.workSecs)}</td>
              </tr>
            ))}
            <tr className="border-t border-zinc-700 font-semibold">
              <td className="py-1.5">Week</td>
              <td className="py-1.5 text-right tabular-nums">{fmtHM(range.totalSecs)}</td>
              <td className="py-1.5 text-right tabular-nums text-zinc-400">{fmtHM(range.breakSecs)}</td>
              <td className="py-1.5 text-right tabular-nums">{fmtHM(range.workSecs)}</td>
            </tr>
          </tbody>
        </table>
        <table className="selectable w-full self-start text-sm">
          <thead>
            <tr className="text-left text-xs uppercase text-zinc-500">
              <th className="pb-2 font-medium">Activity</th>
              <th className="pb-2 text-right font-medium">Total</th>
              <th className="pb-2 text-right font-medium">Hours</th>
            </tr>
          </thead>
          <tbody>
            {(range.byActivity ?? []).map((t) => (
              <tr key={t.activity.id} className="border-t border-zinc-800">
                <td className="py-1.5">
                  <ActivityDot color={activitySolid(t.activity)} />
                  {t.activity.name}
                </td>
                <td className="py-1.5 text-right tabular-nums">{fmtHM(t.seconds)}</td>
                <td className="py-1.5 text-right tabular-nums text-zinc-500">
                  {fmtDecimalHours(t.seconds)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export function ReviewView() {
  const { state } = useApp();
  const today = state?.today ?? "";
  const [weekStart, setWeekStart] = useState(() => startOfWeek(today || new Date().toISOString().slice(0, 10)));
  const [selected, setSelected] = useState<string | null>(null);
  const [range, setRange] = useState<RangeSummary | null>(null);
  const [dayView, setDayView] = useState<DayView | null>(null);

  const weekEnd = addDays(weekStart, 6);

  const reload = useCallback(async () => {
    const rs = await TrackerService.RangeSummary(weekStart, weekEnd);
    setRange(rs);
    // Auto-select the most recent day of the week so the editor is
    // immediately visible; keep an explicit selection while it exists.
    const days = rs.days ?? [];
    let sel = selected;
    if (!sel || !days.some((d) => d.date === sel)) {
      sel = days.length ? days[days.length - 1].date : null;
      setSelected(sel);
    }
    setDayView(sel ? await TrackerService.DayByDate(sel) : null);
  }, [weekStart, weekEnd, selected]);

  useEffect(() => {
    void reload();
    // Any mutation (from this view, the live view or the quick window)
    // triggers a reload so edits are always reflected.
    return Events.On("day:changed", () => void reload());
  }, [reload]);

  const dayTabs = (range?.days ?? []).slice();

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setWeekStart(addDays(weekStart, -7))}
            className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm hover:border-zinc-500"
          >
            ← prev
          </button>
          <span className="min-w-48 text-center text-sm text-zinc-300">
            {fmtDate(weekStart)} – {fmtDate(weekEnd)}
          </span>
          <button
            onClick={() => setWeekStart(addDays(weekStart, 7))}
            className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm hover:border-zinc-500"
          >
            next →
          </button>
          <button
            onClick={() => setWeekStart(startOfWeek(today))}
            className="ml-2 rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-400 hover:border-zinc-500"
          >
            this week
          </button>
        </div>
        <div className="flex gap-1">
          {dayTabs.map((d) => (
            <button
              key={d.dayId}
              onClick={() => setSelected(d.date)}
              className={
                "rounded-lg px-3 py-1.5 text-sm " +
                (selected === d.date
                  ? "bg-indigo-600/30 text-indigo-200"
                  : "border border-zinc-800 text-zinc-400 hover:text-zinc-200")
              }
            >
              {fmtDate(d.date)}
            </button>
          ))}
          {dayTabs.length === 0 && <span className="text-sm text-zinc-500">no tracked days</span>}
        </div>
      </div>

      {dayView && <DayEditor view={dayView} reload={() => setSelected(null)} />}
      {range && <WeekSummary range={range} />}
    </div>
  );
}

import { useEffect, useState } from "react";
import { TrackerService, type DayView } from "../../bindings/timetrack";
import { useApp } from "../lib/app";
import { ActivityInput, PresetButtons } from "../components/ActivityInput";
import { DaySummary } from "../components/DaySummary";
import {
  dateAtTime,
  fmtClock,
  fmtDate,
  fmtHMS,
  nowTimeInput,
  parseTs,
  secondsBetween,
  toISO,
} from "../lib/time";

function Card({ title, children }: { title?: string; children: React.ReactNode }) {
  return (
    <section className="rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6">
      {title && <h2 className="mb-4 text-lg font-semibold">{title}</h2>}
      {children}
    </section>
  );
}

/** Start-day form: editable start time (defaults to now) + activity. */
function StartDay() {
  const { state, now, run } = useApp();
  const [time, setTime] = useState(nowTimeInput);
  const [activity, setActivity] = useState<string>();
  const [touched, setTouched] = useState(false);

  // Keep the default time following the clock until the user edits it.
  useEffect(() => {
    if (!touched) setTime(nowTimeInput());
  }, [now, touched]);

  if (!state) return null;
  const chosen = activity ?? state.defaultActivity;

  const start = () =>
    void run(TrackerService.StartDay(toISO(dateAtTime(state.today, time)), chosen));

  return (
    <Card title="Start your day">
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <label className="text-sm text-zinc-400">Start at</label>
          <input
            type="time"
            value={time}
            onChange={(e) => {
              setTouched(true);
              setTime(e.target.value);
            }}
            className="rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm outline-none focus:border-indigo-500"
          />
          <label className="ml-4 text-sm text-zinc-400">First activity</label>
          <select
            value={chosen}
            onChange={(e) => setActivity(e.target.value)}
            className="rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm outline-none focus:border-indigo-500"
          >
            {(state.activities ?? []).map((a) => (
              <option key={a.id} value={a.name}>
                {a.name}
              </option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={start}
            className="rounded-lg bg-emerald-600 px-5 py-2.5 font-medium hover:bg-emerald-500"
          >
            Start Day
          </button>
          {chosen !== state.defaultActivity && (
            <button
              onClick={() => void run(TrackerService.SetDefaultActivity(chosen))}
              className="text-sm text-zinc-400 underline-offset-2 hover:underline"
            >
              make “{chosen}” the default
            </button>
          )}
        </div>
      </div>
    </Card>
  );
}

/** Live tracking: current activity + ticking elapsed, switch, break, end. */
function LiveDay({ view }: { view: DayView }) {
  const { state, now, run } = useApp();
  const [endTime, setEndTime] = useState(nowTimeInput);
  const [endTouched, setEndTouched] = useState(false);
  useEffect(() => {
    if (!endTouched) setEndTime(nowTimeInput());
  }, [now, endTouched]);

  const boundaries = view.day.boundaries ?? [];
  const last = boundaries[boundaries.length - 1];
  const segments = view.summary.segments ?? [];
  const current = segments[segments.length - 1]?.activity;

  // Elapsed values are recomputed from stored timestamps on every tick, so
  // they survive restarts, sleep and crashes.
  const elapsed = last ? secondsBetween(parseTs(last.at), now) : 0;
  const dayElapsed = secondsBetween(parseTs(view.summary.start), now);

  const notToday = state && view.day.date !== state.today;

  return (
    <div className="space-y-6">
      {notToday && (
        <div className="rounded-lg border border-amber-600/40 bg-amber-950/40 px-4 py-2 text-sm text-amber-300">
          This day was started on {fmtDate(view.day.date)} and is still running.
        </div>
      )}

      <Card>
        <div className="flex items-end justify-between">
          <div>
            <div className="text-sm text-zinc-400">
              Current activity · since {last ? fmtClock(parseTs(last.at)) : ""}
            </div>
            <div className="mt-1 text-3xl font-semibold">{current?.name ?? "—"}</div>
          </div>
          <div className="text-right">
            <div className="text-4xl font-semibold tabular-nums">{fmtHMS(elapsed)}</div>
            <div className="mt-1 text-xs text-zinc-500 tabular-nums">
              day total {fmtHMS(dayElapsed)} · started {fmtClock(parseTs(view.summary.start))}
            </div>
          </div>
        </div>
      </Card>

      <Card title="Switch activity">
        <div className="space-y-3">
          <PresetButtons
            activities={state?.activities ?? []}
            current={current?.name}
            onPick={(name) => void run(TrackerService.SwitchActivity(name))}
          />
          <ActivityInput
            activities={state?.activities ?? []}
            buttonLabel="Switch"
            onSubmit={(name) => void run(TrackerService.SwitchActivity(name))}
          />
          <button
            onClick={() => void run(TrackerService.BackFromBreak())}
            className="rounded-lg border border-amber-600/50 bg-amber-950/40 px-4 py-2 text-sm font-medium text-amber-300 hover:bg-amber-900/40"
          >
            ☕ Back from break — books the last {state?.breakMinutes ?? 45} min
          </button>
        </div>
      </Card>

      <Card title="End day">
        <div className="flex items-center gap-3">
          <label className="text-sm text-zinc-400">End at</label>
          <input
            type="time"
            value={endTime}
            onChange={(e) => {
              setEndTouched(true);
              setEndTime(e.target.value);
            }}
            className="rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm outline-none focus:border-indigo-500"
          />
          <button
            onClick={() =>
              void run(TrackerService.EndDay(toISO(dateAtTime(view.day.date, endTime))))
            }
            className="rounded-lg bg-rose-700 px-5 py-2 font-medium hover:bg-rose-600"
          >
            End Day
          </button>
        </div>
      </Card>
    </div>
  );
}

/** Shown when today's day has been ended: the booking summary. */
function EndedDay({ view }: { view: DayView }) {
  const { run } = useApp();
  return (
    <div className="space-y-6">
      <Card title={`Day complete — ${fmtDate(view.day.date)}`}>
        <DaySummary view={view} />
        <div className="mt-6">
          <button
            onClick={() => void run(TrackerService.ReopenDay(view.day.id))}
            className="rounded-lg border border-zinc-700 px-4 py-2 text-sm hover:border-zinc-500"
          >
            Reopen day
          </button>
        </div>
      </Card>
    </div>
  );
}

export function TodayView() {
  const { state } = useApp();
  if (!state) return null;

  if (state.openDay) return <LiveDay view={state.openDay} />;
  if (state.todayDay) return <EndedDay view={state.todayDay} />;
  return <StartDay />;
}

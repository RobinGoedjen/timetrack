import { TrackerService } from "../../bindings/timetrack";
import { useApp } from "../lib/app";
import { ActivityInput, PresetButtons } from "../components/ActivityInput";
import { ErrorToast } from "../components/ErrorToast";
import { fmtClock, fmtHMS, parseTs, secondsBetween, toISO } from "../lib/time";

/**
 * Compact always-on-top popup attached to the tray icon: current activity,
 * elapsed time, preset + free-text switching, back-from-break and end day.
 */
export function QuickView() {
  const { state, now, run } = useApp();
  if (!state) return null;

  const view = state.openDay;
  const boundaries = view?.day.boundaries ?? [];
  const last = boundaries[boundaries.length - 1];
  const segments = view?.summary.segments ?? [];
  const current = segments[segments.length - 1]?.activity;
  // Today's day exists but was ended: offer reopen instead of a doomed
  // "start day" (the date is unique, starting again would fail).
  const endedToday = !view && state.todayDay ? state.todayDay : null;

  return (
    <div className="flex h-screen flex-col gap-3 overflow-y-auto border border-zinc-700 bg-zinc-950 p-4">
      {view && last ? (
        <>
          <div className="flex items-baseline justify-between">
            <div>
              <div className="text-xs text-zinc-400">since {fmtClock(parseTs(last.at))}</div>
              <div className="text-lg font-semibold">{current?.name}</div>
            </div>
            <div className="text-2xl font-semibold tabular-nums">
              {fmtHMS(secondsBetween(parseTs(last.at), now))}
            </div>
          </div>

          <PresetButtons
            activities={state.activities ?? []}
            current={current?.name}
            onPick={(name) => void run(TrackerService.SwitchActivity(name))}
          />

          <ActivityInput
            activities={state.activities ?? []}
            buttonLabel="Switch"
            placeholder="Any activity…"
            onSubmit={(name) => void run(TrackerService.SwitchActivity(name))}
          />

          <button
            onClick={() => void run(TrackerService.BackFromBreak())}
            className="rounded-lg border border-amber-600/50 bg-amber-950/40 px-3 py-2 text-sm font-medium text-amber-300 hover:bg-amber-900/40"
          >
            ☕ Back from break ({state.breakMinutes} min)
          </button>

          <div className="mt-auto">
            <button
              onClick={() => void run(TrackerService.EndDay(toISO(new Date())))}
              className="w-full rounded-lg bg-rose-800 px-3 py-2 text-sm font-medium hover:bg-rose-700"
            >
              End day at {fmtClock(now)}
            </button>
          </div>
        </>
      ) : endedToday ? (
        <>
          <div className="text-sm text-zinc-400">
            Today's day ended
            {endedToday.day.endedAt ? ` at ${fmtClock(parseTs(endedToday.day.endedAt))}` : ""}.
          </div>
          <button
            onClick={() => void run(TrackerService.ReopenDay(endedToday.day.id))}
            className="rounded-lg bg-emerald-700 px-3 py-2 text-sm font-medium hover:bg-emerald-600"
          >
            Reopen day
          </button>
        </>
      ) : (
        <>
          <div className="text-sm text-zinc-400">No day running.</div>
          <button
            onClick={() => void run(TrackerService.StartDay(toISO(new Date()), state.defaultActivity))}
            className="rounded-lg bg-emerald-700 px-3 py-2 text-sm font-medium hover:bg-emerald-600"
          >
            Start day now · {state.defaultActivity}
          </button>
        </>
      )}
      <ErrorToast />
    </div>
  );
}

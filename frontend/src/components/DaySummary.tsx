import type { DayView } from "../../bindings/timetrack";
import { activitySolid } from "../lib/colors";
import { fmtClock, fmtDecimalHours, fmtHM, parseTs } from "../lib/time";
import { ActivityDot, DayBar } from "./DayBar";

function Stat({ label, secs, accent }: { label: string; secs: number; accent?: string }) {
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/60 p-4">
      <div className="text-xs uppercase tracking-wide text-zinc-400">{label}</div>
      <div className={"selectable mt-1 text-2xl font-semibold tabular-nums " + (accent ?? "")}>
        {fmtHM(secs)}
      </div>
      <div className="selectable text-xs text-zinc-500 tabular-nums">{fmtDecimalHours(secs)} h</div>
    </div>
  );
}

/**
 * The copy-friendly day summary: distribution bar, totals, per-activity
 * table and the full timeline. Values are selectable so they can be pasted
 * into the employer's booking system.
 */
export function DaySummary({ view }: { view: DayView }) {
  const s = view.summary;
  return (
    <div className="space-y-6">
      <DayBar segments={s.segments ?? []} />

      <div className="grid grid-cols-3 gap-3">
        <Stat label="Total" secs={s.totalSecs} />
        <Stat label="Break" secs={s.breakSecs} accent="text-amber-400" />
        <Stat label="Net work" secs={s.workSecs} accent="text-emerald-400" />
      </div>

      <div>
        <h3 className="mb-2 text-sm font-medium text-zinc-400">Per activity</h3>
        <table className="selectable w-full text-sm">
          <tbody>
            {(s.byActivity ?? []).map((t) => (
              <tr key={t.activity.id} className="border-t border-zinc-800">
                <td className="py-1.5 pr-4">
                  <ActivityDot color={activitySolid(t.activity)} />
                  {t.activity.name}
                  {t.activity.isBreak && <span className="ml-2 text-xs text-zinc-500">break</span>}
                </td>
                <td className="py-1.5 pr-4 text-right tabular-nums">{fmtHM(t.seconds)}</td>
                <td className="py-1.5 text-right tabular-nums text-zinc-500">
                  {fmtDecimalHours(t.seconds)} h
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div>
        <h3 className="mb-2 text-sm font-medium text-zinc-400">Timeline</h3>
        <table className="selectable w-full text-sm">
          <tbody>
            {(s.segments ?? []).map((seg) => {
              const end = seg.end ? parseTs(seg.end) : null;
              return (
                <tr key={seg.boundaryId} className="border-t border-zinc-800">
                  <td className="w-36 py-1.5 pr-4 tabular-nums">
                    {fmtClock(parseTs(seg.start))} – {end ? fmtClock(end) : "…"}
                  </td>
                  <td className={"py-1.5 " + (seg.activity.isBreak ? "text-amber-400" : "")}>
                    <ActivityDot color={activitySolid(seg.activity)} />
                    {seg.activity.name}
                  </td>
                  <td className="py-1.5 text-right tabular-nums text-zinc-500">
                    {end ? fmtHM((end.getTime() - parseTs(seg.start).getTime()) / 1000) : "running"}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

import type { Segment } from "../../bindings/timetrack/internal/tracker/models";
import { activitySolid } from "../lib/colors";
import { fmtHM, parseTs } from "../lib/time";

/**
 * Proportional horizontal bar showing how a day was spent — the at-a-glance
 * view. Segment widths are shares of the day; colors match the timeline.
 */
export function DayBar({
  segments,
  className = "",
}: {
  segments: Segment[];
  className?: string;
}) {
  if (segments.length === 0) return null;
  const nowMs = Date.now();
  const spans = segments.map((seg) => {
    const start = parseTs(seg.start).getTime();
    const end = seg.end ? parseTs(seg.end).getTime() : nowMs;
    return { seg, dur: Math.max(end - start, 0) };
  });
  const total = spans.reduce((sum, s) => sum + s.dur, 0) || 1;

  return (
    <div className={"flex h-2.5 w-full overflow-hidden rounded-full " + className}>
      {spans.map(({ seg, dur }) => (
        <div
          key={seg.boundaryId}
          title={`${seg.activity.name} · ${fmtHM(dur / 1000)}`}
          style={{ width: `${(dur / total) * 100}%`, background: activitySolid(seg.activity) }}
        />
      ))}
    </div>
  );
}

/** Small colored dot used next to activity names in tables. */
export function ActivityDot({ color }: { color: string }) {
  return (
    <span
      className="mr-2 inline-block size-2.5 shrink-0 rounded-full align-middle"
      style={{ background: color }}
    />
  );
}

import { useRef, useState } from "react";
import { TrackerService, type DayView } from "../../bindings/timetrack";
import type { Activity } from "../../bindings/timetrack/internal/tracker/models";
import { useApp } from "../lib/app";
import { activityFill } from "../lib/colors";
import { fmtClock, fmtHM, parseTs, toISO } from "../lib/time";

const SNAP_MINUTES = 1;

/**
 * Editable vertical timeline of one day.
 *
 * The gapless model maps directly onto the interactions:
 *  - dragging the handle between two segments = MoveBoundary (resizes both)
 *  - "split" inside a segment = AddBoundary at its midpoint
 *  - "merge ↑" on a segment = DeleteBoundary (previous segment absorbs it)
 *  - the activity label is an inline free-text input with autocomplete
 */
export function Timeline({ view, activities }: { view: DayView; activities: Activity[] }) {
  const { now, run } = useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  // While dragging we preview the boundary position locally and only call
  // the backend on release.
  const [drag, setDrag] = useState<{ boundaryId: number; atMs: number } | null>(null);

  const segments = view.summary.segments ?? [];
  const boundaries = view.day.boundaries ?? [];
  if (segments.length === 0) return null;

  const startMs = parseTs(view.summary.start).getTime();
  const endMs = view.summary.end ? parseTs(view.summary.end).getTime() : now.getTime();

  // Scale the day to a comfortable height: short test days zoom in (up to
  // 24px per minute) so their boundaries stay draggable, long days compress
  // (down to 1.2px per minute).
  const totalMins = Math.max((endMs - startMs) / 60000, 1);
  const pxPerMin = Math.min(24, Math.max(1.2, 560 / totalMins));

  const yOf = (ms: number) => ((ms - startMs) / 60000) * pxPerMin;
  const height = Math.max(yOf(endMs), 60);

  const msAtPointer = (clientY: number) => {
    const rect = containerRef.current!.getBoundingClientRect();
    const mins = (clientY - rect.top) / pxPerMin;
    const snapped = Math.round(mins / SNAP_MINUTES) * SNAP_MINUTES;
    return startMs + snapped * 60000;
  };

  const startDrag = (boundaryId: number, prevMs: number, nextMs: number) => {
    return (e: React.PointerEvent) => {
      e.preventDefault();
      (e.target as Element).setPointerCapture(e.pointerId);
      const clamp = (ms: number) => Math.min(Math.max(ms, prevMs + 60000), nextMs - 60000);
      const move = (ev: PointerEvent) => setDrag({ boundaryId, atMs: clamp(msAtPointer(ev.clientY)) });
      const up = (ev: PointerEvent) => {
        window.removeEventListener("pointermove", move);
        window.removeEventListener("pointerup", up);
        const atMs = clamp(msAtPointer(ev.clientY));
        setDrag(null);
        void run(TrackerService.MoveBoundary(boundaryId, toISO(new Date(atMs))));
      };
      window.addEventListener("pointermove", move);
      window.addEventListener("pointerup", up);
    };
  };

  // Effective boundary times, with the dragged one overridden for preview.
  const timeOf = (boundaryId: number, iso: string) =>
    drag?.boundaryId === boundaryId ? drag.atMs : parseTs(iso).getTime();

  return (
    <div ref={containerRef} className="relative select-none" style={{ height }}>
      {segments.map((seg, i) => {
        const b = boundaries[i];
        const segStart = timeOf(b.id, seg.start);
        const segEnd =
          i + 1 < boundaries.length
            ? timeOf(boundaries[i + 1].id, boundaries[i + 1].at)
            : endMs;
        const midMs = segStart + (segEnd - segStart) / 2;
        const h = yOf(segEnd) - yOf(segStart);
        return (
          <div
            key={seg.boundaryId}
            className="absolute right-0 left-0 overflow-hidden rounded-md border border-zinc-700/60 px-3 py-1"
            style={{ top: yOf(segStart), height: h, background: activityFill(seg.activity) }}
          >
            <div className="flex items-center justify-between gap-2 text-sm">
              <InlineActivity
                key={`${seg.boundaryId}-${seg.activity.name}`}
                name={seg.activity.name}
                activities={activities}
                onChange={(name) => void run(TrackerService.SetBoundaryActivity(b.id, name))}
              />
              <span className="whitespace-nowrap text-xs text-zinc-300 tabular-nums">
                {fmtClock(new Date(segStart))}–{seg.end || i + 1 < segments.length ? fmtClock(new Date(segEnd)) : "now"}
                <span className="ml-2 text-zinc-400">{fmtHM((segEnd - segStart) / 1000)}</span>
              </span>
              {h >= 34 && (
                <span className="flex gap-1">
                  {segEnd - segStart >= 2 * 60000 && (
                    <button
                      title="Split this segment"
                      onClick={() =>
                        void run(
                          TrackerService.AddBoundary(
                            view.day.id,
                            toISO(new Date(midMs)),
                            seg.activity.name,
                          ),
                        )
                      }
                      className="rounded border border-zinc-600/60 px-1.5 text-xs text-zinc-300 hover:bg-zinc-800/60"
                    >
                      split
                    </button>
                  )}
                  {i > 0 && (
                    <button
                      title="Merge into the segment above"
                      onClick={() => void run(TrackerService.DeleteBoundary(b.id))}
                      className="rounded border border-zinc-600/60 px-1.5 text-xs text-zinc-300 hover:bg-zinc-800/60"
                    >
                      merge ↑
                    </button>
                  )}
                </span>
              )}
            </div>
          </div>
        );
      })}

      {/* Drag handles on internal boundaries. */}
      {boundaries.map((b, i) => {
        if (i === 0) return null;
        const prevMs = timeOf(boundaries[i - 1].id, boundaries[i - 1].at);
        const nextMs =
          i + 1 < boundaries.length ? timeOf(boundaries[i + 1].id, boundaries[i + 1].at) : endMs;
        const ms = timeOf(b.id, b.at);
        return (
          <div
            key={b.id}
            onPointerDown={startDrag(b.id, prevMs, nextMs)}
            className="group absolute right-0 left-0 z-10 -translate-y-1/2 cursor-row-resize py-1.5"
            style={{ top: yOf(ms) }}
          >
            <div className="h-0.5 bg-zinc-400/40 group-hover:bg-indigo-400" />
            <div
              className={
                "absolute left-2 top-1/2 -translate-y-1/2 rounded px-1.5 text-[10px] tabular-nums " +
                (drag?.boundaryId === b.id
                  ? "block bg-indigo-600 font-semibold text-white"
                  : "hidden bg-zinc-800 text-zinc-300 group-hover:block")
              }
            >
              {fmtClock(new Date(ms))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

/** Inline activity editor: free text + autocomplete, commits on Enter/blur. */
export function InlineActivity({
  name,
  activities,
  onChange,
}: {
  name: string;
  activities: Activity[];
  onChange: (name: string) => void;
}) {
  const [value, setValue] = useState(name);
  const listId = `acts-${name.replace(/\W/g, "")}`;
  const commit = () => {
    const v = value.trim();
    if (v && v !== name) onChange(v);
  };
  return (
    <>
      <input
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()}
        list={listId}
        className="min-w-0 flex-1 rounded bg-transparent px-1 font-medium outline-none focus:bg-zinc-900/80"
      />
      <datalist id={listId}>
        {activities.map((a) => (
          <option key={a.id} value={a.name} />
        ))}
      </datalist>
    </>
  );
}

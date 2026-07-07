import type { Activity } from "../../bindings/timetrack/internal/tracker/models";

// One stable hue per activity name so the timeline, distribution bars and
// summary tables all agree. Breaks are always amber.

function hue(name: string): number {
  let h = 0;
  for (const c of name) h = (h * 31 + c.charCodeAt(0)) % 360;
  return h;
}

/** Translucent fill for large timeline blocks. */
export function activityFill(a: Activity): string {
  if (a.isBreak) return "hsl(38 85% 50% / 0.35)";
  return `hsl(${hue(a.name)} 55% 45% / 0.35)`;
}

/** Solid color for dots and distribution bars. */
export function activitySolid(a: Activity): string {
  if (a.isBreak) return "hsl(38 85% 50%)";
  return `hsl(${hue(a.name)} 55% 52%)`;
}

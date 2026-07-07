// Time helpers. Backend timestamps are RFC3339 strings (UTC); everything is
// converted to local time here, at the display edge.

export function parseTs(iso: string): Date {
  return new Date(iso);
}

/** Local wall clock, e.g. "09:07". */
export function fmtClock(d: Date): string {
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });
}

/** "7:15 h" style duration from seconds (what booking systems want). */
export function fmtHM(secs: number): string {
  const s = Math.max(0, Math.round(secs));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return `${h}:${String(m).padStart(2, "0")}`;
}

/** Decimal hours, e.g. "7.25". */
export function fmtDecimalHours(secs: number): string {
  return (Math.max(0, secs) / 3600).toFixed(2);
}

/** Ticking display with seconds, e.g. "1:07:42". */
export function fmtHMS(secs: number): string {
  const s = Math.max(0, Math.floor(secs));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const r = s % 60;
  return `${h}:${String(m).padStart(2, "0")}:${String(r).padStart(2, "0")}`;
}

export function secondsBetween(a: Date, b: Date): number {
  return (b.getTime() - a.getTime()) / 1000;
}

/** Current local time as an <input type="time"> value. */
export function nowTimeInput(): string {
  return fmtClock(new Date());
}

/** Local "HH:MM" input value for a timestamp. */
export function toTimeInput(iso: string): string {
  return fmtClock(parseTs(iso));
}

/** Combine a local date string (YYYY-MM-DD) and "HH:MM" into a Date. */
export function dateAtTime(date: string, hhmm: string): Date {
  const [y, mo, d] = date.split("-").map(Number);
  const [h, mi] = hhmm.split(":").map(Number);
  return new Date(y, mo - 1, d, h, mi, 0, 0);
}

export function toISO(d: Date): string {
  return d.toISOString();
}

/** Local date string YYYY-MM-DD. */
export function localDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

/** Human date, e.g. "Mon, Jul 6". */
export function fmtDate(date: string): string {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(y, m - 1, d).toLocaleDateString([], {
    weekday: "short",
    month: "short",
    day: "numeric",
  });
}

export function addDays(date: string, days: number): string {
  const [y, m, d] = date.split("-").map(Number);
  const dt = new Date(y, m - 1, d);
  dt.setDate(dt.getDate() + days);
  return localDate(dt);
}

/** Monday of the week containing the given local date. */
export function startOfWeek(date: string): string {
  const [y, m, d] = date.split("-").map(Number);
  const dt = new Date(y, m - 1, d);
  const shift = (dt.getDay() + 6) % 7; // Mon=0 ... Sun=6
  dt.setDate(dt.getDate() - shift);
  return localDate(dt);
}

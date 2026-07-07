# timetrack

Personal Windows desktop app for gapless working-time tracking. Track your
day live as a sequence of activities, review and edit past days, and copy
the daily totals into your employer's booking system.

Built with [Wails v3](https://v3.wails.io) (Go backend, React/TypeScript
frontend), SQLite, and Tailwind CSS. Fully offline: no network access, no
telemetry, all data stays on your machine.

## Core idea: gapless by construction

A day is **not** a list of independent blocks with start and end. It is an
ordered list of boundary timestamps, each tagged with the activity that
*starts* at that moment, plus a single day-end time. A segment ends where
the next one begins, so gaps and overlaps are structurally impossible —
every edit (drag, split, merge) keeps the day continuous.

## Features

- **Start day** with an editable start time and configurable default activity
- **Live tracking**: current activity and elapsed time, derived from stored
  timestamps on every tick (survives restarts, sleep and crashes)
- **Switch activity** via one-click pinned buttons or free text with
  autocomplete (new names create new activities)
- **Back from break**: retroactively books the last 45 minutes as break and
  resumes the previous activity
- **End day** with a copy-friendly summary: total / break / net work,
  per-activity totals (h:mm and decimal hours), full timeline
- **Review & edit**: week navigation, per-day editable timeline (drag
  boundaries, split and merge segments, retag activities, exact-time
  inputs), weekly summary, CSV export
- **System tray**: left-click opens an always-on-top quick popup (switch,
  back-from-break, end day); closing the main window hides to the tray

## Development

Prerequisites: Go ≥ 1.25, pnpm, the `wails3` CLI
(`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`).

```sh
wails3 dev                                  # run with hot reload
go test ./internal/...                      # domain-layer tests
wails3 task windows:build PRODUCTION=true   # production build -> bin/timetrack.exe
```

Bindings for the frontend are generated from the Go service methods
(`wails3 generate bindings -clean=true -ts -i`); `wails3 dev` does this
automatically.

## Architecture

| Layer | Location | Responsibility |
|---|---|---|
| Store | `internal/store` | SQLite (pure Go driver), embedded SQL migrations |
| Domain | `internal/tracker` | All business rules and invariants, table-driven tests |
| Service | `service.go` | Wails-bound facade: ISO/local-time conversion, change events |
| Frontend | `frontend/` | Thin React view over generated TypeScript bindings |

Every mutation runs in a transaction and re-validates the whole day
(strictly increasing boundaries, day end after the last boundary) before
committing. Times are stored as UTC RFC3339 strings, aligned to whole
minutes, and converted to local time at the edges.

## Data & deployment

- Database: `%APPDATA%\timetrack\timetrack.db` (delete to reset)
- The production exe is a single self-contained binary; the only external
  dependency is the WebView2 runtime (preinstalled on Windows 10/11)
- `timetrack.exe --hidden` starts directly to the tray — point a shortcut in
  `shell:startup` at it for silent autostart

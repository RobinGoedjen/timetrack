-- Initial schema.
--
-- A day is an ordered list of boundary timestamps. Each boundary is tagged
-- with the activity that STARTS at that moment; a segment's end is implicitly
-- the next boundary (or days.ended_at for the last segment). Gaps are
-- structurally impossible.
--
-- Design note: the "day end" is stored in days.ended_at rather than as a
-- NULL-activity boundary row, so activity_id can be NOT NULL here. One
-- representation of the day end instead of two that could disagree.

CREATE TABLE days (
    id       INTEGER PRIMARY KEY,
    date     TEXT NOT NULL UNIQUE, -- local calendar date, YYYY-MM-DD
    ended_at TEXT                  -- UTC RFC3339; NULL while the day is running
);

CREATE TABLE activities (
    id        INTEGER PRIMARY KEY,
    name      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    is_break  INTEGER NOT NULL DEFAULT 0,
    is_preset INTEGER NOT NULL DEFAULT 0,
    archived  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE boundaries (
    id          INTEGER PRIMARY KEY,
    day_id      INTEGER NOT NULL REFERENCES days (id) ON DELETE CASCADE,
    at          TEXT NOT NULL, -- UTC RFC3339
    activity_id INTEGER NOT NULL REFERENCES activities (id),
    UNIQUE (day_id, at)
);

CREATE INDEX idx_boundaries_day_at ON boundaries (day_id, at);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO activities (name, is_break, is_preset) VALUES
    ('Daily Standup', 0, 1),
    ('Break',         1, 1),
    ('Meeting',       0, 1),
    ('Development',   0, 1);

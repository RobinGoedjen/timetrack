import { useCallback, useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { TrackerService } from "../../bindings/timetrack";
import type { Activity } from "../../bindings/timetrack/internal/tracker/models";
import { useApp } from "../lib/app";
import { ActivityInput } from "../components/ActivityInput";

/**
 * Manage the activity list: pin activities as one-click buttons in the
 * switch panels, choose the day-start default, and archive old ones.
 */
export function ActivitiesView() {
  const { state, run } = useApp();
  const [acts, setActs] = useState<Activity[]>([]);

  const reload = useCallback(async () => {
    setActs((await TrackerService.Activities(true)) ?? []);
  }, []);

  useEffect(() => {
    void reload();
    return Events.On("day:changed", () => void reload());
  }, [reload]);

  return (
    <section className="rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6">
      <h2 className="mb-1 text-lg font-semibold">Activities</h2>
      <p className="mb-4 text-sm text-zinc-500">
        Pinned activities appear as one-click buttons when switching. Archived ones are hidden
        from pickers but keep their history.
      </p>

      <div className="mb-5 max-w-md">
        <ActivityInput
          activities={acts.filter((a) => !a.archived)}
          buttonLabel="Add"
          placeholder="New activity name…"
          onSubmit={(name) => void run(TrackerService.CreateActivity(name))}
        />
      </div>

      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-xs uppercase text-zinc-500">
            <th className="pb-2 font-medium">Name</th>
            <th className="pb-2 text-center font-medium">Pinned</th>
            <th className="pb-2 text-center font-medium">Default at start</th>
            <th className="pb-2 text-center font-medium">Archived</th>
          </tr>
        </thead>
        <tbody>
          {acts.map((a) => (
            <tr key={a.id} className={"border-t border-zinc-800 " + (a.archived ? "opacity-50" : "")}>
              <td className="py-2 pr-4">
                {a.name}
                {a.isBreak && <span className="ml-2 text-xs text-amber-500">break</span>}
              </td>
              <td className="py-2 text-center">
                <input
                  type="checkbox"
                  checked={a.isPreset}
                  onChange={(e) => void run(TrackerService.SetActivityPreset(a.id, e.target.checked))}
                  className="size-4 accent-indigo-500"
                />
              </td>
              <td className="py-2 text-center">
                <input
                  type="radio"
                  name="default-activity"
                  checked={a.name === state?.defaultActivity}
                  disabled={a.archived || a.isBreak}
                  onChange={() => void run(TrackerService.SetDefaultActivity(a.name))}
                  className="size-4 accent-emerald-500"
                />
              </td>
              <td className="py-2 text-center">
                <input
                  type="checkbox"
                  checked={a.archived}
                  onChange={(e) => void run(TrackerService.SetActivityArchived(a.id, e.target.checked))}
                  className="size-4 accent-zinc-500"
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

import { useId, useState } from "react";
import type { Activity } from "../../bindings/timetrack/internal/tracker/models";

/**
 * Free-text activity input with autocomplete (native datalist keeps the
 * dependency count at zero) plus a submit button.
 */
export function ActivityInput({
  activities,
  buttonLabel,
  onSubmit,
  initialValue = "",
  placeholder = "Custom activity — type a name, existing or new…",
}: {
  activities: Activity[];
  buttonLabel: string;
  onSubmit: (name: string) => void;
  initialValue?: string;
  placeholder?: string;
}) {
  const [value, setValue] = useState(initialValue);
  const listId = useId();

  const submit = () => {
    const name = value.trim();
    if (!name) return;
    onSubmit(name);
    setValue("");
  };

  return (
    <div className="flex gap-2">
      <input
        type="text"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => e.key === "Enter" && submit()}
        list={listId}
        placeholder={placeholder}
        className="min-w-0 flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm outline-none focus:border-indigo-500"
      />
      <datalist id={listId}>
        {activities.map((a) => (
          <option key={a.id} value={a.name} />
        ))}
      </datalist>
      <button
        onClick={submit}
        disabled={!value.trim()}
        className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium hover:bg-indigo-500 disabled:opacity-40"
      >
        {buttonLabel}
      </button>
    </div>
  );
}

/** One-click preset buttons. */
export function PresetButtons({
  activities,
  current,
  onPick,
}: {
  activities: Activity[];
  current?: string;
  onPick: (name: string) => void;
}) {
  const presets = activities.filter((a) => a.isPreset);
  return (
    <div className="flex flex-wrap gap-2">
      {presets.map((a) => (
        <button
          key={a.id}
          onClick={() => onPick(a.name)}
          disabled={a.name === current}
          className={
            "rounded-lg border px-3 py-1.5 text-sm " +
            (a.name === current
              ? "border-indigo-500 bg-indigo-600/20 text-indigo-300"
              : "border-zinc-700 bg-zinc-900 hover:border-zinc-500")
          }
        >
          {a.name}
        </button>
      ))}
    </div>
  );
}

import { useState } from "react";
import { AppProvider } from "./lib/app";
import { ErrorToast } from "./components/ErrorToast";
import { TodayView } from "./views/TodayView";
import { ReviewView } from "./views/ReviewView";
import { ActivitiesView } from "./views/ActivitiesView";
import { QuickView } from "./views/QuickView";

type Tab = "today" | "review" | "activities";

function Shell() {
  const [tab, setTab] = useState<Tab>("today");
  return (
    <div className="mx-auto min-h-screen max-w-3xl px-6 py-6">
      <header className="mb-6 flex items-center justify-between">
        <h1 className="text-xl font-bold tracking-tight">timetrack</h1>
        <nav className="flex gap-1 rounded-xl border border-zinc-800 bg-zinc-900/60 p-1">
          {(["today", "review", "activities"] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={
                "rounded-lg px-4 py-1.5 text-sm font-medium capitalize " +
                (tab === t ? "bg-zinc-700/70" : "text-zinc-400 hover:text-zinc-200")
              }
            >
              {t}
            </button>
          ))}
        </nav>
      </header>
      {tab === "today" && <TodayView />}
      {tab === "review" && <ReviewView />}
      {tab === "activities" && <ActivitiesView />}
      <ErrorToast />
    </div>
  );
}

function App() {
  // The tray popup window loads the same SPA at "/#/quick".
  const quick = window.location.hash.startsWith("#/quick");
  return (
    <AppProvider>
      {quick ? <QuickView /> : <Shell />}
    </AppProvider>
  );
}

export default App;

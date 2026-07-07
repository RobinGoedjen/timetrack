import { useApp } from "../lib/app";

export function ErrorToast() {
  const { error, clearError } = useApp();
  if (!error) return null;
  return (
    <div
      onClick={clearError}
      className="fixed bottom-4 left-1/2 z-50 -translate-x-1/2 cursor-pointer rounded-lg border border-rose-700 bg-rose-950 px-4 py-2 text-sm text-rose-200 shadow-lg"
      role="alert"
    >
      {error}
    </div>
  );
}

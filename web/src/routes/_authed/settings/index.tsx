/**
 * Settings index — redirects to /settings/account.
 * Uses a simple window.location approach because the app uses a hand-rolled
 * router (not TanStack Router) as established by App.tsx.
 */
export function Settings() {
  if (typeof window !== "undefined") {
    window.location.replace("/settings/account");
  }
  return null;
}

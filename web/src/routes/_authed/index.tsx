export function AuthedIndex() {
  // Redirect to default dashboard
  if (typeof window !== "undefined") window.location.replace("/dashboards/default");
  return null;
}

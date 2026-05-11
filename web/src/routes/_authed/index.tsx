export function AuthedIndex() {
  // Redirect to /home (v2 IA entry point)
  if (typeof window !== "undefined") window.location.replace("/_authed/home");
  return null;
}

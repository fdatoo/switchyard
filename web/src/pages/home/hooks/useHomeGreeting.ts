/**
 * useHomeGreeting — derives greeting string + calm/alert copy.
 *
 * Time bands (per Plan 02 decision #1):
 *   05:00–11:59 → "Good morning"
 *   12:00–17:29 → "Good afternoon"
 *   17:30–20:59 → "Good evening"
 *   21:00–04:59 → "Good night"
 */

export interface UseHomeGreetingInput {
  alertCount: number;
}

export interface UseHomeGreetingOutput {
  greeting: string;
  statusLine: string;
}

function getGreeting(hour: number, minute: number): string {
  const totalMinutes = hour * 60 + minute;
  if (totalMinutes >= 5 * 60 && totalMinutes < 12 * 60) return "Good morning";
  if (totalMinutes >= 12 * 60 && totalMinutes < 17 * 60 + 30) return "Good afternoon";
  if (totalMinutes >= 17 * 60 + 30 && totalMinutes < 21 * 60) return "Good evening";
  return "Good night";
}

function getStatusLine(alertCount: number): string {
  if (alertCount === 0) return "· everything looks calm.";
  if (alertCount === 1) return "· 1 thing needs attention.";
  return `· ${alertCount} things need attention.`;
}

export function useHomeGreeting({ alertCount }: UseHomeGreetingInput): UseHomeGreetingOutput {
  const now = new Date();
  const hour = now.getHours();
  const minute = now.getMinutes();

  return {
    greeting: getGreeting(hour, minute),
    statusLine: getStatusLine(alertCount),
  };
}

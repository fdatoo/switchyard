export type DaemonConnectionStatus = "online" | "reconnecting";

export type DaemonConnectionSnapshot = {
  status: DaemonConnectionStatus;
  sinceMs: number;
  checkedAtMs: number;
  lastError?: string;
};

type Listener = () => void;

let snapshot: DaemonConnectionSnapshot = {
  status: "online",
  sinceMs: Date.now(),
  checkedAtMs: Date.now(),
};
const listeners = new Set<Listener>();

export function getDaemonConnectionSnapshot(): DaemonConnectionSnapshot {
  return snapshot;
}

export function subscribeDaemonConnection(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function markDaemonReachable(): void {
  const now = Date.now();
  if (snapshot.status === "online") {
    snapshot = { ...snapshot, checkedAtMs: now };
    return;
  }
  snapshot = { status: "online", sinceMs: now, checkedAtMs: now };
  emit();
}

export function markDaemonUnreachable(error: unknown): void {
  const now = Date.now();
  snapshot = {
    status: "reconnecting",
    sinceMs: snapshot.status === "reconnecting" ? snapshot.sinceMs : now,
    checkedAtMs: now,
    lastError: errorMessage(error),
  };
  emit();
}

export function resetDaemonConnectionForTest(): void {
  const now = Date.now();
  snapshot = { status: "online", sinceMs: now, checkedAtMs: now };
  emit();
}

function emit(): void {
  for (const listener of listeners) {
    listener();
  }
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "daemon unreachable";
}

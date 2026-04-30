export type PendingState = { state: "idle" } | { state: "pending"; commandId: string; sinceMs: number } | { state: "failed"; commandId: string; error: string; ageMs: number };
type Listener = (p: PendingState) => void;
export class CommandTracker {
  private pending = new Map<string, { entityId: string; t0: number }>();
  private subs = new Map<string, Set<Listener>>();
  issued(id: string, entityId: string) { this.pending.set(id, { entityId, t0: Date.now() }); this.notify(entityId); }
  acked(id: string) { const e = this.pending.get(id); this.pending.delete(id); if (e) this.notify(e.entityId); }
  failed_(id: string, error: string) {
    const e = this.pending.get(id); this.pending.delete(id);
    if (!e) return;
    setTimeout(() => this.notify(e.entityId), 3000);
    this.notify(e.entityId);
    void error;
  }
  current(entityId: string): PendingState {
    const ps = [...this.pending.entries()].filter(([, e]) => e.entityId === entityId);
    if (ps.length > 0) { const [cid, e] = ps[ps.length - 1]; return { state: "pending", commandId: cid, sinceMs: Date.now() - e.t0 }; }
    return { state: "idle" };
  }
  subscribe(entityId: string, fn: Listener) {
    if (!this.subs.has(entityId)) this.subs.set(entityId, new Set());
    this.subs.get(entityId)!.add(fn);
    return () => this.subs.get(entityId)!.delete(fn);
  }
  private notify(id: string) { this.subs.get(id)?.forEach(fn => fn(this.current(id))); }
}

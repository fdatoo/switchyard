type Listener = (s: unknown) => void;
export class StateCache {
  private states = new Map<string, unknown>();
  private subs = new Map<string, Set<Listener>>();
  set(id: string, s: unknown) { this.states.set(id, s); this.subs.get(id)?.forEach(fn => fn(s)); }
  get(id: string) { return this.states.get(id); }
  subscribe(id: string, fn: Listener) {
    if (!this.subs.has(id)) this.subs.set(id, new Set());
    this.subs.get(id)!.add(fn);
    return () => this.subs.get(id)!.delete(fn);
  }
}

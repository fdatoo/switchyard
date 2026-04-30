export class UndoRedo<T> {
  private past: T[] = [];
  private future: T[] = [];
  constructor(private current: T) {}
  push(state: T) { this.past.push(this.current); this.current = state; this.future = []; }
  undo() { const s = this.past.pop(); if (s) { this.future.push(this.current); this.current = s; } return this.current; }
  redo() { const s = this.future.pop(); if (s) { this.past.push(this.current); this.current = s; } return this.current; }
  get() { return this.current; }
  canUndo() { return this.past.length > 0; }
  canRedo() { return this.future.length > 0; }
}

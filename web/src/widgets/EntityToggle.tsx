import type { WidgetProps } from "@switchyard/widget-sdk";
export function EntityToggle({ id, pending }: WidgetProps) {
  const state = pending?.state ?? "idle";
  return (
    <div className="widget entity-toggle" data-testid="widget-entity-toggle" data-widget-id={id} data-pending-state={state}>
      <span>EntityToggle</span>
      <span aria-label="command state">{state}</span>
    </div>
  );
}

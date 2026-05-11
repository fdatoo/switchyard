import { builtInWidgets } from "@/dashboard/catalog";
import { WidgetErrorBoundary } from "./WidgetErrorBoundary";
import type { WidgetProps } from "@switchyard/widget-sdk";

export function WidgetRenderer(props: WidgetProps) {
  const Comp = builtInWidgets[props.classId];
  if (!Comp) return <div className="widget-unknown">Unknown: {props.classId}</div>;
  return <WidgetErrorBoundary><Comp {...props} /></WidgetErrorBoundary>;
}

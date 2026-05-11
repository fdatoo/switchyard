import type { WidgetProps } from "@switchyard/widget-sdk";
export function Markdown({ props }: WidgetProps) {
  return <div className="widget markdown">{String(props["content"] ?? "")}</div>;
}

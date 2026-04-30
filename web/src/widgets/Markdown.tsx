import type { WidgetProps } from "@gohome/widget-sdk";
export function Markdown({ props }: WidgetProps) {
  return <div className="widget markdown">{String(props["content"] ?? "")}</div>;
}

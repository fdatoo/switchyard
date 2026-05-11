import type { ComponentType } from "react";
import type { WidgetProps } from "@switchyard/widget-sdk";
import { EntityToggle } from "@/widgets/EntityToggle";

export const builtInWidgets: Record<string, ComponentType<WidgetProps>> = {
  EntityToggle,
};

export function registerWidget(classId: string, component: ComponentType<WidgetProps>): void {
  builtInWidgets[classId] = component;
}

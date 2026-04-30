import type { ComponentType } from "react";
import type { WidgetProps } from "@gohome/widget-sdk";

export const builtInWidgets: Record<string, ComponentType<WidgetProps>> = {};

export function registerWidget(classId: string, component: ComponentType<WidgetProps>): void {
  builtInWidgets[classId] = component;
}

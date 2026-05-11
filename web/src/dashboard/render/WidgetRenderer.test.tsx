import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { registerWidget } from "@/dashboard/catalog";
import { WidgetRenderer } from "./WidgetRenderer";
import type { WidgetProps } from "@switchyard/widget-sdk";

describe("WidgetRenderer", () => {
  it("renders registered widget-sdk components", () => {
    registerWidget("TestWidget", ({ id }: WidgetProps) => <div data-testid="test-widget">{id}</div>);

    render(<WidgetRenderer id="widget-1" classId="TestWidget" props={{}} pending={{ state: "idle" }} />);

    expect(screen.getByTestId("test-widget")).toHaveTextContent("widget-1");
  });
});

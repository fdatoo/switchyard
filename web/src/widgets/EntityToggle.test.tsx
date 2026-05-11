import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EntityToggle } from "./EntityToggle";
import type { WidgetProps } from "@switchyard/widget-sdk";

describe("EntityToggle", () => {
  it("renders with stubbed WidgetProps", () => {
    const props: WidgetProps = {
      id: "widget-1",
      classId: "EntityToggle",
      props: { entityId: "light.kitchen" },
      pending: { state: "pending", commandId: "command-1", sinceMs: 12 },
    };

    render(<EntityToggle {...props} />);

    expect(screen.getByTestId("widget-entity-toggle")).toHaveAttribute("data-widget-id", "widget-1");
    expect(screen.getByLabelText("command state")).toHaveTextContent("pending");
  });
});

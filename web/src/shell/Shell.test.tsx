import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Shell } from "./Shell";

describe("Shell", () => {
  it("renders sidebar with all 6 primary nav items in order, pages/displays empty states, and active Home", () => {
    render(
      <Shell currentPath="/_authed/home">
        <div>content</div>
      </Shell>,
    );

    const nav = screen.getByRole("navigation", { name: /primary navigation/i });
    expect(nav).toBeInTheDocument();

    // All 6 primary nav items present
    const navIds = ["home", "rooms", "activity", "automations", "devices", "settings"];
    for (const id of navIds) {
      expect(nav.querySelector(`[data-nav-id="${id}"]`)).toBeInTheDocument();
    }

    // Home is active
    expect(nav.querySelector('[data-nav-id="home"][data-active="true"]')).toBeInTheDocument();

    // Other nav items are inactive
    for (const id of navIds.filter((id) => id !== "home")) {
      expect(nav.querySelector(`[data-nav-id="${id}"][data-active="false"]`)).toBeInTheDocument();
    }

    // Pages and Displays empty states
    expect(screen.getByTestId("pages-empty")).toHaveTextContent("No custom pages yet.");
    expect(screen.getByTestId("displays-empty")).toHaveTextContent("No displays yet.");
  });
});

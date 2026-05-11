import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Sidebar } from "./Sidebar";

describe("Sidebar", () => {
  it("marks Activity as active when currentPath is /_authed/activity", () => {
    render(<Sidebar currentPath="/_authed/activity" />);

    const nav = screen.getByRole("navigation", { name: /primary navigation/i });

    expect(nav.querySelector('[data-nav-id="activity"][data-active="true"]')).toBeInTheDocument();
    // Others should not be active
    for (const id of ["home", "rooms", "automations", "devices", "settings"]) {
      expect(nav.querySelector(`[data-nav-id="${id}"][data-active="false"]`)).toBeInTheDocument();
    }
  });

  it("shows Sign in link when no user is authenticated", () => {
    render(<Sidebar currentPath="/_authed/home" />);
    expect(screen.getByText("Sign in")).toBeInTheDocument();
  });

  it("shows all 6 primary nav items", () => {
    render(<Sidebar currentPath="/" />);
    const nav = screen.getByRole("navigation", { name: /primary navigation/i });
    for (const id of ["home", "rooms", "activity", "automations", "devices", "settings"]) {
      expect(nav.querySelector(`[data-nav-id="${id}"]`)).toBeInTheDocument();
    }
  });
});

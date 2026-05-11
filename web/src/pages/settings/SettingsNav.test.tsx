import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { SettingsNav } from "./SettingsNav";

describe("SettingsNav", () => {
  it("renders all eight section labels", () => {
    render(<SettingsNav badges={{}} activeSection="account" />);
    expect(screen.getByText("Account")).toBeInTheDocument();
    expect(screen.getByText("Drivers")).toBeInTheDocument();
    expect(screen.getByText("Pkl config")).toBeInTheDocument();
    expect(screen.getByText("Widget packs")).toBeInTheDocument();
    expect(screen.getByText("Displays")).toBeInTheDocument();
    expect(screen.getByText("Theme & language")).toBeInTheDocument();
    expect(screen.getByText("Diagnostics")).toBeInTheDocument();
    expect(screen.getByText("About")).toBeInTheDocument();
  });

  it("marks active section with aria-current=page", () => {
    render(<SettingsNav badges={{}} activeSection="drivers" />);
    const driversLink = screen.getByRole("link", { name: /Drivers/i });
    expect(driversLink).toHaveAttribute("aria-current", "page");
  });

  it("does not mark non-active sections as current", () => {
    render(<SettingsNav badges={{}} activeSection="drivers" />);
    const accountLink = screen.getByRole("link", { name: /Account/i });
    expect(accountLink).not.toHaveAttribute("aria-current");
  });

  it("renders badge when count is greater than zero", () => {
    render(<SettingsNav badges={{ drivers: 3 }} activeSection="account" />);
    expect(screen.getByText("3 alerts")).toBeInTheDocument();
  });

  it("uses singular form for count of 1", () => {
    render(<SettingsNav badges={{ account: 1 }} activeSection="account" />);
    expect(screen.getByText("1 alert")).toBeInTheDocument();
  });

  it("omits badge when count is 0", () => {
    render(<SettingsNav badges={{ drivers: 0 }} activeSection="account" />);
    expect(screen.queryByText(/alert/)).not.toBeInTheDocument();
  });

  it("omits badge when no badge prop provided", () => {
    render(<SettingsNav badges={{}} activeSection="account" />);
    expect(screen.queryByText(/alert/)).not.toBeInTheDocument();
  });
});

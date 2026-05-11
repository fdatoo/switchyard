import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MobileShell } from "./MobileShell";

// Stub TanStack Router's useRouterState so BottomTabBar doesn't need a full router context
vi.mock("@tanstack/react-router", () => ({
  useRouterState: () => ({ location: { pathname: "/home" } }),
  Link: ({
    children,
    role,
    "aria-selected": ariaSelected,
    "aria-label": ariaLabel,
    className,
  }: {
    children: React.ReactNode;
    role?: string;
    "aria-selected"?: boolean;
    "aria-label"?: string;
    className?: string;
    to?: string;
  }) => (
    <a role={role} aria-selected={ariaSelected} aria-label={ariaLabel} className={className}>
      {children}
    </a>
  ),
}));

describe("MobileShell", () => {
  it("renders 4 bottom tabs", () => {
    render(
      <MobileShell>
        <div>content</div>
      </MobileShell>,
    );
    expect(screen.getByRole("tab", { name: /home/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /rooms/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /activity/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /more/i })).toBeInTheDocument();
  });

  it("renders children above tab bar", () => {
    render(
      <MobileShell>
        <p>page-content</p>
      </MobileShell>,
    );
    expect(screen.getByText("page-content")).toBeInTheDocument();
  });
});

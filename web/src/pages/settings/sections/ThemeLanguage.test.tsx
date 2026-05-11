import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ThemeLanguage } from "./ThemeLanguage";

// Mock useLanguage
const mockSetMode = vi.fn();
const mockSetLanguage = vi.fn();

vi.mock("@/theme/language-provider", () => ({
  useLanguage: vi.fn(() => ({
    language: "friendly",
    mode: "system",
    resolvedTheme: "friendly-light",
    setLanguage: mockSetLanguage,
    setMode: mockSetMode,
  })),
}));

describe("ThemeLanguage section", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders mode buttons: Light, Dark, System", () => {
    render(<ThemeLanguage />);
    expect(screen.getByRole("button", { name: "Light" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Dark" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "System" })).toBeInTheDocument();
  });

  it("clicking Light calls setMode('light')", () => {
    render(<ThemeLanguage />);
    fireEvent.click(screen.getByRole("button", { name: "Light" }));
    expect(mockSetMode).toHaveBeenCalledWith("light");
  });

  it("renders language buttons: Friendly, Ambient, Developer", () => {
    render(<ThemeLanguage />);
    expect(screen.getByRole("button", { name: "Friendly" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ambient" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Developer" })).toBeInTheDocument();
  });

  it("clicking Developer calls setLanguage('developer')", () => {
    render(<ThemeLanguage />);
    fireEvent.click(screen.getByRole("button", { name: "Developer" }));
    expect(mockSetLanguage).toHaveBeenCalledWith("developer");
  });

  it("System mode button has aria-pressed=true when mode=system", () => {
    render(<ThemeLanguage />);
    const systemBtn = screen.getByRole("button", { name: "System" });
    expect(systemBtn).toHaveAttribute("aria-pressed", "true");
  });

  it("Friendly language button has aria-pressed=true when language=friendly", () => {
    render(<ThemeLanguage />);
    const friendlyBtn = screen.getByRole("button", { name: "Friendly" });
    expect(friendlyBtn).toHaveAttribute("aria-pressed", "true");
  });
});

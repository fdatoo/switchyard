import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SearchSheet } from "./SearchSheet";

// Minimal stub for the catalog hook used by SearchSheet
vi.mock("@/hooks/useCommandCatalog", () => ({
  useCommandCatalog: () => ({ verbs: [] }),
}));

describe("SearchSheet", () => {
  it("renders category headers when results exist", async () => {
    const user = userEvent.setup();
    render(<SearchSheet open onOpenChange={() => {}} />);
    const input = screen.getByRole("searchbox");
    await user.type(input, "living");
    // We get at least one section heading even with empty catalog
    expect(screen.getByText(/rooms/i)).toBeInTheDocument();
  });

  it("shows empty state when no query", () => {
    render(<SearchSheet open onOpenChange={() => {}} />);
    expect(screen.getByPlaceholderText(/search/i)).toBeInTheDocument();
  });
});

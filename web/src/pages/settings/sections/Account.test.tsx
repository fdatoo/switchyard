import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { Account } from "./Account";

// Mock the auth-client module
vi.mock("@/data/auth-client", () => ({
  usePasskeys: vi.fn(() => ({
    passkeys: [],
    loading: false,
    revokePasskey: vi.fn(),
  })),
  useSessions: vi.fn(() => ({
    sessions: [],
    loading: false,
    revokeSession: vi.fn(),
  })),
  useTokens: vi.fn(() => ({
    tokens: [],
    loading: false,
    revokeToken: vi.fn(),
  })),
}));

import * as authClient from "@/data/auth-client";

describe("Account section", () => {
  beforeEach(() => {
    vi.mocked(authClient.usePasskeys).mockReturnValue({
      passkeys: [],
      loading: false,
      revokePasskey: vi.fn(),
    });
    vi.mocked(authClient.useSessions).mockReturnValue({
      sessions: [],
      loading: false,
      revokeSession: vi.fn(),
    });
    vi.mocked(authClient.useTokens).mockReturnValue({
      tokens: [],
      loading: false,
      revokeToken: vi.fn(),
    });
  });

  it("renders the Passkeys heading", () => {
    render(<Account />);
    expect(screen.getByText("Passkeys")).toBeInTheDocument();
  });

  it("renders the Active sessions heading", () => {
    render(<Account />);
    expect(screen.getByText("Active sessions")).toBeInTheDocument();
  });

  it("renders the Issued tokens heading", () => {
    render(<Account />);
    expect(screen.getByText("Issued tokens")).toBeInTheDocument();
  });

  it("renders passkey row data when passkeys are present", () => {
    vi.mocked(authClient.usePasskeys).mockReturnValue({
      passkeys: [
        { id: "pk1", name: "MacBook Pro", createdAt: "2026-01-01" },
        { id: "pk2", name: "iPhone 16", createdAt: "2026-02-15" },
      ],
      loading: false,
      revokePasskey: vi.fn(),
    });
    render(<Account />);
    expect(screen.getByText("MacBook Pro")).toBeInTheDocument();
    expect(screen.getByText("iPhone 16")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Revoke" })).toHaveLength(2);
  });

  it("shows empty state when no passkeys", () => {
    render(<Account />);
    expect(screen.getByText("No passkeys registered.")).toBeInTheDocument();
  });
});

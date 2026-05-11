import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { WidgetPacks } from "./WidgetPacks";

// Mock the widget-pack-client module
vi.mock("@/data/widget-pack-client", () => ({
  useInstalledPacks: vi.fn(),
  widgetPackClient: {
    listInstalledPacks: vi.fn().mockResolvedValue([]),
    installPack: vi.fn().mockResolvedValue({}),
  },
}));

import * as widgetPackClientModule from "@/data/widget-pack-client";

const mockInstallPack = vi.fn();

describe("WidgetPacks section", () => {
  beforeEach(() => {
    vi.mocked(widgetPackClientModule.useInstalledPacks).mockReturnValue({
      packs: [
        {
          name: "sy-core",
          version: "1.0.0",
          sha256: "abc123",
          signature: "verified",
          signerIdentity: "switchyard@example.com",
          ociRef: "ghcr.io/switchyard/core:1.0.0",
        },
        {
          name: "sy-dashboard",
          version: "2.1.0",
          sha256: "def456",
          signature: "unverified",
          signerIdentity: "",
          ociRef: "ghcr.io/user/dashboard:2.1.0",
        },
      ],
      loading: false,
      error: null,
      installPack: mockInstallPack,
      refresh: vi.fn(),
    });
  });

  it("renders OCI ref for each pack", () => {
    render(<WidgetPacks />);
    expect(screen.getByText("ghcr.io/switchyard/core:1.0.0")).toBeInTheDocument();
    expect(screen.getByText("ghcr.io/user/dashboard:2.1.0")).toBeInTheDocument();
  });

  it("renders verified chip for verified pack", () => {
    render(<WidgetPacks />);
    expect(screen.getByText("verified")).toBeInTheDocument();
  });

  it("opens the install dialog when + Install is clicked", () => {
    render(<WidgetPacks />);
    fireEvent.click(screen.getByRole("button", { name: "+ Install" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("shows the OCI ref input inside the dialog", () => {
    render(<WidgetPacks />);
    fireEvent.click(screen.getByRole("button", { name: "+ Install" }));
    const input = screen.getByPlaceholderText("ghcr.io/owner/pack:version");
    expect(input).toBeInTheDocument();
  });
});

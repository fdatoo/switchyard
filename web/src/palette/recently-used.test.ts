import { describe, it, expect, beforeEach } from "vitest";
import {
  getPaletteCliPreview,
  setPaletteCliPreview,
  recordRecentlyUsed,
} from "./recently-used";

// ─── CLI preview pref ────────────────────────────────────────────────────────

describe("getPaletteCliPreview", () => {
  beforeEach(() => {
    localStorage.removeItem("sy.palette.cliPreview");
  });

  it("returns false when key is absent", () => {
    expect(getPaletteCliPreview()).toBe(false);
  });

  it('returns true when key is "on"', () => {
    localStorage.setItem("sy.palette.cliPreview", "on");
    expect(getPaletteCliPreview()).toBe(true);
  });

  it('returns false when key is "off"', () => {
    localStorage.setItem("sy.palette.cliPreview", "off");
    expect(getPaletteCliPreview()).toBe(false);
  });

  it('setPaletteCliPreview(true) writes "on"', () => {
    setPaletteCliPreview(true);
    expect(localStorage.getItem("sy.palette.cliPreview")).toBe("on");
  });

  it('setPaletteCliPreview(false) writes "off"', () => {
    setPaletteCliPreview(true);
    setPaletteCliPreview(false);
    expect(localStorage.getItem("sy.palette.cliPreview")).toBe("off");
  });
});

// ─── Recently-used records ───────────────────────────────────────────────────

describe("recordRecentlyUsed", () => {
  beforeEach(() => {
    localStorage.removeItem("sy.palette.recentlyUsed");
  });

  it("records a command and reads it back", () => {
    recordRecentlyUsed("events tail", { source: "z2m" });
    const raw = JSON.parse(
      localStorage.getItem("sy.palette.recentlyUsed") ?? "[]",
    ) as Array<{ verbName: string; args: Record<string, string>; ranAt: string }>;
    expect(raw).toHaveLength(1);
    expect(raw[0].verbName).toBe("events tail");
    expect(raw[0].args).toEqual({ source: "z2m" });
  });

  it("deduplicates by exact verb+args (most recent wins)", () => {
    recordRecentlyUsed("events tail", { source: "z2m" });
    recordRecentlyUsed("entity get", { id: "abc" });
    recordRecentlyUsed("events tail", { source: "z2m" }); // duplicate
    const raw = JSON.parse(
      localStorage.getItem("sy.palette.recentlyUsed") ?? "[]",
    ) as Array<{ verbName: string }>;
    // Should have 2 entries (the duplicate is removed and re-added at the front)
    expect(raw).toHaveLength(2);
    expect(raw[0].verbName).toBe("events tail"); // most recent
  });
});

import { test, expect } from "vitest";
import { findStarlarkRegions } from "./embedded";

const PKL_SOURCE = `module foo

brightness = starlark("""
def compute(sun, now):
    return int(sun.altitude * 100)
compute(sun, now)
""")

name = "hello"`.trim();

test("detects starlark region line range", () => {
  const regions = findStarlarkRegions(PKL_SOURCE);
  expect(regions).toHaveLength(1);
  // The region starts after the opening triple-quote and ends before the closing one.
  expect(regions[0].startLine).toBeGreaterThan(0);
  expect(regions[0].endLine).toBeGreaterThan(regions[0].startLine);
});

test("returns empty for source with no starlark call", () => {
  expect(findStarlarkRegions("name = 42")).toHaveLength(0);
});

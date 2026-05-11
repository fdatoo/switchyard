import { renderHook, act } from "@testing-library/react";
import { expect, test } from "vitest";
import { useBreakpoint } from "./breakpoint";

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", {
    writable: true,
    configurable: true,
    value: width,
  });
  window.dispatchEvent(new Event("resize"));
}

test("isMobile is true below 768px", () => {
  setViewport(375);
  const { result } = renderHook(() => useBreakpoint());
  expect(result.current.isMobile).toBe(true);
});

test("isMobile is false at 768px and above", () => {
  setViewport(768);
  const { result } = renderHook(() => useBreakpoint());
  expect(result.current.isMobile).toBe(false);
});

test("isMobile updates when viewport changes", async () => {
  setViewport(1024);
  const { result } = renderHook(() => useBreakpoint());
  expect(result.current.isMobile).toBe(false);

  act(() => setViewport(375));
  expect(result.current.isMobile).toBe(true);
});

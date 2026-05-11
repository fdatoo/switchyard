/**
 * usePullToRefresh — touchstart/touchmove/touchend hook.
 * When the user pulls down by ≥ THRESHOLD pixels from the top of the scroll
 * container, onRefresh() is called.
 *
 * Note: pull-to-refresh is suppressed when the container is not scrolled to
 * the top (scrollTop > 0).
 */
import { useEffect } from "react";
import type { RefObject } from "react";

const THRESHOLD = 64; // px of overscroll needed to trigger refresh

export function usePullToRefresh(
  scrollRef: RefObject<HTMLElement | null>,
  onRefresh: () => void,
): void {
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    let startY = 0;
    let pulling = false;

    const onTouchStart = (e: TouchEvent) => {
      if (el.scrollTop === 0) {
        startY = e.touches[0].clientY;
        pulling = true;
      }
    };

    const onTouchMove = (e: TouchEvent) => {
      if (!pulling) return;
      const dy = e.touches[0].clientY - startY;
      if (dy > 0) {
        // Optionally prevent default to avoid native browser pull-to-refresh
        e.preventDefault();
      }
    };

    const onTouchEnd = (e: TouchEvent) => {
      if (!pulling) return;
      pulling = false;
      const dy = (e.changedTouches[0]?.clientY ?? startY) - startY;
      if (dy >= THRESHOLD) {
        onRefresh();
      }
      startY = 0;
    };

    el.addEventListener("touchstart", onTouchStart, { passive: true });
    el.addEventListener("touchmove", onTouchMove, { passive: false });
    el.addEventListener("touchend", onTouchEnd, { passive: true });

    return () => {
      el.removeEventListener("touchstart", onTouchStart);
      el.removeEventListener("touchmove", onTouchMove);
      el.removeEventListener("touchend", onTouchEnd);
    };
  }, [scrollRef, onRefresh]);
}

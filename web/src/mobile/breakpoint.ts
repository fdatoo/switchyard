import { useEffect, useState } from "react";

const MOBILE_MAX = 767; // px — viewport width ≤ this → isMobile

function isMobileViewport(): boolean {
  return window.innerWidth <= MOBILE_MAX;
}

export function useBreakpoint(): { isMobile: boolean } {
  const [isMobile, setIsMobile] = useState<boolean>(isMobileViewport);

  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${MOBILE_MAX}px)`);
    const mqHandler = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    const resizeHandler = () => setIsMobile(isMobileViewport());
    mq.addEventListener("change", mqHandler);
    window.addEventListener("resize", resizeHandler);
    return () => {
      mq.removeEventListener("change", mqHandler);
      window.removeEventListener("resize", resizeHandler);
    };
  }, []);

  return { isMobile };
}

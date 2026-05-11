import { Outlet } from "@tanstack/react-router";
import { Shell } from "@/shell/Shell";
import { MobileShell } from "@/mobile/MobileShell";
import { useBreakpoint } from "@/mobile/breakpoint";

export function AuthedLayout() {
  const { isMobile } = useBreakpoint();
  return isMobile ? (
    <MobileShell>
      <Outlet />
    </MobileShell>
  ) : (
    <Shell>
      <Outlet />
    </Shell>
  );
}

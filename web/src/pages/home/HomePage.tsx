import { useHomeStatus } from "./hooks/useHomeStatus";
import { GreetingSection } from "./GreetingSection";
import { StatusRowSection } from "./StatusRowSection";
import { RightNowStripSection } from "./RightNowStripSection";
import { RoomsGridSection } from "./RoomsGridSection";
import { RecentActivitySection } from "./RecentActivitySection";
import { ActiveAutomationsSection } from "./ActiveAutomationsSection";

/**
 * HomePage — composes all six curated Home sections in order (Plan 02, decision #8):
 *   1. Greeting
 *   2. Status row
 *   3. Right Now strip
 *   4. Rooms grid
 *   5. Recent activity
 *   6. Active automations
 *
 * alertCount is derived internally from the status hook so the route stays thin.
 */
export function HomePage() {
  const statusItems = useHomeStatus();
  const alertCount = statusItems.filter((item) => item.severity === "bad").length;

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--sy-space-6)",
        padding: "var(--sy-space-5) var(--sy-space-6)",
        maxWidth: "1200px",
      }}
    >
      <GreetingSection alertCount={alertCount} />
      <StatusRowSection />
      <RightNowStripSection />
      <RoomsGridSection />
      <RecentActivitySection />
      <ActiveAutomationsSection />
    </div>
  );
}

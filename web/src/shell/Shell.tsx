import type { ReactNode } from "react";
import { ReconnectingBanner } from "./ReconnectingBanner";
import { Sidebar } from "./Sidebar";
import { TopBar } from "./TopBar";

interface ShellProps {
  children: ReactNode;
  currentPath?: string;
}

/**
 * App shell — renders on every authed route except /login and the Ambient renderer.
 *
 * Layout: 200px Sidebar | flex content
 *   - TopBar sits at the top of the content area
 *   - Children render in the scrollable content area below the TopBar
 */
export function Shell({ children, currentPath }: ShellProps) {
  const path = currentPath ?? (typeof window !== "undefined" ? window.location.pathname : "/");

  return (
    <div
      data-testid="shell"
      style={{
        display: "flex",
        height: "100vh",
        background: "var(--sy-color-bg)",
      }}
    >
      <Sidebar currentPath={path} />
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          flex: 1,
          minWidth: 0,
          height: "100vh",
          overflow: "hidden",
        }}
      >
        <ReconnectingBanner />
        <TopBar currentPath={path} />
        <main
          style={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            overflowY: "auto",
          }}
        >
          {children}
        </main>
      </div>
    </div>
  );
}

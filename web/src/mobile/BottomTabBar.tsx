import { Link, useRouterState } from "@tanstack/react-router";
import styles from "./BottomTabBar.module.css";

const TABS = [
  { id: "home", label: "Home", href: "/home" },
  { id: "rooms", label: "Rooms", href: "/rooms" },
  { id: "activity", label: "Activity", href: "/activity" },
  { id: "more", label: "More", href: "/settings" },
] as const;

export function BottomTabBar() {
  const { location } = useRouterState();
  return (
    <nav className={styles.bar} aria-label="Main tabs">
      {TABS.map((tab) => {
        const active = location.pathname.startsWith(tab.href);
        return (
          <Link
            key={tab.id}
            to={tab.href}
            role="tab"
            aria-selected={active}
            aria-label={tab.label}
            className={`${styles.tab} ${active ? styles.active : ""}`}
          >
            <span className={styles.label}>{tab.label}</span>
          </Link>
        );
      })}
    </nav>
  );
}

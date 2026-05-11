import type { ReactNode } from "react";
import { BottomTabBar } from "./BottomTabBar";
import styles from "./MobileShell.module.css";

interface Props {
  children: ReactNode;
}

export function MobileShell({ children }: Props) {
  return (
    <div className={styles.root}>
      <main className={styles.content}>{children}</main>
      <BottomTabBar />
    </div>
  );
}

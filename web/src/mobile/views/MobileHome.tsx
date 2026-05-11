import { useState } from "react";
import { useHomeSummary } from "@/hooks/useHomeSummary";
import { RoomSheet } from "@/mobile/RoomSheet";
import styles from "./MobileHome.module.css";

export function MobileHome() {
  const { stats, rooms } = useHomeSummary();
  const [activeRoom, setActiveRoom] = useState<string | null>(null);
  const selectedRoom = rooms.find((r) => r.slug === activeRoom) ?? null;

  return (
    <div className={styles.page}>
      <section className={styles.statsGrid}>
        {stats.slice(0, 4).map((s) => (
          <div key={s.id} className={styles.statTile}>
            <span className={styles.statValue}>{s.value}</span>
            <span className={styles.statLabel}>{s.label}</span>
          </div>
        ))}
      </section>
      <section className={styles.roomsGrid}>
        {rooms.map((r) => (
          <button key={r.slug} className={styles.roomCard} onClick={() => setActiveRoom(r.slug)}>
            {r.name}
          </button>
        ))}
      </section>
      {selectedRoom && (
        <RoomSheet
          open={!!activeRoom}
          room={{ ...selectedRoom, brightness: 80, scenes: [], entities: [] }}
          onOpenChange={(open) => {
            if (!open) setActiveRoom(null);
          }}
        />
      )}
    </div>
  );
}

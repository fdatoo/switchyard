import { Sheet, SheetContent } from "./Sheet";
import styles from "./RoomSheet.module.css";

interface Entity {
  id: string;
  name: string;
  on: boolean;
}

interface Room {
  slug: string;
  name: string;
  brightness: number;
  scenes: string[];
  entities: Entity[];
}

interface Props {
  open: boolean;
  room: Room;
  onOpenChange: (open: boolean) => void;
}

export function RoomSheet({ open, room, onOpenChange }: Props) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <h2 className={styles.title}>{room.name}</h2>
        <input
          type="range"
          min={0}
          max={100}
          defaultValue={room.brightness}
          className={styles.slider}
          aria-label="Brightness"
        />
        <div className={styles.scenes}>
          {room.scenes.slice(0, 4).map((s) => (
            <button key={s} className={styles.scene}>
              {s}
            </button>
          ))}
        </div>
        <ul className={styles.entities}>
          {room.entities.map((e) => (
            <li key={e.id} className={styles.entity}>
              <span>{e.name}</span>
              <input type="checkbox" defaultChecked={e.on} aria-label={e.name} />
            </li>
          ))}
        </ul>
      </SheetContent>
    </Sheet>
  );
}

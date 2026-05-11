import { Sheet, SheetContent } from "./Sheet";
import styles from "./StorySheet.module.css";

interface Story {
  id: string;
  title: string;
  whyInteresting: string[];
  events: { id: string; summary: string }[];
  actions: string[];
}

interface Props {
  open: boolean;
  story: Story;
  onOpenChange: (open: boolean) => void;
}

export function StorySheet({ open, story, onOpenChange }: Props) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <h2 className={styles.title}>{story.title}</h2>
        <section className={styles.section}>
          <h3 className={styles.sectionHeading}>Why interesting</h3>
          {story.whyInteresting.map((reason, i) => (
            <div key={i} className={styles.card}>
              {reason}
            </div>
          ))}
        </section>
        <section className={styles.section}>
          <h3 className={styles.sectionHeading}>Events</h3>
          <ul className={styles.events}>
            {story.events.map((e) => (
              <li key={e.id} className={styles.event}>
                {e.summary}
              </li>
            ))}
          </ul>
        </section>
        <div className={styles.actions}>
          {story.actions.map((a) => (
            <button key={a} className={styles.action}>
              {a}
            </button>
          ))}
        </div>
      </SheetContent>
    </Sheet>
  );
}

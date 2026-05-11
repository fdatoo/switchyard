import { useState } from "react";
import { Sheet, SheetContent } from "./Sheet";
import styles from "./SearchSheet.module.css";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// Each category shown in the search sheet. Results within categories
// come from the router tree (rooms, pages) and the command catalog verbs
// that have a navigation target. No verb parsing is done here.
const CATEGORIES = ["Rooms", "Entities", "Automations", "Activity"] as const;

export function SearchSheet({ open, onOpenChange }: Props) {
  const [query, setQuery] = useState("");

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <input
          role="searchbox"
          className={styles.input}
          placeholder="Search rooms, entities, automations…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          autoFocus
        />
        {CATEGORIES.map((cat) => (
          <section key={cat} className={styles.section}>
            <h3 className={styles.heading}>{cat}</h3>
            {/* Results wired in subsequent tasks; placeholder empty state */}
            {query.length === 0 && (
              <p className={styles.empty}>Type to search {cat.toLowerCase()}</p>
            )}
          </section>
        ))}
      </SheetContent>
    </Sheet>
  );
}

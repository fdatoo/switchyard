import type { InterestingnessCategory } from "../../gen/activity/v1/activity_pb";
import styles from "./InterestingnessBadge.module.css";

export interface InterestingnessBadgeProps {
  category: InterestingnessCategory;
  name?: string;
}

/**
 * InterestingnessBadge renders a colored chip for an interestingness category.
 *
 * Colors are driven entirely by CSS attribute selectors on `data-interesting-category`
 * so no inline style colors are used. This satisfies the `no-raw-tokens` lint rule.
 */
export function InterestingnessBadge({ category, name }: InterestingnessBadgeProps) {
  return (
    <span
      className={styles.badge}
      data-interesting-category={category}
      role="status"
      aria-label={`Interesting: ${name ?? category}`}
    >
      {name ?? category}
    </span>
  );
}

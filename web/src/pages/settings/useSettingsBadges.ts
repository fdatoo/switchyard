import type { SettingsBadges } from "./SettingsNav";

/**
 * useSettingsBadges — returns per-section alert badge counts for the SettingsNav rail.
 *
 * When Plan 03 (ActivityService) ships, replace this stub with the real stream.
 *
 * TODO(plan-3): wire to ActivityService.Stories interestingness counts.
 *
 * Once Plan 03 is available:
 *   1. Import activityClient from "@/data/activity-client"
 *   2. Replace the stub body below with:
 *
 *   const [badges, setBadges] = useState<SettingsBadges>({});
 *   useEffect(() => {
 *     const counts: Partial<Record<keyof SettingsBadges, number>> = {};
 *     const stream = activityClient.stories({ filter: { interestingOnly: true } });
 *     (async () => {
 *       try {
 *         for await (const story of stream) {
 *           // category → section mapping:
 *           //   "failure" | "performance" | "anomaly"  → "drivers"
 *           //   "security" | "configuration"            → "account"
 *           //   "novelty"                               → "widget-packs"
 *           const section = categoryToSection(story.category);
 *           if (section) counts[section] = (counts[section] ?? 0) + 1;
 *         }
 *       } catch {
 *         // stream closed or error — leave current counts
 *       }
 *       setBadges(counts as SettingsBadges);
 *     })();
 *   }, []);
 *   return badges;
 *
 * The categoryToSection helper:
 *   function categoryToSection(category: string): keyof SettingsBadges | null {
 *     if (["failure", "performance", "anomaly"].includes(category)) return "drivers";
 *     if (["security", "configuration"].includes(category)) return "account";
 *     if (category === "novelty") return "widget-packs";
 *     return null;
 *   }
 */
export function useSettingsBadges(): SettingsBadges {
  // Guard: Plan 03 ActivityService not yet available — return empty badges.
  return {};
}

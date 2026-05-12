/**
 * Derive a breadcrumb trail from a route path + the primary nav table.
 *
 * Rules:
 *   1. Empty path or "/" → single "Home" crumb.
 *   2. The first segment matches the primary nav by `path` to get the
 *      canonical top-level label (so `/settings/drivers` yields
 *      "Settings" rather than the segment-cased "Settings").
 *   3. Deeper segments are title-cased and humanized (dashes / underscores
 *      become spaces). The deepest crumb has no `to`; earlier ones are
 *      links.
 *
 * Pure function — no Vue dependency, easy to unit-test, and the only
 * piece of crumb logic in the app. AppLayout just `computed(() =>
 * crumbsFor(route.path, PRIMARY))`.
 */

import type { BreadcrumbItem } from "@/lib/components/breadcrumb/SyBreadcrumb.vue";
import type { SidebarNavItem } from "@/lib/components/sidebar/SySidebar.vue";

const HOME: BreadcrumbItem = { label: "Home" };

function humanize(segment: string): string {
  if (!segment) return "";
  const clean = segment.replace(/[-_]/g, " ");
  return clean.charAt(0).toUpperCase() + clean.slice(1);
}

export function crumbsFor(path: string, primary: SidebarNavItem[]): BreadcrumbItem[] {
  const segs = path.split("/").filter(Boolean);
  if (segs.length === 0) return [HOME];

  return segs.map((seg, i) => {
    const isLast = i === segs.length - 1;
    const navMatch = i === 0
      ? primary.find((p) => p.path === `/${seg}`)
      : undefined;
    const label = navMatch ? navMatch.label : humanize(seg);
    if (isLast) return { label };
    return { label, to: "/" + segs.slice(0, i + 1).join("/") };
  });
}

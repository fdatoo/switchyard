import { DashboardView } from "@/dashboard/render/DashboardView";

type Props = { slug?: string };
export function DashboardSlug({ slug = "default" }: Props) {
  return <DashboardView slug={slug} />;
}

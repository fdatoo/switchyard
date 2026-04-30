import { DashboardSkeleton } from "./DashboardSkeleton";

type Props = { slug: string };
export function DashboardView({ slug }: Props) {
  // Stub — real implementation queries the server
  return (
    <div className="dashboard-view">
      <DashboardSkeleton />
      <p>Dashboard: {slug}</p>
    </div>
  );
}

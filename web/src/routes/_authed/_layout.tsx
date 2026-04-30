import { Outlet } from "@tanstack/react-router";
import { Shell } from "@/shell/Shell";

export function AuthedLayout() {
  return <Shell><Outlet /></Shell>;
}

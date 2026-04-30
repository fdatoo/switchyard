import type { ReactNode } from "react";

type Props = { activeNavId?: string; title?: string; children: ReactNode };
export function Shell({ children }: Props) {
  return <div className="shell">{children}</div>;
}

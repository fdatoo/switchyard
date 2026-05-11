import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react";
import { createMultiplexer, type Mux } from "./multiplexer";
import { transport } from "@/data/client";

const MuxCtx = createContext<Mux | null>(null);

export function Multiplexer({ entityIds, children }: { entityIds: string[]; children: ReactNode }) {
  const mux = useMemo(() => createMultiplexer({ transport }), []);
  useEffect(() => {
    mux.subscribe(entityIds);
    return () => mux.shutdown();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mux, entityIds.join(",")]);
  return <MuxCtx.Provider value={mux}>{children}</MuxCtx.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useMultiplexer(): Mux {
  const v = useContext(MuxCtx);
  if (!v) throw new Error("useMultiplexer must be inside <Multiplexer>");
  return v;
}

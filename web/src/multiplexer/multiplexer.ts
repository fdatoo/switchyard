import { StateCache } from "./state-cache";
import { CommandTracker } from "./command-tracker";

export type Mux = {
  cache: StateCache;
  tracker: CommandTracker;
  subscribe: (ids: string[]) => void;
  shutdown: () => void;
  callCapability: (entityId: string, capability: string, args: Record<string, string>) => Promise<void>;
};

export function createMultiplexer(_opts: { transport: unknown }): Mux {
  const cache = new StateCache();
  const tracker = new CommandTracker();
  return {
    cache,
    tracker,
    subscribe: (_ids: string[]) => {},
    shutdown: () => {},
    callCapability: async (_entityId, _capability, _args) => {},
  };
}

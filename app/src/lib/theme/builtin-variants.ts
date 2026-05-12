/**
 * Registers the built-in language variants for every variant-bearing slot.
 *
 * Called once from `main.ts` before the app mounts. The `installed` guard
 * makes repeated calls safe (e.g., Vite HMR re-evaluating this module).
 *
 * To add a new variant-bearing primitive, add registrations here and the slot
 * name to `SlotName` in `./types.ts`.
 */

import { registerVariant } from "./variant-registry";
import SyButtonFriendly from "@/lib/components/button/SyButtonFriendly.vue";
import SyButtonDeveloper from "@/lib/components/button/SyButtonDeveloper.vue";
import SyButtonAmbient from "@/lib/components/button/SyButtonAmbient.vue";
import SyInputFriendly from "@/lib/components/input/SyInputFriendly.vue";
import SyInputDeveloper from "@/lib/components/input/SyInputDeveloper.vue";
import SyInputAmbient from "@/lib/components/input/SyInputAmbient.vue";

let installed = false;

export function installBuiltinVariants(): void {
  if (installed) return;
  installed = true;

  registerVariant("Button", "friendly", SyButtonFriendly);
  registerVariant("Button", "developer", SyButtonDeveloper);
  registerVariant("Button", "ambient", SyButtonAmbient);

  registerVariant("Input", "friendly", SyInputFriendly);
  registerVariant("Input", "developer", SyInputDeveloper);
  registerVariant("Input", "ambient", SyInputAmbient);
}

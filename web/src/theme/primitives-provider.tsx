import {
  createContext,
  useContext,
  useMemo,
  type ComponentType,
  type ReactNode,
} from "react";
import { useLanguage } from "./language-provider";
import type { Language } from "./languages/index";
import { Button } from "./primitives/button";
import { Chip } from "./primitives/chip";
import { Pill } from "./primitives/pill";
import { Surface } from "./primitives/surface";
import { DeveloperButton } from "./primitives/developer/button";
import { DeveloperChip } from "./primitives/developer/chip";
import { DeveloperPill } from "./primitives/developer/pill";
import { DeveloperSurface } from "./primitives/developer/surface";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type PrimitiveName = "Button" | "Chip" | "Pill" | "Surface";
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type PrimitiveRegistry = Partial<Record<Language, Partial<Record<PrimitiveName, ComponentType<any>>>>>;

// ---------------------------------------------------------------------------
// Fallback component — renders children inside a plain <div> with no styling
// ---------------------------------------------------------------------------

function FallbackPrimitive({ children, ...rest }: { children?: ReactNode; [key: string]: unknown }) {
  return <div {...rest}>{children}</div>;
}

// ---------------------------------------------------------------------------
// Built-in registry for friendly (the only language with primitives in Plan 1)
// ---------------------------------------------------------------------------

const BUILT_IN_REGISTRY: PrimitiveRegistry = {
  friendly: {
    Button,
    Chip,
    Pill,
    Surface,
  },
  // ambient: falls back to friendly primitives via FallbackPrimitive for now
  ambient: {},
  developer: {
    Button: DeveloperButton,
    Chip: DeveloperChip,
    Pill: DeveloperPill,
    Surface: DeveloperSurface,
  },
};

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

interface PrimitivesContextValue {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  getPrimitive: (name: PrimitiveName) => ComponentType<any>;
}

const PrimitivesContext = createContext<PrimitivesContextValue | null>(null);

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

interface LanguagePrimitivesProps {
  children: ReactNode;
  /**
   * Optional extra registry — allows later plans to register their own primitive
   * variants without changing this provider's shape.
   */
  registry?: PrimitiveRegistry;
}

export function LanguagePrimitives({ children, registry }: LanguagePrimitivesProps) {
  const { language } = useLanguage();

  const getPrimitive = useMemo<PrimitivesContextValue["getPrimitive"]>(() => {
    return (name: PrimitiveName) => {
      // Extra registry wins over built-in (allows override by later plans)
      const extraVariant = registry?.[language]?.[name];
      if (extraVariant) return extraVariant;
      const builtIn = BUILT_IN_REGISTRY[language]?.[name];
      if (builtIn) return builtIn;
      return FallbackPrimitive;
    };
  }, [language, registry]);

  const value = useMemo<PrimitivesContextValue>(() => ({ getPrimitive }), [getPrimitive]);

  return <PrimitivesContext.Provider value={value}>{children}</PrimitivesContext.Provider>;
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

// eslint-disable-next-line react-refresh/only-export-components
export function usePrimitive(name: PrimitiveName): ComponentType<any> { // eslint-disable-line @typescript-eslint/no-explicit-any
  const ctx = useContext(PrimitivesContext);
  if (!ctx) throw new Error("usePrimitive must be used inside <LanguagePrimitives>");
  return ctx.getPrimitive(name);
}

/**
 * Sheet — hand-rolled bottom-sheet primitive.
 *
 * Uses React context to share open state between Sheet, SheetTrigger, and
 * SheetContent without requiring Radix UI.
 *
 * Controlled usage:
 *   <Sheet open={open} onOpenChange={setOpen}>…</Sheet>
 *
 * Uncontrolled usage:
 *   <Sheet>
 *     <SheetTrigger>Open</SheetTrigger>
 *     <SheetContent>…</SheetContent>
 *   </Sheet>
 */
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import type { ReactNode } from "react";
import styles from "./Sheet.module.css";

interface SheetContextValue {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const SheetContext = createContext<SheetContextValue | null>(null);

function useSheetContext(): SheetContextValue {
  const ctx = useContext(SheetContext);
  if (!ctx) throw new Error("Sheet compound components must be used inside <Sheet>");
  return ctx;
}

interface SheetProps {
  children: ReactNode;
  /** Controlled open state. When provided, the Sheet is fully controlled. */
  open?: boolean;
  /** Called when the Sheet requests an open-state change. */
  onOpenChange?: (open: boolean) => void;
}

/** Root component — provides open state context. */
export function Sheet({ children, open: controlledOpen, onOpenChange }: SheetProps) {
  const [internalOpen, setInternalOpen] = useState(false);

  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : internalOpen;
  const setOpen = isControlled
    ? (onOpenChange ?? (() => undefined))
    : setInternalOpen;

  return (
    <SheetContext.Provider value={{ open, onOpenChange: setOpen }}>
      {children}
    </SheetContext.Provider>
  );
}

interface SheetTriggerProps {
  children: ReactNode;
  className?: string;
}

/** Button that opens the Sheet when clicked. */
export function SheetTrigger({ children, className }: SheetTriggerProps) {
  const { onOpenChange } = useSheetContext();
  return (
    <button type="button" className={className} onClick={() => onOpenChange(true)}>
      {children}
    </button>
  );
}

interface SheetContentProps {
  children: ReactNode;
}

/** Portaled bottom-sheet panel — rendered when the Sheet is open. */
export function SheetContent({ children }: SheetContentProps) {
  const { open, onOpenChange } = useSheetContext();
  const overlayRef = useRef<HTMLDivElement>(null);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onOpenChange(false);
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, onOpenChange]);

  if (!open) return null;

  return createPortal(
    <div
      ref={overlayRef}
      className={styles.overlay}
      onClick={(e) => {
        if (e.target === overlayRef.current) onOpenChange(false);
      }}
    >
      <div className={styles.content} role="dialog" aria-modal>
        <div className={styles.handle} aria-hidden />
        {children}
      </div>
    </div>,
    document.body,
  );
}

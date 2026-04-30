import type { ButtonHTMLAttributes } from "react";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" };
export function Button({ variant: _variant = "primary", ...props }: Props) {
  return <button {...props} />;
}

import { Outlet } from "@tanstack/react-router";
import { ThemeProvider } from "@/theme/provider";
import "@/theme/tokens.css";

export function RootLayout() {
  return (
    <ThemeProvider>
      <Outlet />
    </ThemeProvider>
  );
}

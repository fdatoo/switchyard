import { useLanguage } from "./theme/language-provider";
import { DashboardSlug } from "./routes/_authed/dashboards/$slug";
import { Login } from "./routes/login";
import { ReconnectingBanner } from "./shell/ReconnectingBanner";

export default function App() {
  const { resolvedTheme } = useLanguage();
  const path = window.location.pathname;
  if (path === "/login") {
    return (
      <>
        <ReconnectingBanner />
        <Login />
      </>
    );
  }
  if (path.startsWith("/dashboards/")) {
    return (
      <>
        <ReconnectingBanner />
        <DashboardSlug slug={decodeURIComponent(path.slice("/dashboards/".length))} />
      </>
    );
  }
  return (
    <div>
      <ReconnectingBanner />
      <p>switchyard — theme: {resolvedTheme}</p>
    </div>
  );
}

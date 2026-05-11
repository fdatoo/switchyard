import { useLanguage } from "./theme/language-provider";
import { DashboardSlug } from "./routes/_authed/dashboards/$slug";
import { Login } from "./routes/login";
import { ReconnectingBanner } from "./shell/ReconnectingBanner";
import { Automations } from "./routes/_authed/automations/index";
import { AutomationSlug } from "./routes/_authed/automations/$slug";
import { TimeMachineRun } from "./routes/_authed/time-machine/$correlationId";

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
  if (path === "/_authed/automations" || path === "/automations") {
    return <Automations />;
  }
  if (path.startsWith("/_authed/automations/") || path.startsWith("/automations/")) {
    const base = path.startsWith("/_authed/automations/") ? "/_authed/automations/" : "/automations/";
    const slug = decodeURIComponent(path.slice(base.length));
    return <AutomationSlug slug={slug} />;
  }
  if (path.startsWith("/_authed/time-machine/") || path.startsWith("/time-machine/")) {
    const base = path.startsWith("/_authed/time-machine/") ? "/_authed/time-machine/" : "/time-machine/";
    const correlationId = decodeURIComponent(path.slice(base.length));
    return <TimeMachineRun correlationId={correlationId} />;
  }
  return (
    <div>
      <ReconnectingBanner />
      <p>switchyard — theme: {resolvedTheme}</p>
    </div>
  );
}

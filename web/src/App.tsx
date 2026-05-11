import { lazy, Suspense } from "react";
import { useLanguage } from "./theme/language-provider";
import { PageSlug } from "./routes/_authed/pages/$slug";
import { Login } from "./routes/login";
import { ReconnectingBanner } from "./shell/ReconnectingBanner";
import { Automations } from "./routes/_authed/automations/index";
import { AutomationSlug } from "./routes/_authed/automations/$slug";
import { TimeMachineEvent } from "./routes/_authed/time-machine/$eventId";
import { DisplayPage } from "./routes/display.$id";
import { PairPage } from "./routes/pair";

// Pkl editor routes — lazy-loaded (Monaco is heavy)
const PklEditorRoute = lazy(() => import("./pkl-editor/route"));
const MergeRoute = lazy(() => import("./pkl-editor/merge-route"));

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
  // Public: display renderer — no Shell, auth via per-display token
  if (path.startsWith("/display/")) {
    const id = decodeURIComponent(path.slice("/display/".length));
    return <DisplayPage id={id} />;
  }
  // Public: pair code redemption — no Shell
  if (path === "/pair") {
    return <PairPage />;
  }
  // Redirect legacy /dashboards/* URLs to /pages/*
  if (path.startsWith("/dashboards/")) {
    const slug = decodeURIComponent(path.slice("/dashboards/".length));
    window.location.replace(`/pages/${slug}`);
    return null;
  }
  if (path.startsWith("/pages/")) {
    const slug = decodeURIComponent(path.slice("/pages/".length));
    return (
      <>
        <ReconnectingBanner />
        <PageSlug slug={slug} />
      </>
    );
  }
  if (path.startsWith("/_authed/pkl-editor/merge/")) {
    return (
      <Suspense fallback={null}>
        <ReconnectingBanner />
        <MergeRoute />
      </Suspense>
    );
  }
  if (path.startsWith("/_authed/pkl-editor/")) {
    return (
      <Suspense fallback={null}>
        <ReconnectingBanner />
        <PklEditorRoute />
      </Suspense>
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
    const eventId = decodeURIComponent(path.slice(base.length));
    return <TimeMachineEvent eventId={eventId} />;
  }
  return (
    <div>
      <ReconnectingBanner />
      <p>switchyard — theme: {resolvedTheme}</p>
    </div>
  );
}

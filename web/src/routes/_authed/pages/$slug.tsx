/**
 * PageSlug — renders a custom page from PageService.Get.
 * Supports render mode and edit mode (EditChrome wrapping).
 */

import { useState, useEffect } from "react";
import { Page } from "@/pages-system/render/Page";
import { EditChrome } from "@/pages-system/edit/EditChrome";
import { usePageEditor } from "@/pages-system/edit/use-page-editor";
import { pageClient } from "@/data/page-client";
import type { PageModel } from "@/pages-system/model";

// Import all section/tile/cell widgets so they register themselves
import "@/pages-system/widgets/sections/Hero";
import "@/pages-system/widgets/sections/Chart";
import "@/pages-system/widgets/sections/EntityList";
import "@/pages-system/widgets/sections/ActivityFeed";
import "@/pages-system/widgets/sections/RoomGrid";
import "@/pages-system/widgets/sections/Markdown";
import "@/pages-system/widgets/sections/CameraGrid";
import "@/pages-system/widgets/sections/StatGrid";
import "@/pages-system/widgets/sections/WebhookButton";
import "@/pages-system/widgets/tiles/RoomTile";
import "@/pages-system/widgets/tiles/StatTile";
import "@/pages-system/widgets/tiles/EntityToggle";
import "@/pages-system/widgets/tiles/SceneButton";
import "@/pages-system/widgets/cells/EntityRow";
import "@/pages-system/widgets/cells/EventRow";

interface Props {
  slug?: string;
}

export function PageSlug({ slug = "unknown" }: Props) {
  const [pageModel, setPageModel] = useState<PageModel | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editMode, setEditMode] = useState(false);

  const { setSections, sections, resetDirty } = usePageEditor((s) => ({
    setSections: s.setSections,
    sections: s.sections,
    resetDirty: s.resetDirty,
  }));

  useEffect(() => {
    setLoading(true);
    setError(null);
    pageClient
      .get(slug)
      .then((p) => {
        setPageModel(p);
        setSections(p.sections);
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        setLoading(false);
      });
  }, [slug, setSections]);

  async function handleSave() {
    if (!pageModel) return;
    try {
      await pageClient.saveLayout(slug, sections);
      resetDirty();
      setEditMode(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  function handleDiscard() {
    if (pageModel) setSections(pageModel.sections);
    resetDirty();
    setEditMode(false);
  }

  if (loading) {
    return (
      <div
        style={{
          padding: "var(--sy-space-8)",
          color: "var(--sy-color-fg-3)",
          fontSize: "0.875rem",
        }}
      >
        Loading…
      </div>
    );
  }

  if (error) {
    return (
      <div
        style={{
          padding: "var(--sy-space-8)",
          color: "var(--sy-color-bad)",
          fontSize: "0.875rem",
        }}
      >
        Error: {error}
      </div>
    );
  }

  if (!pageModel) return null;

  const pageContent = (
    <div style={{ padding: "var(--sy-space-4)" }}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: "var(--sy-space-4)",
        }}
      >
        <h1
          style={{
            margin: 0,
            fontSize: "1.25rem",
            fontWeight: 700,
            color: "var(--sy-color-fg)",
          }}
        >
          {pageModel.title}
        </h1>
        {pageModel.writable && !editMode && (
          <button
            onClick={() => setEditMode(true)}
            style={{
              background: "none",
              border: "1px solid var(--sy-color-line)",
              borderRadius: "var(--sy-radius)",
              color: "var(--sy-color-fg-2)",
              padding: "var(--sy-space-1) var(--sy-space-3)",
              fontSize: "0.875rem",
              cursor: "pointer",
            }}
          >
            Edit page
          </button>
        )}
      </div>
      <Page page={{ ...pageModel, sections }} editMode={editMode} />
    </div>
  );

  if (editMode) {
    return (
      <EditChrome onSave={() => void handleSave()} onDiscard={handleDiscard}>
        {pageContent}
      </EditChrome>
    );
  }

  return pageContent;
}

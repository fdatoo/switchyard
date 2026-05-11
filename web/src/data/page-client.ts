/**
 * page-client.ts — typed ConnectRPC client wrapper for PageService
 * (proto: switchyard/page/v1/page.proto).
 */

import type { PageModel, SectionDef, TileDef, CellDef } from "@/pages-system/model";

interface RawProps {
  fields?: Record<string, { stringValue?: string; boolValue?: boolean; numberValue?: number }>;
}

interface RawCell {
  id?: string;
  type?: string;
  props?: RawProps;
}

interface RawTile {
  id?: string;
  type?: string;
  props?: RawProps;
}

interface RawSection {
  id?: string;
  type?: string;
  props?: RawProps;
  tiles?: RawTile[];
  cells?: RawCell[];
}

interface RawPage {
  slug?: string;
  title?: string;
  sections?: RawSection[];
  sourcePkl?: string;
  layoutPkl?: string;
  writable?: boolean;
}

async function postConnect<TRequest, TResponse>(
  procedure: string,
  body: TRequest,
): Promise<TResponse> {
  const response = await fetch("/api" + procedure, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "Connect-Protocol-Version": "1",
    },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw new Error(`page-client: ${procedure} failed: ${response.status}`);
  }
  return response.json() as Promise<TResponse>;
}

function parseProps(raw: RawProps | undefined): Record<string, unknown> {
  if (!raw?.fields) return {};
  const result: Record<string, unknown> = {};
  for (const [key, val] of Object.entries(raw.fields)) {
    if (val.stringValue !== undefined) result[key] = val.stringValue;
    else if (val.boolValue !== undefined) result[key] = val.boolValue;
    else if (val.numberValue !== undefined) result[key] = val.numberValue;
  }
  return result;
}

function parseCell(raw: RawCell): CellDef {
  return { id: raw.id ?? "", type: raw.type ?? "", props: parseProps(raw.props) };
}

function parseTile(raw: RawTile): TileDef {
  return { id: raw.id ?? "", type: raw.type ?? "", props: parseProps(raw.props) };
}

function parseSection(raw: RawSection): SectionDef {
  return {
    id: raw.id ?? "",
    type: raw.type ?? "",
    props: parseProps(raw.props),
    tiles: (raw.tiles ?? []).map(parseTile),
    cells: (raw.cells ?? []).map(parseCell),
  };
}

function parsePage(raw: RawPage): PageModel {
  return {
    slug: raw.slug ?? "",
    title: raw.title ?? "",
    sections: (raw.sections ?? []).map(parseSection),
    writable: raw.writable ?? false,
  };
}

export const pageClient = {
  async get(slug: string): Promise<PageModel> {
    const res = await postConnect<{ slug: string }, { page?: RawPage }>(
      "/switchyard.page.v1.PageService/Get",
      { slug },
    );
    return parsePage(res.page ?? {});
  },

  async saveLayout(slug: string, sections: SectionDef[]): Promise<void> {
    await postConnect<{ slug: string; sections: SectionDef[] }, Record<string, never>>(
      "/switchyard.page.v1.PageService/SaveLayout",
      { slug, sections },
    );
  },
};

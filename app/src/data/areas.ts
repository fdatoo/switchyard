/**
 * AreaService client. Areas are the room/zone hierarchy used by the
 * Rooms page and any area-scoped UI affordances.
 */

import { rpcCall, type RpcOptions } from "./rpc";

export interface Area {
  id: string;
  displayName: string;
  parentId: string;
}

export interface ListAreasResponse {
  areas: Area[];
}

interface RawArea {
  id?: string;
  display_name?: string; displayName?: string;
  parent_id?: string;    parentId?: string;
}

function decode(r: RawArea): Area {
  return {
    id:          r.id ?? "",
    displayName: r.displayName ?? r.display_name ?? r.id ?? "",
    parentId:    r.parentId    ?? r.parent_id    ?? "",
  };
}

export async function listAreas(opts: RpcOptions = {}): Promise<ListAreasResponse> {
  const res = await rpcCall<Record<string, never>, { areas?: RawArea[] }>(
    "switchyard.v1alpha1.AreaService/List",
    {},
    opts,
  );
  return { areas: (res.areas ?? []).map(decode) };
}

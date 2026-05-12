/**
 * EntityService client. Lists registered entities and lets callers read
 * a single entity's current state. CallCapability and Subscribe live
 * here too eventually; for now we expose just enough for the Home,
 * Rooms, and Devices views.
 */

import { rpcCall, type RpcOptions } from "./rpc";

export interface Entity {
  id: string;
  type: string;
  deviceId: string;
  areaId: string;
  zoneId: string;
  friendlyName: string;
}

export interface ListEntitiesResponse {
  entities: Entity[];
}

interface RawEntity {
  id?: string;
  type?: string;
  device_id?: string; deviceId?: string;
  area_id?: string;   areaId?: string;
  zone_id?: string;   zoneId?: string;
  friendly_name?: string; friendlyName?: string;
}

function decode(r: RawEntity): Entity {
  return {
    id:           r.id ?? "",
    type:         r.type ?? "",
    deviceId:     r.deviceId     ?? r.device_id     ?? "",
    areaId:       r.areaId       ?? r.area_id       ?? "",
    zoneId:       r.zoneId       ?? r.zone_id       ?? "",
    friendlyName: r.friendlyName ?? r.friendly_name ?? "",
  };
}

export async function listEntities(opts: RpcOptions = {}): Promise<ListEntitiesResponse> {
  const res = await rpcCall<Record<string, never>, { entities?: RawEntity[] }>(
    "switchyard.v1alpha1.EntityService/List",
    {},
    opts,
  );
  return { entities: (res.entities ?? []).map(decode) };
}

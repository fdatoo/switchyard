/**
 * SceneService client. Lists declared scenes and applies them.
 * Scenes can be global (empty areaId) or scoped to a room.
 */

import { rpcCall, type RpcOptions } from "./rpc";

const SCENE_SVC = "switchyard.v1alpha1.SceneService";

export interface Scene {
  id: string;
  displayName: string;
  areaId: string;  // "" = global
}

interface RawScene {
  id?: string;
  display_name?: string; displayName?: string;
  area_id?: string;      areaId?: string;
}

function decode(r: RawScene): Scene {
  return {
    id:          r.id ?? "",
    displayName: r.displayName ?? r.display_name ?? "",
    areaId:      r.areaId      ?? r.area_id      ?? "",
  };
}

export async function listScenes(opts: RpcOptions = {}): Promise<{ scenes: Scene[] }> {
  const res = await rpcCall<Record<string, never>, { scenes?: RawScene[] }>(
    `${SCENE_SVC}/List`, {}, opts,
  );
  return { scenes: (res.scenes ?? []).map(decode) };
}

export interface ApplySceneResult {
  correlationId: string;
}

export async function applyScene(id: string, opts: RpcOptions = {}): Promise<ApplySceneResult> {
  const res = await rpcCall<unknown, { correlationId?: string; correlation_id?: string }>(
    `${SCENE_SVC}/Apply`, { id }, opts,
  );
  return { correlationId: res.correlationId ?? res.correlation_id ?? "" };
}

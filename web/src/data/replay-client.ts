/**
 * replay-client.ts
 *
 * Connect JSON protocol client for ReplayService.
 * Hand-maintained in the same pattern as activity-client.ts.
 */

import type {
  LoadAtSeqRequest,
  LoadAtSeqResponse,
  CausationChainRequest,
  CausationChainResponse,
  WindowRequest,
  WindowResponse,
  ChainEvent,
  EntityState,
  StateDiff,
} from "../gen/replay/v1/replay_pb";

export type { ChainEvent, EntityState, StateDiff, LoadAtSeqResponse, CausationChainResponse, WindowResponse };

const BASE_URL = "/switchyard.replay.v1.ReplayService";

async function callUnary<TReq, TResp>(procedure: string, body: TReq): Promise<TResp> {
  const response = await fetch(`${BASE_URL}/${procedure}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "Connect-Protocol-Version": "1",
    },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw new Error(`ReplayService/${procedure} failed: ${response.status}`);
  }

  return response.json() as Promise<TResp>;
}

/**
 * loadAtSeq reconstructs entity state at the given event-store sequence.
 * Returns the entity-state map, diff vs the previous step, and event metadata.
 * Also exposes whyInteresting sourced from event.tags["interestingness_reason"].
 */
export async function loadAtSeq(seq: string | number): Promise<LoadAtSeqResponse> {
  const resp = await callUnary<LoadAtSeqRequest, LoadAtSeqResponse>("LoadAtSeq", {
    seq: String(seq),
  });
  return resp;
}

/**
 * causationChain walks the causation_id links for the given event_id and
 * returns the chain root-first.
 */
export async function causationChain(eventId: string): Promise<CausationChainResponse> {
  return callUnary<CausationChainRequest, CausationChainResponse>("CausationChain", {
    eventId,
  });
}

/**
 * window returns event metadata for the given sequence range.
 */
export async function window(fromSeq: string | number, toSeq: string | number): Promise<WindowResponse> {
  return callUnary<WindowRequest, WindowResponse>("Window", {
    fromSeq: String(fromSeq),
    toSeq: String(toSeq),
  });
}

/**
 * activity-client.ts
 *
 * Connect-ES-style client wrapper for ActivityService.
 * Since the web project does not have the buf ES plugin wired, we use
 * the Connect JSON protocol via fetch and hand-maintained types from
 * web/src/gen/activity/v1/activity_pb.ts.
 */

import type {
  StoriesFilter,
  EventsFilter,
  StoriesResponse,
  EventsResponse,
  EventDetailResponse,
  SaveQueryRequest,
  SaveQueryResponse,
  ListSavedQueriesResponse,
  DeleteSavedQueryResponse,
  Story,
  EventRecord,
  SavedQuery,
} from "../gen/activity/v1/activity_pb";

export type { Story, EventRecord, SavedQuery };
export type { StoriesFilter, EventsFilter };

const BASE_URL = "/switchyard.activity.v1.ActivityService";

async function callUnary<TReq, TResp>(
  procedure: string,
  body: TReq,
): Promise<TResp> {
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
    throw new Error(`ActivityService/${procedure} failed: ${response.status}`);
  }

  return response.json() as Promise<TResp>;
}

/**
 * listStories returns stories from the ActivityService filtered by `filter`.
 */
export async function listStories(filter?: StoriesFilter): Promise<Story[]> {
  const resp = await callUnary<{ filter?: StoriesFilter; cursor?: string }, StoriesResponse>(
    "Stories",
    { filter },
  );
  return resp.stories ?? [];
}

/**
 * listEvents returns events from the ActivityService filtered by `filter`.
 */
export async function listEvents(filter?: EventsFilter): Promise<EventRecord[]> {
  const resp = await callUnary<{ filter?: EventsFilter; cursor?: string }, EventsResponse>(
    "Events",
    { filter },
  );
  return resp.events ?? [];
}

/**
 * getEventDetail returns a single event with its causation chain and tags.
 */
export async function getEventDetail(eventId: string): Promise<EventDetailResponse> {
  return callUnary<{ eventId: string }, EventDetailResponse>("EventDetail", { eventId });
}

/**
 * saveQuery persists a named query.
 */
export async function saveQuery(req: SaveQueryRequest): Promise<SaveQueryResponse> {
  return callUnary<SaveQueryRequest, SaveQueryResponse>("SaveQuery", req);
}

/**
 * listSavedQueries returns all saved queries for the caller.
 */
export async function listSavedQueries(): Promise<SavedQuery[]> {
  const resp = await callUnary<Record<string, never>, ListSavedQueriesResponse>(
    "ListSavedQueries",
    {},
  );
  return resp.queries ?? [];
}

/**
 * deleteSavedQuery removes a saved query by id.
 */
export async function deleteSavedQuery(id: string): Promise<DeleteSavedQueryResponse> {
  return callUnary<{ id: string }, DeleteSavedQueryResponse>("DeleteSavedQuery", { id });
}

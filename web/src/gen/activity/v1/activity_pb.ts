// Hand-maintained TypeScript types mirroring proto/switchyard/activity/v1/activity.proto
// These types are serialised as Connect-protocol JSON by the server.

// ---------------------------------------------------------------------------
// Shared types
// ---------------------------------------------------------------------------

export type InterestingnessCategory =
  | "failure"
  | "performance"
  | "causation"
  | "anomaly"
  | "security"
  | "configuration"
  | "novelty";

export interface InterestingnessTag {
  category: InterestingnessCategory;
  name: string;
  explanation: string;
}

export interface Story {
  id: string;
  title: string;
  innerEventIds: string[];
  occurredAt: string; // ISO-8601 timestamp
  tags: InterestingnessTag[];
  source: string;
  entityIds: string[];
}

export interface EventRecord {
  eventId: string;
  commandId: string;
  causationId: string;
  correlationId: string;
  kind: string;
  entity: string;
  source: string;
  sequence: string; // uint64 as string
  occurredAt: string; // ISO-8601
  payloadJson: string;
  otelTraceId: string;
  otelSpanId: string;
  tags: InterestingnessTag[];
}

export interface SavedQuery {
  id: string;
  name: string;
  filter: string;
  cron: string;
  lastRun: string | null; // ISO-8601 or null
  nextRun: string | null;
  createdAt: string;
}

export interface StoriesFilter {
  kind?: string;
  source?: string;
  entityId?: string;
  interestingOnly?: boolean;
  interestingCategory?: InterestingnessCategory | "";
  since?: string; // ISO-8601
  until?: string;
  freeText?: string;
}

export interface EventsFilter {
  kind?: string;
  source?: string;
  entityId?: string;
  issuedBy?: string;
  interestingOnly?: boolean;
  interestingCategory?: InterestingnessCategory | "";
  since?: string;
  until?: string;
  freeText?: string;
  cursor?: string;
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

export interface StoriesRequest {
  filter?: StoriesFilter;
  cursor?: string;
}

export interface StoriesResponse {
  story: Story;
}

export interface EventsRequest {
  filter?: EventsFilter;
  cursor?: string;
}

export interface EventsResponse {
  event: EventRecord;
}

export interface EventDetailRequest {
  eventId: string;
}

export interface EventDetailResponse {
  event: EventRecord;
  causationChain: EventRecord[];
}

export interface SaveQueryRequest {
  name: string;
  filter: string;
  cron?: string;
}

export interface SaveQueryResponse {
  query: SavedQuery;
}

export type ListSavedQueriesRequest = Record<string, never>;

export interface ListSavedQueriesResponse {
  queries: SavedQuery[];
}

export interface DeleteSavedQueryRequest {
  id: string;
}

export type DeleteSavedQueryResponse = Record<string, never>;

/**
 * ActivityService client.
 *
 * Wraps `switchyard.activity.v1.ActivityService` so views can deal with
 * camelCase types and never see wire details (snake_case, Timestamp
 * encoding, opaque cursors).
 *
 * Only the methods the Activity page needs are exposed today — Stories,
 * Events, and EventDetail. Saved-query CRUD is intentionally omitted
 * until the Saved tab is built.
 *
 * Timestamps cross the wire as `{seconds, nanos}` objects on JSON. The
 * decode helpers turn them into JS `Date`s — easier for views to format
 * with the standard Intl APIs.
 */

import { rpcCall, type RpcOptions } from "./rpc";

/** Subset of EventRecord fields the UI actually renders. */
export interface EventRecord {
  eventId: string;
  commandId: string;
  causationId: string;
  correlationId: string;
  kind: string;
  entity: string;
  source: string;
  sequence: number;
  occurredAt: Date;
  payloadJson: string;
  tags: InterestingnessTag[];
}

export interface Story {
  id: string;
  title: string;
  innerEventIds: string[];
  occurredAt: Date;
  tags: InterestingnessTag[];
  source: string;
  entityIds: string[];
}

export interface InterestingnessTag {
  category: string;
  name: string;
  explanation: string;
}

export interface EventsFilter {
  kind?: string;
  source?: string;
  entityId?: string;
  /** Match if the story / event touches ANY of these entities. Forms a
   *  union with entityId when both are set. Currently honoured by the
   *  Stories filter; Events ignores it. */
  entityIds?: string[];
  freeText?: string;
  interestingOnly?: boolean;
  /** ISO 8601 or Date — converted to {seconds, nanos} on the wire. */
  since?: Date;
  until?: Date;
}

export interface ListEventsArgs {
  filter?: EventsFilter;
  cursor?: string;
}

export interface ListEventsResponse {
  events: EventRecord[];
  nextCursor: string;
}

export interface ListStoriesArgs {
  filter?: EventsFilter;
  cursor?: string;
}

export interface ListStoriesResponse {
  stories: Story[];
  nextCursor: string;
}

/* ---- Wire shapes ----------------------------------------------------- */

/**
 * Wire timestamp. Connect-RPC + protojson encode `google.protobuf.Timestamp`
 * as an RFC 3339 string (e.g. `"2026-05-11T09:35:42.411139Z"`). We support
 * the `{seconds, nanos}` object form too just in case a future variant
 * encoder uses it — defensive decoding is cheaper than a runtime surprise.
 */
type RawTimestamp = string | { seconds?: number | string; nanos?: number };

interface RawTag {
  category?: string;
  name?: string;
  explanation?: string;
}

interface RawEvent {
  event_id?: string;       eventId?: string;
  command_id?: string;     commandId?: string;
  causation_id?: string;   causationId?: string;
  correlation_id?: string; correlationId?: string;
  kind?: string;
  entity?: string;
  source?: string;
  sequence?: number | string;
  occurred_at?: RawTimestamp; occurredAt?: RawTimestamp;
  payload_json?: string;   payloadJson?: string;
  tags?: RawTag[];
}

interface RawStory {
  id?: string;
  title?: string;
  inner_event_ids?: string[]; innerEventIds?: string[];
  occurred_at?: RawTimestamp; occurredAt?: RawTimestamp;
  tags?: RawTag[];
  source?: string;
  entity_ids?: string[]; entityIds?: string[];
}

/* ---- Decode helpers -------------------------------------------------- */

function decodeTimestamp(t: RawTimestamp | undefined): Date {
  if (!t) return new Date(0);
  if (typeof t === "string") return new Date(t);
  const seconds = typeof t.seconds === "string" ? Number(t.seconds) : (t.seconds ?? 0);
  const nanos = t.nanos ?? 0;
  return new Date(seconds * 1000 + nanos / 1_000_000);
}

function decodeTag(t: RawTag): InterestingnessTag {
  return {
    category: t.category ?? "",
    name: t.name ?? "",
    explanation: t.explanation ?? "",
  };
}

function decodeEvent(r: RawEvent): EventRecord {
  const seqRaw = r.sequence;
  const sequence =
    typeof seqRaw === "string" ? Number(seqRaw) : (seqRaw ?? 0);
  return {
    eventId:       r.eventId       ?? r.event_id       ?? "",
    commandId:     r.commandId     ?? r.command_id     ?? "",
    causationId:   r.causationId   ?? r.causation_id   ?? "",
    correlationId: r.correlationId ?? r.correlation_id ?? "",
    kind:          r.kind          ?? "",
    entity:        r.entity        ?? "",
    source:        r.source        ?? "",
    sequence,
    occurredAt:    decodeTimestamp(r.occurredAt ?? r.occurred_at),
    payloadJson:   r.payloadJson   ?? r.payload_json   ?? "",
    tags:          (r.tags ?? []).map(decodeTag),
  };
}

function decodeStory(r: RawStory): Story {
  return {
    id:            r.id ?? "",
    title:         r.title ?? "",
    innerEventIds: r.innerEventIds ?? r.inner_event_ids ?? [],
    occurredAt:    decodeTimestamp(r.occurredAt ?? r.occurred_at),
    tags:          (r.tags ?? []).map(decodeTag),
    source:        r.source ?? "",
    entityIds:     r.entityIds ?? r.entity_ids ?? [],
  };
}

/**
 * Convert a Date filter bound to the wire's RFC 3339 string form.
 * Returns undefined for an undefined input so the field is omitted
 * entirely (proto3 distinguishes absent from zero on the read side).
 */
function encodeTimestamp(d?: Date): string | undefined {
  return d ? d.toISOString() : undefined;
}

function encodeFilter(f: EventsFilter | undefined): Record<string, unknown> {
  if (!f) return {};
  return {
    kind: f.kind,
    source: f.source,
    entity_id: f.entityId,
    entity_ids: f.entityIds && f.entityIds.length > 0 ? f.entityIds : undefined,
    free_text: f.freeText,
    interesting_only: f.interestingOnly,
    since: encodeTimestamp(f.since),
    until: encodeTimestamp(f.until),
  };
}

/* ---- RPC operations ------------------------------------------------- */

export async function listEvents(
  args: ListEventsArgs = {},
  opts: RpcOptions = {},
): Promise<ListEventsResponse> {
  const res = await rpcCall<
    { filter?: Record<string, unknown>; cursor?: string },
    { events?: RawEvent[]; next_cursor?: string; nextCursor?: string }
  >(
    "switchyard.activity.v1.ActivityService/Events",
    {
      filter: encodeFilter(args.filter),
      cursor: args.cursor,
    },
    opts,
  );
  return {
    events: (res.events ?? []).map(decodeEvent),
    nextCursor: res.nextCursor ?? res.next_cursor ?? "",
  };
}

export interface EventDetailResponse {
  event: EventRecord;
  /** Ancestors of `event` in causation order, oldest first. */
  causationChain: EventRecord[];
}

export async function eventDetail(
  eventId: string,
  opts: RpcOptions = {},
): Promise<EventDetailResponse> {
  const res = await rpcCall<
    { event_id: string },
    { event?: RawEvent; causation_chain?: RawEvent[]; causationChain?: RawEvent[] }
  >(
    "switchyard.activity.v1.ActivityService/EventDetail",
    { event_id: eventId },
    opts,
  );
  return {
    event: decodeEvent(res.event ?? {}),
    causationChain: (res.causationChain ?? res.causation_chain ?? []).map(decodeEvent),
  };
}

export async function listStories(
  args: ListStoriesArgs = {},
  opts: RpcOptions = {},
): Promise<ListStoriesResponse> {
  const res = await rpcCall<
    { filter?: Record<string, unknown>; cursor?: string },
    { stories?: RawStory[]; next_cursor?: string; nextCursor?: string }
  >(
    "switchyard.activity.v1.ActivityService/Stories",
    {
      filter: encodeFilter(args.filter),
      cursor: args.cursor,
    },
    opts,
  );
  return {
    stories: (res.stories ?? []).map(decodeStory),
    nextCursor: res.nextCursor ?? res.next_cursor ?? "",
  };
}

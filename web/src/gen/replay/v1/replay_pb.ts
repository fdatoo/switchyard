// Hand-maintained TypeScript types mirroring proto/switchyard/replay/v1/replay.proto
// These types are serialised as Connect-protocol JSON by the server.

// ---------------------------------------------------------------------------
// Shared value types
// ---------------------------------------------------------------------------

export interface FieldDiff {
  field: string;
  was: string;
  now: string;
}

export interface EntityDiff {
  entityId: string;
  fieldDiffs: FieldDiff[];
}

export interface StateDiff {
  entityDiffs: EntityDiff[];
}

export interface EntityState {
  entityId: string;
  fields: Record<string, string>;
}

// ---------------------------------------------------------------------------
// LoadAtSeq
// ---------------------------------------------------------------------------

export interface LoadAtSeqRequest {
  seq: string; // uint64 as string
}

export interface LoadAtSeqResponse {
  seq: string; // uint64 as string
  entities: EntityState[];
  diff: StateDiff;
  eventId: string;
  kind: string;
  entityId: string;
  source: string;
  causationId: string;
  correlationId: string;
  emitter: string;
  spanId: string;
  occurredAt: string; // ISO-8601
  payloadJson: string;
  whyInteresting: string;
}

// ---------------------------------------------------------------------------
// CausationChain
// ---------------------------------------------------------------------------

export interface CausationChainRequest {
  eventId: string;
}

export interface ChainEvent {
  eventId: string;
  seq: string; // uint64 as string
  causationId: string;
  kind: string;
  entityId: string;
  occurredAt: string; // ISO-8601
}

export interface CausationChainResponse {
  events: ChainEvent[];
}

// ---------------------------------------------------------------------------
// Window
// ---------------------------------------------------------------------------

export interface WindowRequest {
  fromSeq: string; // uint64 as string
  toSeq: string;
}

export interface WindowResponse {
  events: ChainEvent[];
}

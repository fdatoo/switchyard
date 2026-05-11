/**
 * Time Machine — event-keyed route.
 *
 * Loaded via App.tsx path matching:
 *   /_authed/time-machine/<eventId>
 *
 * Replaces the Plan 10 placeholder ($correlationId.tsx).
 * Route is full-screen — no standard Shell sidebar.
 */

import { TimeMachinePage } from "../../../pages/time-machine/TimeMachinePage";

interface Props {
  eventId: string;
}

export function TimeMachineEvent({ eventId }: Props) {
  return <TimeMachinePage mode="event" eventId={eventId} />;
}

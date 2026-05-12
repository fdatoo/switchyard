<!--
  SyAmbientRoomTile — room tile for ambient wall displays.

  The big-glass-tile rendering of a room used by Display surfaces (wall
  tablets, fridge displays, bedside docks, phone lock screens). Per the
  vision spec §13.1, ambient renders rooms as room tiles configurable on
  three axes:
    - `width`     — `standard` (1 col) / `wide` (2 cols across the grid)
    - `metric`    — pre-formatted single line; consumer chooses from
                    sensor / presence / now-playing / next-automation /
                    last-activity per server-side defaults or per-tile
                    overrides
    - `scenes`    — 0 to 4 inline scene chips for one-tap activation
    - `urgency`   — three-tier attention level: `normal` (default glass),
                    `notice` (accent-bordered with gentle glow, for
                    doorbell/visitor/delivery — attention-grabbing but
                    non-urgent), `alert` (solid red, visceral pulsing,
                    for fire/leak/intrusion).

  Visually distinct from the friendly SyRoomTile: larger, glassier, no
  border (just the bordered backdrop-blur surface), larger radius,
  display-weight typography, oversized touch targets for at-a-glance
  reading from across the room.

  This component is ambient-only by design. Friendly renders rooms with
  SyRoomTile; developer renders them as SyDataTable rows. Same data,
  three surfaces — that's the three-language architecture working.
-->
<script setup lang="ts">
import { computed } from "vue";
import SyText from "@/lib/components/text/SyText.vue";
import SyIcon from "@/lib/components/icon/SyIcon.vue";

export interface SceneDef {
  id: string;
  label: string;
}

type Urgency = "normal" | "notice" | "alert";

const props = withDefaults(
  defineProps<{
    name: string;
    /** Tile width on the grid: standard takes 1 col, wide takes 2. */
    width?: "standard" | "wide";
    /** Pre-formatted single-line metric. Empty string or omit to render none. */
    metric?: string;
    /** Up to 4 scene chips. Beyond that, the row scrolls. */
    scenes?: SceneDef[];
    /**
     * Three-tier urgency:
     *   - `normal` (default) — glass tile.
     *   - `notice` — accent border + gentle glow, for doorbell / visitor /
     *     delivery / package arrivals. Attention-grabbing, not urgent.
     *   - `alert` — solid red, fast pulsing, for fire / leak / intrusion.
     */
    urgency?: Urgency;
    /**
     * Custom uppercase label for the urgency banner. Defaults to "NOTICE" /
     * "ALERT" based on urgency. Use to add context: "DOORBELL", "DELIVERY",
     * "SMOKE", "WATER LEAK", etc.
     */
    urgencyLabel?: string;
  }>(),
  { width: "standard", scenes: () => [], urgency: "normal" },
);

const emit = defineEmits<{
  /** Whole tile tapped — open room detail. */
  select: [];
  /** Inline scene chip tapped. */
  scene: [id: string];
}>();

const classes = computed(() => [
  "sy-amb-room",
  `sy-amb-room--${props.width}`,
  props.urgency !== "normal" && `sy-amb-room--${props.urgency}`,
]);

const showBanner = computed(() => props.urgency !== "normal");
const bannerLabel = computed(() =>
  props.urgencyLabel ?? (props.urgency === "alert" ? "ALERT" : "NOTICE"),
);
const bannerIcon = computed(() =>
  props.urgency === "alert" ? "alert" : "sparkle",
);
const ariaRole = computed(() =>
  props.urgency === "alert" ? "alert" : props.urgency === "notice" ? "status" : undefined,
);

function onTileClick(): void {
  emit("select");
}
function onSceneClick(id: string, e: Event): void {
  /* Stop propagation so the scene chip doesn't also trigger the tile's
     `select` event. Same pattern as SyAutomationCard's trailing actions. */
  e.stopPropagation();
  emit("scene", id);
}
</script>

<template>
  <button
    type="button"
    :class="classes"
    :role="ariaRole"
    :aria-label="showBanner ? `${bannerLabel}: ${name}, ${metric}` : `${name} room`"
    @click="onTileClick"
  >
    <div class="sy-amb-room__head">
      <SyText v-if="showBanner" as="div" class="sy-amb-room__banner">
        <SyIcon :name="bannerIcon" :size="20" />
        {{ bannerLabel }}
      </SyText>
      <SyText variant="display" weight="semibold" class="sy-amb-room__name">
        {{ name }}
      </SyText>
      <SyText v-if="metric" variant="body" tone="muted" class="sy-amb-room__metric">
        {{ metric }}
      </SyText>
    </div>

    <div v-if="scenes.length > 0" class="sy-amb-room__scenes">
      <button
        v-for="s in scenes"
        :key="s.id"
        type="button"
        class="sy-amb-room__scene"
        @click="onSceneClick(s.id, $event)"
      >
        {{ s.label }}
      </button>
    </div>
  </button>
</template>

<style scoped>
.sy-amb-room {
  /* Reset native button styling. */
  appearance: none;
  border: 1px solid var(--sy-color-line);
  font: inherit;
  text-align: left;
  cursor: pointer;

  display: flex;
  flex-direction: column;
  gap: var(--sy-space-4);
  justify-content: space-between;

  padding: var(--sy-space-5);
  border-radius: var(--sy-radius-xl);
  background: var(--sy-color-surface-1);
  color: var(--sy-color-fg);

  -webkit-backdrop-filter: blur(20px) saturate(140%);
  backdrop-filter: blur(20px) saturate(140%);

  min-height: 180px;
  transition: background var(--sy-motion),
              border-color var(--sy-motion),
              transform var(--sy-motion-fast);
}

/* `wide` spans 2 columns in a CSS grid context. Consumers wrap tiles in a
   grid; this lets `wide` express its width via `grid-column`. */
.sy-amb-room--wide {
  grid-column: span 2;
}

.sy-amb-room:hover:not(:disabled) {
  background: var(--sy-color-surface-2);
}
.sy-amb-room:active:not(:disabled) {
  transform: scale(0.985);
}
.sy-amb-room:focus-visible {
  outline: 3px solid var(--sy-color-accent);
  outline-offset: 3px;
}

/* Urgency banner shared by `notice` and `alert` — small uppercase label
   with leading icon. The two states style it differently below. */
.sy-amb-room__banner {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-2);
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.18em;
  margin-bottom: var(--sy-space-2);
}

/* Notice state — for doorbell, visitor, delivery, package. Attention-
   grabbing but non-urgent. Keeps the glass aesthetic but adds an accent-
   colored border + gentle glow that pulses slowly. Banner sits in the
   accent color, scene chips remain neutral so the visual hierarchy is:
   "look here" without "drop what you're doing." */
.sy-amb-room--notice {
  border-color: var(--sy-color-accent);
  box-shadow:
    0 0 20px color-mix(in srgb, var(--sy-color-accent) 35%, transparent),
    var(--sy-shadow-elevated);
  animation: sy-amb-notice 3s ease-in-out infinite;
}
.sy-amb-room--notice .sy-amb-room__banner {
  color: var(--sy-color-accent);
}

@keyframes sy-amb-notice {
  0%, 100% {
    box-shadow:
      0 0 20px color-mix(in srgb, var(--sy-color-accent) 35%, transparent),
      var(--sy-shadow-elevated);
  }
  50% {
    box-shadow:
      0 0 32px color-mix(in srgb, var(--sy-color-accent) 55%, transparent),
      var(--sy-shadow-elevated);
  }
}
@media (prefers-reduced-motion: reduce) {
  .sy-amb-room--notice { animation: none; }
}

/* Alert state — for fire, leak, intrusion.

   Visceral, not polite. The whole tile fills with the token-bad color,
   text inverts to white, an "ALERT" banner sits above the room name, and
   the perimeter pulses a wide glow every 1.2 seconds. Scene buttons
   invert their colors so they read white-on-red against the alert fill.
   Reduced-motion fallback drops the glow animation but keeps the
   high-contrast fill and label. */
.sy-amb-room--alert {
  background: var(--sy-color-bad);
  color: #fff;
  border-color: var(--sy-color-bad);
  box-shadow:
    0 0 32px color-mix(in srgb, var(--sy-color-bad) 65%, transparent),
    var(--sy-shadow-elevated);
  animation: sy-amb-alert 1.2s ease-in-out infinite;
  /* Backdrop-filter is meaningless on an opaque red fill; turning it off
     also frees the GPU for the animated box-shadow. */
  -webkit-backdrop-filter: none;
  backdrop-filter: none;
}
.sy-amb-room--alert .sy-amb-room__banner,
.sy-amb-room--alert .sy-amb-room__name,
.sy-amb-room--alert .sy-amb-room__metric {
  color: #fff;
}
.sy-amb-room--alert .sy-amb-room__metric {
  font-weight: 600;
}
.sy-amb-room--alert .sy-amb-room__scene {
  background: #fff;
  color: var(--sy-color-bad);
  border-color: #fff;
  font-weight: 600;
}
.sy-amb-room--alert .sy-amb-room__scene:hover {
  background: color-mix(in srgb, #fff 90%, var(--sy-color-bad));
}

@keyframes sy-amb-alert {
  0%, 100% {
    box-shadow:
      0 0 32px color-mix(in srgb, var(--sy-color-bad) 65%, transparent),
      var(--sy-shadow-elevated);
  }
  50% {
    box-shadow:
      0 0 56px color-mix(in srgb, var(--sy-color-bad) 95%, transparent),
      var(--sy-shadow-elevated);
  }
}
@media (prefers-reduced-motion: reduce) {
  .sy-amb-room--alert { animation: none; }
}

.sy-amb-room__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
  min-width: 0;
}

.sy-amb-room__name {
  font-size: 1.625rem;
  /* Allow wrapping for long room names ("Primary Bedroom"). */
  line-height: 1.15;
}

.sy-amb-room__metric {
  font-size: 0.9375rem;
}

.sy-amb-room__scenes {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sy-space-2);
}

.sy-amb-room__scene {
  appearance: none;
  border: 1px solid var(--sy-color-line);
  background: var(--sy-color-surface-2);
  color: var(--sy-color-fg);
  font: inherit;
  font-size: 0.875rem;
  font-weight: 500;
  padding: 8px 14px;
  border-radius: var(--sy-radius-pill);
  cursor: pointer;
  min-height: 36px;
  transition: background var(--sy-motion-fast),
              transform var(--sy-motion-fast);
}
.sy-amb-room__scene:hover {
  background: var(--sy-color-surface-3);
}
.sy-amb-room__scene:active {
  transform: scale(0.96);
}
.sy-amb-room__scene:focus-visible {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}
</style>

<!--
  SySwitch — boolean input rendered as a sliding toggle.

  Use for instantly-applied settings ("enable notifications") where the
  change takes effect immediately. Use SyCheckbox for discrete selections
  in forms ("I agree", multi-select rows).

  Token-driven, no variant components. Same hidden-real-input + styled-
  overlay pattern as SyCheckbox, so keyboard, focus, and form semantics
  are native. ARIA `role="switch"` on the input is the canonical way to
  signal toggle-vs-checkbox semantics to screen readers.

  The thumb slides via `transform: translateX(...)` with the language's
  spring easing token, giving it a slight overshoot for tactile feel.
  Translation distances are computed so the thumb sits flush with the
  inner edge of the track at both ends for every size.
-->
<script setup lang="ts">
import { computed } from "vue";

type Size = "sm" | "md" | "lg";

const props = withDefaults(
  defineProps<{
    modelValue?: boolean;
    disabled?: boolean;
    size?: Size;
  }>(),
  { modelValue: false, disabled: false, size: "md" },
);

const emit = defineEmits<{
  "update:modelValue": [value: boolean];
}>();

function onChange(e: Event): void {
  emit("update:modelValue", (e.target as HTMLInputElement).checked);
}

const classes = computed(() => [
  "sy-switch",
  `sy-switch--${props.size}`,
  props.disabled && "sy-switch--disabled",
]);
</script>

<template>
  <label :class="classes">
    <input
      type="checkbox"
      role="switch"
      class="sy-switch__input"
      :checked="modelValue"
      :disabled="disabled"
      @change="onChange"
    />
    <span class="sy-switch__track" aria-hidden="true">
      <span class="sy-switch__thumb" />
    </span>
    <span v-if="$slots.default" class="sy-switch__label">
      <slot />
    </span>
  </label>
</template>

<style scoped>
.sy-switch {
  display: inline-flex;
  align-items: center;
  gap: var(--sy-space-3);
  cursor: pointer;
  user-select: none;
  font-family: var(--sy-font-body);
}
.sy-switch--disabled { cursor: not-allowed; opacity: 0.5; }

.sy-switch__input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
  pointer-events: none;
  margin: 0;
}

.sy-switch__track {
  position: relative;
  display: inline-block;
  width: 34px;
  height: 20px;
  background: var(--sy-color-surface-3);
  border: 1px solid var(--sy-color-line);
  border-radius: var(--sy-radius-pill);
  transition: background var(--sy-motion-fast),
              border-color var(--sy-motion-fast);
  flex-shrink: 0;
}
.sy-switch--sm .sy-switch__track { width: 28px; height: 16px; }
.sy-switch--lg .sy-switch__track { width: 44px; height: 26px; }

.sy-switch__thumb {
  position: absolute;
  top: 1px;
  left: 1px;
  width: 16px;
  height: 16px;
  background: var(--sy-color-surface-1);
  border-radius: var(--sy-radius-pill);
  box-shadow: var(--sy-shadow);
  /* Spring easing gives the thumb a slight overshoot — feels tactile rather
     than mechanical. The fast (240ms) keeps it from feeling sluggish. */
  transition: transform 240ms var(--sy-motion-spring),
              background var(--sy-motion-fast);
}
.sy-switch--sm .sy-switch__thumb { width: 12px; height: 12px; }
.sy-switch--lg .sy-switch__thumb { width: 22px; height: 22px; }

.sy-switch__input:focus-visible + .sy-switch__track {
  outline: 2px solid var(--sy-color-accent);
  outline-offset: 2px;
}

.sy-switch__input:checked + .sy-switch__track {
  background: var(--sy-color-accent);
  border-color: var(--sy-color-accent);
}

/* Translate distances per size: track inner width minus thumb width minus
   the 1px starting offset = travel distance. md: 32 - 16 - 2 = 14. */
.sy-switch__input:checked + .sy-switch__track .sy-switch__thumb {
  transform: translateX(14px);
}
.sy-switch--sm .sy-switch__input:checked + .sy-switch__track .sy-switch__thumb {
  transform: translateX(12px);
}
.sy-switch--lg .sy-switch__input:checked + .sy-switch__track .sy-switch__thumb {
  transform: translateX(18px);
}

.sy-switch:hover:not(.sy-switch--disabled) .sy-switch__track {
  border-color: var(--sy-color-fg-4);
}

.sy-switch__label {
  font-size: 0.9375rem;
  color: var(--sy-color-fg);
}
.sy-switch--sm .sy-switch__label { font-size: 0.8125rem; }
.sy-switch--lg .sy-switch__label { font-size: 1rem; }
</style>

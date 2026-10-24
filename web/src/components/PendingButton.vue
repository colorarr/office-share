<script setup>
import { useDelayedIndicator } from '../composables/useDelayedIndicator.js';

const props = defineProps({
  busy: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  type: { type: String, default: 'button' },
  pendingLabel: { type: String, default: '正在保存…' },
});
const showIndicator = useDelayedIndicator(() => props.busy);

function guardPending(event) {
  if (props.busy || props.disabled) {
    event.preventDefault();
    event.stopImmediatePropagation();
  }
}
</script>

<template>
  <button
    class="button pending-button"
    :type="type"
    :disabled="disabled"
    :aria-disabled="disabled || busy"
    :aria-busy="busy"
    @click="guardPending"
  >
    <span class="pending-button-label" :class="{ concealed: busy }" :aria-hidden="busy">
      <slot />
    </span>
    <span class="pending-button-label" :class="{ concealed: !busy }" :aria-hidden="!busy">
      <span class="pending-indicator" aria-hidden="true">
        <span v-if="showIndicator" class="pending-spinner"></span>
      </span>
      {{ pendingLabel }}
    </span>
  </button>
</template>

<style scoped>
.pending-button {
  display: inline-grid;
  place-items: center;
}
.pending-button[aria-disabled='true'],
.pending-button[aria-disabled='true']:hover,
.pending-button[aria-disabled='true']:active {
  color: var(--muted);
  background: var(--raised);
  cursor: wait;
  transform: none;
}
.pending-button-label {
  grid-area: 1 / 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.concealed {
  visibility: hidden;
}
.pending-indicator,
.pending-spinner {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  display: block;
}
.pending-spinner {
  border: 1.5px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: pending-spin 800ms linear infinite;
}
@keyframes pending-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .pending-spinner {
    animation: none;
  }
}
</style>

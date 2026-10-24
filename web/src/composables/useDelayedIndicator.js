import { ref, watch } from 'vue';

// Business busy state stays immediate. This only delays decorative feedback.
export function useDelayedIndicator(source, { delay = 180, clock = globalThis } = {}) {
  const visible = ref(false);
  watch(
    source,
    (pending, _previous, onCleanup) => {
      visible.value = false;
      if (!pending) return;
      const timer = clock.setTimeout(() => (visible.value = true), delay);
      onCleanup(() => {
        clock.clearTimeout(timer);
        visible.value = false;
      });
    },
    { immediate: true, flush: 'sync' },
  );
  return visible;
}

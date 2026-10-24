import { computed, ref } from 'vue';

// Keep selection by ID so replacing the list never leaves a stale preview object.
// The editor's form remains a separate draft owned by App.vue.
export function useItemPreview(items) {
  const selectedId = ref(null);
  return computed({
    get: () => items.value.find((item) => item.id === selectedId.value) || null,
    set: (item) => {
      selectedId.value = item?.id || null;
    },
  });
}

<script setup>
import { onMounted, onUnmounted, ref } from 'vue';
import AppIcon from '../AppIcon.vue';
const props = defineProps({
  title: { type: String, required: true },
  embedded: { type: Boolean, default: false },
  wide: { type: Boolean, default: false },
  busy: { type: Boolean, default: false },
});
const emit = defineEmits(['close']);
const dialog = ref(null);
let previous;
onMounted(() => {
  previous = document.activeElement;
  if (!props.embedded) dialog.value.showModal();
});
onUnmounted(() => {
  if (!props.embedded && previous?.isConnected) previous.focus();
});
function cancel(event) {
  event.preventDefault();
  if (!props.busy) emit('close');
}
</script>
<template>
  <component
    :is="embedded ? 'section' : 'dialog'"
    ref="dialog"
    :class="{ wide, 'inline-panel': embedded }"
    :aria-labelledby="embedded ? undefined : 'dialog-title'"
    @cancel="cancel"
  >
    <div v-if="!embedded" class="dialog-head">
      <h2 id="dialog-title">{{ title }}</h2>
      <button class="icon-btn" :disabled="busy" aria-label="关闭弹窗" @click="emit('close')">
        <AppIcon name="close" />
      </button>
    </div>
    <div class="dialog-body"><slot /></div>
    <div v-if="$slots.footer" class="dialog-footer"><slot name="footer" /></div>
  </component>
</template>

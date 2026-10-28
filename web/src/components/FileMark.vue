<script setup>
import { computed } from 'vue';
import AppIcon from './AppIcon.vue';
import { fileVisualKind } from '../utils/file-presentation.js';
const props = defineProps({ item: { type: Object, required: true }, large: Boolean });
const kind = computed(() => fileVisualKind(props.item));
</script>
<template>
  <span class="file-icon" :class="[kind, { large }]">
    <AppIcon :name="kind === 'pdf' ? 'file' : kind" />
  </span>
</template>

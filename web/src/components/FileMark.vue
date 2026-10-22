<script setup>
import AppIcon from './AppIcon.vue';
import { fileType } from '../utils/format';
defineProps({ item: { type: Object, required: true }, large: Boolean });
</script>

<template>
  <span class="file-mark" :class="[item.kind, { large }]">
    <AppIcon
      v-if="item.kind === 'folder' || item.kind === 'image' || item.kind === 'text'"
      :name="item.kind"
      :size="large ? 29 : 22"
    />
    <span v-else>{{ fileType(item).slice(0, 4) }}</span>
  </span>
</template>

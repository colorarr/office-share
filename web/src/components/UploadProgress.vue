<script setup>
import { computed } from 'vue';
import { transferLabel } from '../utils/transfer-feedback.js';

const props = defineProps({ task: { type: Object, required: true } });
const indeterminate = computed(
  () =>
    props.task.status === 'processing' ||
    (props.task.status === 'uploading' && props.task.progress === null),
);
</script>

<template>
  <div
    class="transfer-track"
    :class="{ 'is-indeterminate': indeterminate }"
    role="progressbar"
    :aria-label="`${task.name}上传进度`"
    aria-valuemin="0"
    aria-valuemax="100"
    :aria-valuenow="indeterminate || task.progress === null ? undefined : task.progress"
    :aria-valuetext="transferLabel(task)"
  >
    <i
      :class="task.status"
      :style="{ transform: indeterminate ? undefined : `scaleX(${task.progress / 100})` }"
    ></i>
  </div>
</template>

<style scoped>
.transfer-track i {
  width: 100%;
  transform-origin: left;
  transition: transform 160ms ease-out;
}
.is-indeterminate i {
  width: 35%;
  animation: transfer-wait 1.4s ease-in-out infinite;
}
@keyframes transfer-wait {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(290%);
  }
}
@media (prefers-reduced-motion: reduce) {
  .transfer-track i {
    transition: none;
    animation: none;
  }
}
</style>

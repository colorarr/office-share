<script setup>
import { computed, ref } from 'vue';
import BaseDialog from './BaseDialog.vue';
import PendingButton from '../PendingButton.vue';
import { collaboration } from '../../api/collaboration.js';
const props = defineProps({
  space: { type: String, required: true },
  folders: { type: Array, required: true },
  pins: { type: Array, required: true },
});
const emit = defineEmits(['close', 'saved']);
const ids = ref([...props.pins]),
  busy = ref(false),
  error = ref('');
const ordered = computed(() => [
  ...ids.value.map((id) => props.folders.find((item) => item.id === id)).filter(Boolean),
  ...props.folders.filter((item) => !ids.value.includes(item.id)),
]);
function move(id, delta) {
  const index = ids.value.indexOf(id),
    next = index + delta;
  if (next < 0 || next >= ids.value.length) return;
  [ids.value[index], ids.value[next]] = [ids.value[next], ids.value[index]];
}
async function save() {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    const saved = await collaboration.savePins(props.space, ids.value);
    emit('saved', saved);
    emit('close');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <BaseDialog title="管理置顶文件夹" :busy="busy" @close="emit('close')"
    ><p class="dialog-note" style="margin-top: 0; margin-bottom: 16px">
      任意层级的文件夹都可置顶。仅改变你的快捷入口，不影响真实目录和其他成员。
    </p>
    <div class="pin-list">
      <div v-for="folder in ordered" :key="folder.id" class="pin-row">
        <input
          v-model="ids"
          type="checkbox"
          :value="folder.id"
          :aria-label="`置顶 ${folder.name}`"
          :disabled="busy"
        /><span class="pin-name">{{ folder.name }}</span
        ><template v-if="ids.includes(folder.id)"
          ><button
            :disabled="busy || ids[0] === folder.id"
            :aria-label="`上移 ${folder.name}`"
            @click="move(folder.id, -1)"
          >
            上移</button
          ><button
            :disabled="busy || ids.at(-1) === folder.id"
            :aria-label="`下移 ${folder.name}`"
            @click="move(folder.id, 1)"
          >
            下移
          </button></template
        >
      </div>
      <p v-if="!ordered.length" class="dialog-note">先创建文件夹，再将它置顶。</p>
    </div>
    <p v-if="error" class="dialog-error" role="alert">{{ error }}</p>
    <template #footer
      ><button class="btn outline" :disabled="busy" @click="emit('close')">取消</button
      ><PendingButton class="btn primary" :busy="busy" @click="save"
        >保存置顶</PendingButton
      ></template
    ></BaseDialog
  >
</template>

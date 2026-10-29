<script setup>
import { ref } from 'vue';
import BaseDialog from './BaseDialog.vue';
import DocumentPreview from '../DocumentPreview.vue';
import TextNotePanel from '../TextNotePanel.vue';
import { itemImageURL } from '../../api/collaboration.js';
import { api } from '../../api/client.js';
const props = defineProps({ item: { type: Object, required: true } });
const emit = defineEmits(['close', 'saved', 'share', 'notify']);
const editing = ref(false),
  draft = ref(''),
  busy = ref(false),
  error = ref('');
function edit() {
  draft.value = props.item.content;
  editing.value = true;
}
async function save() {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await api.update(props.item.id, { action: 'edit', content: draft.value });
    emit('saved');
    emit('close');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
async function copy(text) {
  try {
    await navigator.clipboard.writeText(text);
    emit('notify', '已复制文字');
  } catch {
    error.value = '请选中文字，使用系统复制快捷键。';
  }
}
</script>
<template>
  <BaseDialog :title="item.name" wide :busy="busy" @close="emit('close')"
    ><TextNotePanel
      v-if="item.kind === 'text'"
      v-model="draft"
      :item="item"
      :editing="editing"
      :busy="busy"
      :error="error"
      :can-edit="!!item.capabilities?.write"
      @edit="edit"
      @save="save"
      @cancel="editing = false"
      @copy="copy"
      @copy-link="emit('share', item)"
    /><img
      v-else-if="item.kind === 'image'"
      class="preview-image"
      :src="itemImageURL(item)"
      :alt="item.name"
    /><DocumentPreview v-else :item="item" @copy="copy" />
    <p v-if="error && item.kind !== 'text'" class="dialog-error" role="alert">{{ error }}</p>
    <template v-if="!editing" #footer
      ><button class="btn outline" @click="emit('share', item)">分享与权限</button
      ><a
        v-if="item.capabilities?.download !== false"
        class="btn primary"
        :href="`/api/files/${encodeURIComponent(item.id)}`"
        :download="item.name"
        >下载原文件</a
      ></template
    ></BaseDialog
  >
</template>

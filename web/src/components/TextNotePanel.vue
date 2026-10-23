<script setup>
import { onMounted, ref } from 'vue';
import AppIcon from './AppIcon.vue';

const props = defineProps({
  item: { type: Object, required: true },
  editing: { type: Boolean, default: false },
  modelValue: { type: String, default: '' },
  busy: { type: Boolean, default: false },
  error: { type: String, default: '' },
});
const emit = defineEmits(['update:modelValue', 'save', 'cancel', 'edit', 'copy', 'copy-link']);
const editor = ref(null);
const readContent = ref(null);
const editButton = ref(null);
let scrollPosition = 0;

function rememberScroll(element) {
  scrollPosition = element.querySelector('textarea, .text-preview')?.scrollTop || 0;
  // Only the entering mode should accept input during the handoff.
  element.inert = true;
}
function focusContent() {
  const content = props.editing ? editor.value : readContent.value;
  if (props.editing) editor.value?.focus({ preventScroll: true });
  else editButton.value?.focus({ preventScroll: true });
  if (content) content.scrollTop = scrollPosition;
}
function restoreInput(element) {
  element.inert = false;
}
onMounted(() => {
  if (props.editing) focusContent();
});
</script>

<template>
  <Transition
    name="note-mode"
    mode="out-in"
    @before-enter="restoreInput"
    @leave-cancelled="restoreInput"
    @before-leave="rememberScroll"
    @after-enter="focusContent"
  >
    <form
      v-if="editing"
      key="edit"
      class="text-editor-form"
      :aria-busy="busy"
      @submit.prevent="emit('save')"
    >
      <label for="edit-content" class="visually-hidden">文字内容</label>
      <textarea
        id="edit-content"
        ref="editor"
        :value="modelValue"
        aria-label="文字内容"
        maxlength="30000"
        :disabled="busy"
        placeholder="写点什么，或者直接粘贴…"
        @input="emit('update:modelValue', $event.target.value)"
      ></textarea>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <footer class="preview-actions">
        <span class="editor-hint">{{
          busy ? '正在保存，请稍候…' : '编辑中 · 最多 30,000 字节'
        }}</span>
        <button class="button secondary" type="button" :disabled="busy" @click="emit('cancel')">
          取消
        </button>
        <button class="button primary" :disabled="busy">
          {{ busy ? '正在保存…' : '保存修改' }}
        </button>
      </footer>
    </form>
    <div v-else key="read" class="text-note-view">
      <pre ref="readContent" class="text-preview" tabindex="0">{{ item.content }}</pre>
      <footer class="preview-actions">
        <button class="button secondary" @click="emit('copy-link')">
          <AppIcon name="link" :size="17" />复制链接
        </button>
        <button ref="editButton" class="button secondary" @click="emit('edit')">
          <AppIcon name="edit" :size="17" />编辑
        </button>
        <button class="button primary" @click="emit('copy', item.content)">
          <AppIcon name="copy" :size="17" />复制文字
        </button>
      </footer>
    </div>
  </Transition>
</template>

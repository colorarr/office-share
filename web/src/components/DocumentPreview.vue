<script setup>
import { ref, watch } from 'vue';
import AppIcon from './AppIcon.vue';
import FileMark from './FileMark.vue';
import { readTextPreview } from '../api/file-preview.js';
import { supportsTextPreview } from '../utils/text-preview.js';

const props = defineProps({ item: { type: Object, required: true } });
const emit = defineEmits(['copy']);
const loading = ref(false);
const error = ref('');
const document = ref(null);
const reload = ref(0);

watch(
  [
    () => props.item.id,
    () => props.item.name,
    () => props.item.mime,
    () => props.item.size,
    reload,
  ],
  async (_values, _previous, onCleanup) => {
    const item = props.item;
    document.value = null;
    error.value = '';
    loading.value = false;
    if (!supportsTextPreview(item)) return;
    const controller = new AbortController();
    onCleanup(() => controller.abort());
    loading.value = true;
    try {
      const result = await readTextPreview(item, controller.signal);
      if (!controller.signal.aborted) document.value = result;
    } catch (failure) {
      if (!controller.signal.aborted) error.value = failure.message;
    } finally {
      if (!controller.signal.aborted) loading.value = false;
    }
  },
  { immediate: true },
);
</script>

<template>
  <section v-if="supportsTextPreview(item)" class="document-preview" aria-label="文档内容预览">
    <div v-if="loading" class="loading-state" role="status">
      <span class="spinner"></span>正在读取文档内容…
    </div>
    <div v-else-if="error" class="document-preview-error" role="alert">
      <AppIcon name="warning" :size="24" />
      <p>{{ error }}</p>
      <button class="button secondary" @click="reload++">重新加载</button>
    </div>
    <template v-else-if="document">
      <div class="document-preview-toolbar">
        <span>文档内容 · {{ document.encoding }}</span>
        <button class="text-button" @click="emit('copy', document.content)">
          <AppIcon name="copy" :size="15" />{{ document.truncated ? '复制预览内容' : '复制全文' }}
        </button>
      </div>
      <p v-if="document.truncated" class="document-preview-notice">
        文档较大，当前显示前 256 KB；完整内容请下载原文件。
      </p>
      <pre v-if="document.content" class="document-preview-content" tabindex="0">{{
        document.content
      }}</pre>
      <p v-else class="document-preview-empty">这是一个空文档。</p>
    </template>
  </section>
  <div v-else class="file-preview">
    <FileMark :item="item" large />
    <h3>{{ item.name }}</h3>
    <p>此格式暂未接入在线预览，可下载原文件查看。</p>
  </div>
</template>

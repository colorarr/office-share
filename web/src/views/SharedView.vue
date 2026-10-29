<script setup>
import { computed, ref, watch } from 'vue';
import { api } from '../api/client.js';
import { collaboration, sharedFileURL } from '../api/collaboration.js';
import LoginView from './LoginView.vue';
import AppIcon from '../components/AppIcon.vue';
import FileMark from '../components/FileMark.vue';
import { supportsTextPreview } from '../utils/text-preview.js';
import { sizeLabel } from '../utils/format.js';
const props = defineProps({
  token: { type: String, required: true },
  user: { type: Object, default: null },
});
const emit = defineEmits(['login', 'logout']);
const info = ref(null),
  error = ref(null),
  loading = ref(false),
  text = ref(''),
  previewError = ref(''),
  path = ref([]),
  target = ref('');
let sequence = 0;
const imageURL = computed(() =>
  info.value?.item.kind === 'image' ? sharedFileURL(props.token, info.value.item.id, true) : '',
);
const downloadURL = computed(() =>
  info.value ? sharedFileURL(props.token, info.value.item.id) : '',
);
const wantsText = computed(
  () => info.value && (info.value.item.kind === 'text' || supportsTextPreview(info.value.item)),
);
async function load(id = '') {
  const turn = ++sequence;
  target.value = id;
  loading.value = true;
  error.value = null;
  info.value = null;
  text.value = '';
  previewError.value = '';
  try {
    const data = await collaboration.shared(props.token, id);
    if (turn !== sequence) return;
    info.value = data;
    if (data.item.kind === 'text') text.value = data.content;
    else if (supportsTextPreview(data.item)) {
      try {
        const preview = await collaboration.sharedPreview(props.token, data.item.id);
        if (turn === sequence)
          text.value = preview.text + (preview.truncated ? '\n\n（仅显示前 256 KiB）' : '');
      } catch (failure) {
        if (turn === sequence) previewError.value = failure.message;
      }
    }
  } catch (failure) {
    if (turn === sequence) error.value = failure;
  } finally {
    if (turn === sequence) loading.value = false;
  }
}
watch(
  [() => props.token, () => props.user?.id],
  () => {
    path.value = [];
    load();
  },
  { immediate: true },
);
function enter(item) {
  if (info.value?.item) path.value.push(info.value.item);
  load(item.id);
}
function back() {
  const parent = path.value.pop();
  load(parent?.id || '');
}
async function switchAccount() {
  await api.logout();
  emit('logout');
  load(target.value);
}
function loggedIn(user) {
  emit('login', user);
  load(target.value);
}
</script>
<template>
  <LoginView
    v-if="error?.status === 401"
    title="登录后访问分享资料"
    description="此链接需要使用受邀企业账号登录。"
    @login="loggedIn"
  />
  <main v-else class="public-share-page">
    <header class="public-share-bar">
      <a class="auth-brand" href="/"
        ><span class="brand-mark">同</span><strong>同桌 · 文件分享</strong></a
      ><span v-if="user" class="tiny muted">{{ user.name }} <a href="/">返回工作台</a></span>
    </header>
    <section class="public-share-content">
      <div v-if="loading" class="empty" role="status">正在验证访问权限并读取资料…</div>
      <div v-else-if="error" class="empty">
        <AppIcon name="lock" :size="38" />
        <h1>分享暂不可访问</h1>
        <p>{{ error.message }}</p>
        <button class="btn outline" @click="load(target)">重新检查</button
        ><button v-if="error.status === 403" class="btn outline" @click="switchAccount">
          使用其他账号登录
        </button>
      </div>
      <template v-else-if="info"
        ><div class="public-share-heading">
          <div>
            <span class="tag">{{
              info.mode === 'public'
                ? '任何人持链接访问'
                : info.mode === 'specific'
                  ? '指定成员访问'
                  : '空间成员访问'
            }}</span>
            <h1>{{ info.item.name }}</h1>
            <p class="muted tiny">
              {{ info.item.kind === 'folder' ? '文件夹' : sizeLabel(info.item.size) }} ·
              {{
                info.expires
                  ? `有效至 ${new Date(info.expires).toLocaleDateString('zh-CN')}`
                  : '永久有效'
              }}
            </p>
          </div>
          <a
            v-if="info.allowDownload && info.item.kind !== 'folder'"
            class="btn primary"
            :href="downloadURL"
            :download="info.item.name"
            ><AppIcon name="download" :size="17" />下载原文件</a
          ><span v-else-if="info.item.kind !== 'folder'" class="tag">下载已关闭</span>
        </div>
        <button v-if="path.length" class="btn outline" style="margin-bottom: 16px" @click="back">
          返回上一级
        </button>
        <div v-if="info.item.kind === 'folder'" class="shared-file-list">
          <button
            v-for="item in info.children"
            :key="item.id"
            class="shared-file-row"
            @click="enter(item)"
          >
            <FileMark :item="item" /><strong>{{ item.name }}</strong
            ><small>{{ item.kind === 'folder' ? '文件夹' : sizeLabel(item.size) }}</small
            ><AppIcon name="right" :size="15" />
          </button>
          <p v-if="!info.children?.length" class="empty">文件夹为空。</p>
        </div>
        <img
          v-else-if="imageURL"
          class="preview-image"
          :src="imageURL"
          :alt="info.item.name"
          @error="previewError = '图片预览暂不可用'"
        />
        <pre v-else-if="wantsText && !previewError" class="text-preview">{{ text }}</pre>
        <div v-else class="empty">
          <FileMark :item="info.item" large />
          <p>{{ previewError || '此格式暂未接入在线排版预览，原文件格式保持不变。' }}</p>
        </div>
        <p v-if="imageURL && previewError" class="dialog-error">{{ previewError }}</p>
        <p class="dialog-note">链接访问范围不包含其他文件或整个工作空间。</p></template
      >
    </section>
  </main>
</template>

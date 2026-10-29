<script setup>
import { computed, onMounted, reactive, ref } from 'vue';
import BaseDialog from '../common/BaseDialog.vue';
import PendingButton from '../PendingButton.vue';
import { api } from '../../api/client.js';
import { collaboration } from '../../api/collaboration.js';
const props = defineProps({
  item: { type: Object, required: true },
  defaults: { type: Object, required: true },
});
const emit = defineEmits(['close', 'saved']);
const draft = reactive({
  mode: props.defaults.shareMode || 'members',
  days: props.defaults.shareDays ?? 7,
  allowDownload: props.defaults.allowDownload !== false,
  recipients: [],
});
const busy = ref(false),
  loading = ref(false),
  error = ref(''),
  people = ref([]),
  active = ref(null),
  url = ref(''),
  copyState = ref('');
const manage = computed(() => props.item.capabilities?.manage);
const internalURL = computed(
  () =>
    `${location.origin}/?item=${encodeURIComponent(props.item.id)}&space=${encodeURIComponent(props.item.spaceId)}`,
);
const displayURL = computed(() => (manage.value ? url.value : internalURL.value));
onMounted(async () => {
  if (!manage.value) return;
  loading.value = true;
  try {
    const [users, shares] = await Promise.all([api.members(), collaboration.shares(props.item.id)]);
    people.value = users.filter((user) => !user.disabled);
    active.value = shares.find((share) => !share.revoked) || null;
    if (active.value) {
      Object.assign(draft, {
        mode: active.value.mode,
        allowDownload: active.value.allowDownload,
        recipients: [...active.value.recipients],
        days: active.value.expires
          ? Math.max(1, Math.min(30, Math.ceil((active.value.expires - Date.now()) / 86400000)))
          : 0,
      });
      if (![0, 1, 7, 30].includes(draft.days)) draft.days = 7;
    }
  } catch (failure) {
    error.value = failure.message;
  } finally {
    loading.value = false;
  }
});
function dirty() {
  url.value = '';
  copyState.value = '';
}
async function save() {
  if (busy.value) return;
  if (draft.mode === 'specific' && !draft.recipients.length) {
    error.value = '请至少选择一名成员。';
    return;
  }
  busy.value = true;
  error.value = '';
  try {
    const result = await collaboration.saveShare(props.item.id, {
      ...draft,
      days: Number(draft.days),
    });
    active.value = result.share;
    url.value = result.url;
    copyState.value = '链接已生成，请复制保存；再次生成会使旧链接失效。';
    emit('saved');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
async function revoke() {
  if (!active.value || busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await collaboration.revokeShare(active.value.id);
    active.value = null;
    url.value = '';
    copyState.value = '链接已撤销。';
    emit('saved');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
async function copy() {
  try {
    await navigator.clipboard.writeText(displayURL.value);
    copyState.value = '链接已复制';
  } catch {
    copyState.value = '请选中下方链接，使用系统复制快捷键。';
  }
}
</script>
<template>
  <BaseDialog title="分享与访问权限" :busy="busy" @close="emit('close')">
    <strong class="share-file-title">{{ item.name }}</strong>
    <p v-if="loading" role="status" class="dialog-note">正在读取分享设置…</p>
    <template v-else-if="manage">
      <div class="share-mode-options" role="radiogroup" aria-label="链接访问范围">
        <label class="share-mode-option"
          ><input
            v-model="draft.mode"
            type="radio"
            value="members"
            :disabled="busy"
            @change="dirty"
          /><span
            ><strong>空间成员</strong><small>已拥有访问权限的成员，登录后访问。</small></span
          ></label
        >
        <label class="share-mode-option"
          ><input
            v-model="draft.mode"
            type="radio"
            value="public"
            :disabled="busy"
            @change="dirty"
          /><span
            ><strong>任何人持链接访问</strong
            ><small>无需登录，仅开放当前文件或文件夹。</small></span
          ></label
        >
        <label class="share-mode-option"
          ><input
            v-model="draft.mode"
            type="radio"
            value="specific"
            :disabled="busy"
            @change="dirty"
          /><span
            ><strong>指定成员访问</strong><small>只允许选中的企业账号登录访问。</small></span
          ></label
        >
      </div>
      <p v-if="draft.mode === 'public'" class="public-warning">
        任何获得链接的人均可访问分享范围，请确认资料适合对外分享。
      </p>
      <div v-if="draft.mode === 'specific'" class="share-audience">
        <label v-for="person in people" :key="person.id" class="access-person"
          ><input
            v-model="draft.recipients"
            type="checkbox"
            :value="person.id"
            :disabled="busy"
            @change="dirty"
          /><span
            >{{ person.name }} <small class="muted">{{ person.username }}</small></span
          ></label
        >
      </div>
      <label class="share-settings-row"
        >链接有效期<select v-model.number="draft.days" :disabled="busy" @change="dirty">
          <option :value="1">1 天</option>
          <option :value="7">7 天</option>
          <option :value="30">30 天</option>
          <option :value="0">永久有效</option>
        </select></label
      >
      <label class="share-settings-row"
        ><span
          >允许下载原文件<small class="muted" style="display: block"
            >不改变成员原有空间权限；预览仍可能被截图。</small
          ></span
        ><input v-model="draft.allowDownload" type="checkbox" :disabled="busy" @change="dirty"
      /></label>
      <p v-if="active && !url" class="dialog-note">
        此文件已有分享设置。保存将生成新链接并使旧链接失效。
      </p>
    </template>
    <p v-else class="dialog-note">
      你可以复制内部访问路径；接收人仍需具备访问权限。公开链接由文件所有者或空间管理员设置。
    </p>
    <div class="share-link">
      <input
        :value="displayURL"
        readonly
        aria-label="分享链接"
        placeholder="保存设置后生成链接"
      /><button class="btn outline" :disabled="!displayURL || busy" @click="copy">复制链接</button>
    </div>
    <p v-if="copyState" class="dialog-note" role="status">{{ copyState }}</p>
    <p v-if="error" class="dialog-error" role="alert">{{ error }}</p>
    <template #footer
      ><button
        v-if="manage && active"
        class="btn danger"
        :disabled="busy"
        style="margin-right: auto"
        @click="revoke"
      >
        撤销链接</button
      ><a v-if="url" class="btn outline" :href="url" target="_blank" rel="noopener noreferrer"
        >打开分享页</a
      ><PendingButton
        v-if="manage"
        class="btn primary"
        :busy="busy"
        :disabled="loading"
        @click="save"
        >保存并生成链接</PendingButton
      ><button v-else class="btn outline" @click="emit('close')">关闭</button></template
    >
  </BaseDialog>
</template>

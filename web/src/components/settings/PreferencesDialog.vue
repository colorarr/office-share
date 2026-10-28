<script setup>
import { reactive, ref } from 'vue';
import BaseDialog from '../common/BaseDialog.vue';
import PendingButton from '../PendingButton.vue';
import { api } from '../../api/client.js';
import { collaboration } from '../../api/collaboration.js';
const props = defineProps({
  embedded: { type: Boolean, default: false },
  preferences: { type: Object, required: true },
  user: { type: Object, required: true },
});
const emit = defineEmits(['close', 'saved', 'password-changed']);
const draft = reactive({ ...props.preferences }),
  password = reactive({ old: '', new: '' }),
  busy = ref(false),
  error = ref('');
async function save() {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    const saved = await collaboration.savePreferences({
      ...draft,
      shareDays: Number(draft.shareDays),
    });
    emit('saved', saved);
    emit('close');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
async function changePassword() {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await api.password(password);
    emit('password-changed');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <BaseDialog :embedded="embedded" title="账号与分享设置" :busy="busy" @close="emit('close')">
    <div class="settings-grid">
      <div class="setting">
        <div>
          <h3>当前账号</h3>
          <p>{{ user.name }} · {{ user.username }}</p>
        </div>
        <span class="tag green">企业成员</span>
      </div>
      <div class="setting">
        <div>
          <h3>默认文件视图</h3>
          <p>文件列表适合文档，网格适合视觉资料。</p>
        </div>
        <select aria-label="默认文件视图" v-model="draft.view">
          <option value="list">列表视图</option>
          <option value="grid">网格视图</option>
        </select>
      </div>
      <div class="setting">
        <div>
          <h3>默认分享范围</h3>
          <p>新链接的初始范围，已有分享不受影响。</p>
        </div>
        <select aria-label="默认分享范围" v-model="draft.shareMode">
          <option value="members">空间成员</option>
          <option value="public">任何人持链接</option>
          <option value="specific">指定成员</option>
        </select>
      </div>
      <div class="setting">
        <div>
          <h3>默认链接有效期</h3>
          <p>每个分享面板都可以单独调整。</p>
        </div>
        <select aria-label="默认链接有效期" v-model.number="draft.shareDays">
          <option :value="1">1 天</option>
          <option :value="7">7 天</option>
          <option :value="30">30 天</option>
          <option :value="0">永久有效</option>
        </select>
      </div>
      <div class="setting">
        <div>
          <h3>默认允许下载</h3>
          <p>关闭后仅开放支持的在线预览，链接可单独修改。</p>
        </div>
        <input aria-label="默认允许下载" v-model="draft.allowDownload" type="checkbox" />
      </div>
    </div>
    <details style="margin-top: 24px">
      <summary>修改登录密码</summary>
      <form style="margin-top: 14px" @submit.prevent="changePassword">
        <label class="form-field"
          >当前密码<input
            v-model="password.old"
            type="password"
            autocomplete="current-password"
            required
            maxlength="72" /></label
        ><label class="form-field"
          >新密码<input
            v-model="password.new"
            type="password"
            autocomplete="new-password"
            required
            minlength="12"
            maxlength="72" /></label
        ><PendingButton class="btn outline" :busy="busy" type="submit"
          >修改密码并重新登录</PendingButton
        >
      </form>
    </details>
    <p v-if="error" class="dialog-error" role="alert">{{ error }}</p>
    <template #footer
      ><button class="btn outline" :disabled="busy" @click="emit('close')">取消</button
      ><PendingButton class="btn primary" :busy="busy" @click="save"
        >保存设置</PendingButton
      ></template
    ></BaseDialog
  >
</template>

<script setup>
import { ref } from 'vue';
import { api } from '../api/client.js';
import PendingButton from '../components/PendingButton.vue';
defineProps({
  title: { type: String, default: '回到同一张工作桌' },
  description: { type: String, default: '登录企业账号，访问你的个人文件与团队资料。' },
});
const emit = defineEmits(['login']);
const username = ref(''),
  password = ref(''),
  busy = ref(false),
  error = ref('');
async function login() {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    const user = await api.login({ username: username.value, password: password.value });
    password.value = '';
    emit('login', user);
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <main class="auth-page">
    <section class="auth-card">
      <div class="auth-brand">
        <span class="brand-mark">同</span><strong>同桌 <small>OFFICE SHARE</small></strong>
      </div>
      <h1>{{ title }}</h1>
      <p>{{ description }}</p>
      <form @submit.prevent="login">
        <label class="form-field"
          >企业账号<input
            v-model="username"
            autocomplete="username"
            required
            maxlength="64"
            autofocus
            placeholder="输入登录账号" /></label
        ><label class="form-field"
          >密码<input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            maxlength="72"
            placeholder="输入密码"
        /></label>
        <p v-if="error" class="dialog-error" role="alert">{{ error }}</p>
        <PendingButton
          class="btn primary auth-submit"
          :busy="busy"
          type="submit"
          pending-label="正在登录…"
          >登录工作空间</PendingButton
        >
      </form>
      <p class="dialog-note">如需账号，请联系系统管理员。</p>
    </section>
  </main>
</template>

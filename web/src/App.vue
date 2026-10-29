<script setup>
import { onMounted, onUnmounted, ref } from 'vue';
import { api } from './api/client.js';
import WorkspaceView from './views/WorkspaceView.vue';
import LoginView from './views/LoginView.vue';
import SharedView from './views/SharedView.vue';
const user = ref(null),
  ready = ref(false);
const shareToken = new URLSearchParams(location.search).get('share');
function expired() {
  user.value = null;
}
onMounted(async () => {
  window.addEventListener('session-expired', expired);
  try {
    user.value = await api.me();
  } catch {
    /* A public share can be read without a session. */
  } finally {
    ready.value = true;
  }
});
onUnmounted(() => window.removeEventListener('session-expired', expired));
</script>
<template>
  <div v-if="!ready" class="app-boot" role="status">正在打开工作空间…</div>
  <SharedView
    v-else-if="shareToken"
    :token="shareToken"
    :user="user"
    @login="user = $event"
    @logout="expired"
  />
  <WorkspaceView v-else-if="user" :account="user" @logout="expired" />
  <LoginView v-else @login="user = $event" />
</template>

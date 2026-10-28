<script setup>
import { nextTick, onMounted, onUnmounted, ref } from 'vue';
import AppIcon from '../AppIcon.vue';
defineProps({ user: { type: Object, required: true } });
const emit = defineEmits(['settings', 'members', 'logout', 'create-space']);
const open = ref(false),
  root = ref(null),
  trigger = ref(null);
function outside(event) {
  if (!root.value?.contains(event.target)) open.value = false;
}
function choose(action) {
  open.value = false;
  emit(action);
}
async function onKey(event) {
  if (event.key === 'Escape') {
    open.value = false;
    trigger.value.focus();
    event.stopPropagation();
    return;
  }
  if (event.key === 'Tab') {
    open.value = false;
    return;
  }
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
  event.preventDefault();
  open.value = true;
  await nextTick();
  const buttons = [...root.value.querySelectorAll('[role="menuitem"]')];
  let index = buttons.indexOf(document.activeElement);
  index =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? buttons.length - 1
        : (index + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length;
  buttons[index]?.focus();
}
onMounted(() => document.addEventListener('pointerdown', outside));
onUnmounted(() => document.removeEventListener('pointerdown', outside));
</script>
<template>
  <div ref="root" class="account-control" @keydown="onKey">
    <button
      ref="trigger"
      class="account-trigger"
      aria-label="账号菜单"
      aria-haspopup="menu"
      :aria-expanded="open"
      @click="open = !open"
    >
      <span class="avatar">{{ user.name.slice(0, 1) }}</span
      ><span class="account-label"
        ><strong>{{ user.name }}</strong
        ><small>{{ user.role === 'admin' ? '系统管理员' : '企业成员' }}</small></span
      ><AppIcon name="down" :size="12" />
    </button>
    <div v-if="open" class="account-menu" role="menu" aria-label="账号操作">
      <div class="account-menu-label">{{ user.username }}</div>
      <button role="menuitem" class="account-option" @click="choose('settings')">
        <AppIcon name="settings" /><span class="account-option-label"
          ><strong>账号与分享设置</strong><small>个人偏好与默认访问范围</small></span
        ></button
      ><button role="menuitem" class="account-option" @click="choose('members')">
        <AppIcon name="users" /><span class="account-option-label"
          ><strong>成员管理</strong><small>账号与团队权限</small></span
        ></button
      ><button
        v-if="user.role === 'admin'"
        role="menuitem"
        class="account-option"
        @click="choose('create-space')"
      >
        <AppIcon name="plus" /><span class="account-option-label"
          ><strong>创建团队空间</strong><small>建立独立的协作空间</small></span
        ></button
      ><button role="menuitem" class="account-option" @click="choose('logout')">
        <AppIcon name="logout" /><span class="account-option-label"
          ><strong>退出登录</strong><small>结束当前会话</small></span
        >
      </button>
    </div>
  </div>
</template>

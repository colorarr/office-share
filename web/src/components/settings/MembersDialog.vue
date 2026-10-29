<script setup>
import { computed, onMounted, reactive, ref } from 'vue';
import BaseDialog from '../common/BaseDialog.vue';
import PendingButton from '../PendingButton.vue';
import { api } from '../../api/client.js';
import { collaboration } from '../../api/collaboration.js';
const props = defineProps({
  embedded: { type: Boolean, default: false },
  user: { type: Object, required: true },
  space: { type: Object, required: true },
});
const emit = defineEmits(['close', 'changed']);
const people = ref([]),
  members = ref([]),
  busy = ref(false),
  error = ref(''),
  create = ref(false),
  draft = reactive({ username: '', name: '', password: '' });
const manager = computed(() => ['manager', 'owner'].includes(props.space.role));
const rows = computed(() =>
  people.value.map((person) => ({
    ...person,
    spaceRole: members.value.find((member) => member.id === person.id)?.role || '',
  })),
);
async function load() {
  error.value = '';
  try {
    const [users, team] = await Promise.all([
      api.members(),
      props.space.kind === 'team'
        ? collaboration.spaceMembers(props.space.id)
        : Promise.resolve([]),
    ]);
    people.value = users;
    members.value = team;
  } catch (failure) {
    error.value = failure.message;
  }
}
onMounted(load);
async function role(person, value) {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await collaboration.setMemberRole(props.space.id, person.id, value);
    await load();
    emit('changed');
  } catch (failure) {
    error.value = failure.message;
    await load();
  } finally {
    busy.value = false;
  }
}
async function disable(person) {
  if (busy.value) return;
  if (
    !window.confirm(
      `${person.disabled ? '启用' : '停用'}账号「${person.name}」？停用会注销其会话。`,
    )
  )
    return;
  busy.value = true;
  error.value = '';
  try {
    await collaboration.setDisabled(person.id, !person.disabled);
    await load();
    emit('changed');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
async function invite() {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await api.invite({ ...draft });
    draft.password = '';
    create.value = false;
    await load();
    emit('changed');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <BaseDialog :embedded="embedded" title="成员管理" wide :busy="busy" @close="emit('close')"
    ><p class="dialog-note" style="margin-top: 0">
      {{ space.name }} · 系统账号与团队角色分开管理。个人空间默认仅本人。
    </p>
    <div style="overflow-x: auto">
      <table class="management-table">
        <thead>
          <tr>
            <th>成员</th>
            <th>系统角色</th>
            <th>空间角色</th>
            <th>账号</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="person in rows" :key="person.id">
            <td>
              <div class="person">
                <span class="avatar">{{ person.name.slice(0, 1) }}</span>
                <div>
                  {{ person.name }}<small>{{ person.username }}</small>
                </div>
              </div>
            </td>
            <td>{{ person.role === 'admin' ? '管理员' : '成员' }}</td>
            <td>
              <select
                v-if="space.kind === 'team' && manager"
                :key="person.spaceRole"
                :value="person.spaceRole"
                :aria-label="`${person.name} 的空间角色`"
                :disabled="busy || person.disabled"
                @change="role(person, $event.target.value)"
              >
                <option value="">未加入</option>
                <option value="manager">空间管理员</option>
                <option value="editor">协作者</option>
                <option value="viewer">只读成员</option></select
              ><span v-else>{{ person.spaceRole || '—' }}</span>
            </td>
            <td>
              <button
                v-if="user.role === 'admin' && person.id !== user.id"
                class="btn"
                :disabled="busy"
                @click="disable(person)"
              >
                {{ person.disabled ? '启用' : '停用' }}</button
              ><span v-else class="tag">{{ person.disabled ? '停用' : '正常' }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <button
      v-if="user.role === 'admin'"
      class="btn outline"
      style="margin-top: 20px"
      :disabled="busy"
      @click="create = !create"
    >
      创建内部账号
    </button>
    <form v-if="create" class="preview-note" @submit.prevent="invite">
      <label class="form-field"
        >姓名<input v-model="draft.name" required maxlength="30" autocomplete="off" /></label
      ><label class="form-field"
        >账号<input
          v-model="draft.username"
          required
          pattern="[a-zA-Z0-9_.-]{3,32}"
          maxlength="32"
          autocomplete="off" /></label
      ><label class="form-field"
        >初始密码（12–72 字节）<input
          v-model="draft.password"
          type="password"
          required
          minlength="12"
          maxlength="72"
          autocomplete="new-password" /></label
      ><PendingButton class="btn primary" type="submit" :busy="busy">创建成员</PendingButton>
    </form>
    <p v-if="error" class="dialog-error" role="alert">{{ error }}</p>
    <template #footer
      ><button class="btn outline" :disabled="busy" @click="emit('close')">关闭</button></template
    ></BaseDialog
  >
</template>

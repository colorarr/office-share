<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue';
import AppIcon from './components/AppIcon.vue';
import FileMark from './components/FileMark.vue';
import { api } from './api/client';
import { useWorkspace } from './composables/useWorkspace';
import { sizeLabel, dateLabel, fileType } from './utils/format';

const toast = ref(null);
let toastTimer;
function notify(message, kind = 'success') {
  toast.value = { message, kind };
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (toast.value = null), 4500);
}
const workspace = useWorkspace(notify);
const {
  user,
  items,
  overview,
  transfers,
  connection,
  loading,
  loadError,
  activeTransfers,
  refresh,
  connect,
  disconnect,
  upload,
} = workspace;
const initializing = ref(true);
const loginForm = ref({ username: '', password: '' });
const loginError = ref('');
const busy = ref(false);
const view = ref('all');
const parent = ref('');
const tab = ref('all');
const query = ref('');
const sort = ref('new');
const layout = ref('list');
const selection = ref([]);
const fileInput = ref(null);
const searchInput = ref(null);
const dragDepth = ref(0);
const modal = ref(null);
const form = ref({});
const formError = ref('');
const members = ref([]);
const preview = ref(null);
const mobileNav = ref(false);
const transferOpen = ref(false);
const isMac = /Mac|iPhone|iPad/.test(navigator.platform);
const modifier = isMac ? '⌘' : 'Ctrl';
const nav = [
  { id: 'all', label: '全部文件', icon: 'squares' },
  { id: 'recent', label: '最近共享', icon: 'clock' },
  { id: 'starred', label: '我的收藏', icon: 'star' },
  { id: 'trash', label: '回收站', icon: 'trash' },
];
const title = computed(() =>
  parent.value
    ? items.value.find((item) => item.id === parent.value)?.name || '文件夹'
    : { all: '团队共享', recent: '最近共享', starred: '我的收藏', trash: '回收站' }[view.value],
);
const subtitle = computed(
  () =>
    ({
      all: '文件、图片与灵感，都在同一个地方。',
      recent: '接住每一份新进展，不错过团队的更新。',
      starred: '重要的内容，随时回来找到。',
      trash: '误删也没关系，随时恢复共享内容。',
    })[view.value],
);
function inTrash(item) {
  let node = item;
  const visited = new Set();
  while (node) {
    if (node.deleted) return true;
    if (!node.parent || visited.has(node.id)) return false;
    visited.add(node.id);
    node = items.value.find((entry) => entry.id === node.parent);
  }
  return false;
}
const activeItems = computed(() => items.value.filter((item) => !inTrash(item)));
const scope = computed(() =>
  items.value.filter((item) => {
    if (view.value === 'trash') return item.deleted;
    if (inTrash(item)) return false;
    if (view.value === 'starred') return item.starred;
    if (view.value === 'recent') return item.created > Date.now() - 7 * 86400000;
    return query.value ? true : item.parent === parent.value;
  }),
);
const tabs = computed(() => [
  { id: 'all', label: '全部', count: scope.value.length },
  { id: 'file', label: '文件', count: scope.value.filter((i) => i.kind === 'file').length },
  { id: 'image', label: '图片', count: scope.value.filter((i) => i.kind === 'image').length },
  { id: 'text', label: '文字', count: scope.value.filter((i) => i.kind === 'text').length },
]);
const filtered = computed(() =>
  scope.value
    .filter((item) => {
      const matchesTab = tab.value === 'all' || item.kind === tab.value;
      const needle = query.value.toLocaleLowerCase();
      return (
        matchesTab &&
        (!needle ||
          `${item.name} ${item.content} ${item.owner}`.toLocaleLowerCase().includes(needle))
      );
    })
    .sort((a, b) =>
      sort.value === 'name'
        ? a.name.localeCompare(b.name, 'zh-CN')
        : sort.value === 'size'
          ? b.size - a.size
          : b.created - a.created,
    ),
);
const folders = computed(() =>
  activeItems.value.filter((i) => i.kind === 'folder' && i.parent === parent.value).slice(0, 4),
);
const crumbs = computed(() => {
  const result = [];
  let id = parent.value;
  const seen = new Set();
  while (id && !seen.has(id)) {
    seen.add(id);
    const item = items.value.find((i) => i.id === id);
    if (!item) break;
    result.unshift(item);
    id = item.parent;
  }
  return result;
});
const allSelected = computed(
  () =>
    filtered.value.length > 0 && filtered.value.every((item) => selection.value.includes(item.id)),
);
const storagePercent = computed(() =>
  Math.min(100, Math.max(1, (overview.value.used / overview.value.quota) * 100)),
);
const connectionLabel = computed(() =>
  connection.value === 'online'
    ? '实时同步已连接'
    : connection.value === 'reconnecting'
      ? '正在重新连接…'
      : '正在连接…',
);
const completedCount = computed(
  () => transfers.value.filter((task) => task.status === 'done').length,
);
const guide = [
  { icon: 'upload', label: '拖进来，就共享', description: '将文件或图片拖到这里，即可上传。' },
  {
    icon: 'copy',
    label: '截图，粘贴，发送',
    description: `截图后按 ${modifier} + V，让想法更直观。`,
  },
  { icon: 'users', label: '一起工作，不必等待', description: '多人同时传输，新内容实时可见。' },
];

async function login() {
  if (busy.value) return;
  busy.value = true;
  loginError.value = '';
  try {
    const account = await api.login(loginForm.value);
    loginForm.value.password = '';
    await connect(account);
    openLinkedItem();
  } catch (error) {
    loginError.value = error.message;
  } finally {
    busy.value = false;
  }
}
function navigate(id) {
  view.value = id;
  parent.value = '';
  tab.value = 'all';
  query.value = '';
  selection.value = [];
  mobileNav.value = false;
}
function enterFolder(id = '') {
  parent.value = id;
  view.value = 'all';
  query.value = '';
  tab.value = 'all';
  selection.value = [];
}
function openItem(item) {
  if (item.kind === 'folder') {
    enterFolder(item.id);
    return;
  }
  preview.value = item;
}
function openLinkedItem() {
  const id = new URLSearchParams(location.search).get('item');
  const item = activeItems.value.find((entry) => entry.id === id);
  if (item) openItem(item);
}
function openModal(type, item = null) {
  modal.value = type;
  formError.value = '';
  if (type === 'text') form.value = { name: '', content: '' };
  else if (type === 'rename') form.value = { id: item.id, name: item.name };
  else if (type === 'edit') form.value = { id: item.id, name: item.name, content: item.content };
  else if (type === 'invite') form.value = { username: '', name: '', password: '' };
  else if (type === 'password') form.value = { old: '', new: '' };
  else if (type === 'trash') form.value = { ids: item ? [item.id] : [...selection.value] };
  else form.value = { name: '' };
  nextTick(() => document.querySelector('.dialog input, .dialog textarea')?.focus());
}
async function saveModal() {
  if (busy.value) return;
  busy.value = true;
  formError.value = '';
  try {
    const type = modal.value;
    if (type === 'folder' || type === 'text') {
      await api.create({
        ...form.value,
        kind: type === 'folder' ? 'folder' : 'text',
        parent: parent.value,
      });
    } else if (type === 'rename') {
      await api.update(form.value.id, { action: 'rename', name: form.value.name });
    } else if (type === 'edit') {
      await api.update(form.value.id, { action: 'edit', content: form.value.content });
      preview.value = null;
    } else if (type === 'invite') {
      await api.invite(form.value);
      members.value = await api.members();
    } else if (type === 'password') {
      await api.password(form.value);
      disconnect();
      notify('密码已更新，请使用新密码登录');
    } else if (type === 'trash') {
      await Promise.all(form.value.ids.map((id) => api.update(id, { action: 'trash' })));
      selection.value = [];
      preview.value = null;
    }
    modal.value = type === 'invite' ? 'members' : null;
    if (user.value) await refresh();
    if (type !== 'password') notify(type === 'trash' ? '已移入回收站，可随时恢复' : '保存成功');
  } catch (error) {
    formError.value = error.message;
  } finally {
    busy.value = false;
  }
}
async function toggleStar(item) {
  try {
    await api.update(item.id, { action: 'star' });
    await refresh();
  } catch (error) {
    notify(error.message, 'error');
  }
}
async function restore(item) {
  try {
    await api.update(item.id, { action: 'restore' });
    await refresh();
    notify('已恢复到原位置');
  } catch (error) {
    notify(error.message, 'error');
  }
}
async function copy(text, label = '已复制到剪贴板') {
  try {
    await navigator.clipboard.writeText(text);
    notify(label);
  } catch {
    notify('请允许浏览器访问剪贴板，或选中文字使用快捷键复制', 'error');
  }
}
function copyLink(item) {
  copy(`${location.origin}/?item=${item.id}`, '已复制链接，团队成员登录后可查看');
}
async function copyImage(item) {
  try {
    const blob = await fetch(`/api/files/${item.id}?preview=1`).then((r) => r.blob());
    if (blob.type !== 'image/png') {
      notify('此格式请下载原图；一键复制支持 PNG 图片', 'error');
      return;
    }
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
    notify('图片已复制');
  } catch {
    notify('当前浏览器未完成图片复制，请下载原图', 'error');
  }
}
function download(item) {
  const anchor = document.createElement('a');
  anchor.href = `/api/files/${item.id}`;
  anchor.download = item.name;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}
function toggleSelection(id) {
  selection.value = selection.value.includes(id)
    ? selection.value.filter((value) => value !== id)
    : [...selection.value, id];
}
function toggleAll() {
  selection.value = allSelected.value ? [] : filtered.value.map((item) => item.id);
}
function acceptFiles(files) {
  if (!files.length) return;
  upload(Array.from(files), parent.value);
  transferOpen.value = true;
}
function onDrop(event) {
  event.preventDefault();
  dragDepth.value = 0;
  if (user.value) acceptFiles(event.dataTransfer.files);
}
function onDragEnter(event) {
  if (event.dataTransfer?.types.includes('Files')) {
    event.preventDefault();
    dragDepth.value++;
  }
}
function onDragLeave(event) {
  if (event.dataTransfer?.types.includes('Files'))
    dragDepth.value = Math.max(0, dragDepth.value - 1);
}
function onPaste(event) {
  if (!user.value || modal.value || preview.value) return;
  if (event.target.closest('input, textarea, [contenteditable="true"]')) return;
  const files = event.clipboardData?.files;
  if (files?.length) {
    event.preventDefault();
    acceptFiles(files);
    return;
  }
  const text = event.clipboardData?.getData('text/plain');
  if (text) {
    event.preventDefault();
    openModal('text');
    form.value.content = text.slice(0, 30000);
    form.value.name = text.split('\n')[0].slice(0, 40) || '共享文字';
  }
}
function onKey(event) {
  if (event.key === 'Escape') {
    if (!busy.value) {
      modal.value = null;
      preview.value = null;
    }
    mobileNav.value = false;
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault();
    searchInput.value?.focus();
  }
  if (
    (event.metaKey || event.ctrlKey) &&
    event.key.toLowerCase() === 'c' &&
    !event.target.closest('input,textarea') &&
    !window.getSelection()?.toString() &&
    selection.value.length === 1
  ) {
    const item = items.value.find((entry) => entry.id === selection.value[0]);
    if (item?.kind === 'text') {
      event.preventDefault();
      copy(item.content);
    }
  }
  if (event.key === 'Tab' && (modal.value || preview.value)) {
    const focusable = [
      ...document.querySelectorAll(
        '.dialog button:not(:disabled), .dialog input, .dialog textarea, .dialog [tabindex="0"]',
      ),
    ].filter((el) => el.offsetParent !== null);
    const first = focusable[0];
    const last = focusable.at(-1);
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  }
}
async function showMembers() {
  try {
    members.value = await api.members();
    modal.value = 'members';
    formError.value = '';
  } catch (error) {
    notify(error.message, 'error');
  }
}
async function logout() {
  try {
    await api.logout();
    disconnect();
    modal.value = null;
    preview.value = null;
  } catch (error) {
    notify(error.message, 'error');
  }
}
function expired() {
  if (user.value) {
    disconnect();
    modal.value = null;
    preview.value = null;
    notify('登录已过期，请重新登录', 'error');
  }
}
const modalTitles = {
  folder: '新建文件夹',
  text: '分享文字',
  rename: '重命名',
  edit: '编辑文字',
  invite: '添加团队成员',
  password: '修改密码',
  trash: '移入回收站',
  members: '团队成员',
  settings: '个人设置',
  help: '使用帮助',
};

onMounted(async () => {
  document.addEventListener('paste', onPaste);
  document.addEventListener('keydown', onKey);
  window.addEventListener('session-expired', expired);
  try {
    await connect(await api.me());
    openLinkedItem();
  } catch {
    /* 未登录时显示登录页。 */
  } finally {
    initializing.value = false;
  }
});
onUnmounted(() => {
  document.removeEventListener('paste', onPaste);
  document.removeEventListener('keydown', onKey);
  window.removeEventListener('session-expired', expired);
  disconnect();
  clearTimeout(toastTimer);
});
</script>

<template>
  <div v-if="initializing" class="initializing">
    <span class="brand-symbol"><i></i><i></i><i></i><i></i></span>
    <p>正在打开工作空间…</p>
  </div>
  <main v-else-if="!user" class="login-page">
    <section class="login-story">
      <a class="brand" href="/"
        ><span class="brand-symbol"><i></i><i></i><i></i><i></i></span
        ><span>同桌<span class="brand-en">OFFICE SHARE</span></span></a
      >
      <div class="login-story-main">
        <span class="eyebrow">A LITTLE CLOSER, WORK BETTER.</span>
        <h1>同一张办公桌，<br />不止一个地方。</h1>
        <p>把文件、截图和文字放在一起。<br />无论在哪台电脑，团队的工作都能接着往下走。</p>
        <div class="desk-illustration">
          <div class="desk-note">
            <span>TEAM WORKSPACE</span><AppIcon name="folder" :size="70" /><b
              >好想法，在这里汇合。</b
            >
            <div class="desk-note-line"></div>
            <div class="desk-note-line short"></div>
          </div>
          <div class="floating-file">
            <AppIcon name="file" :size="30" /><span>项目资料</span
            ><AppIcon name="check" :size="18" />
          </div>
          <div class="floating-text"><AppIcon name="text" :size="24" />刚刚想到的，马上分享。</div>
        </div>
      </div>
      <span class="login-foot">为团队协作留一点简单。</span>
    </section>
    <section class="login-form-side">
      <form class="login-form" @submit.prevent="login">
        <div class="login-lock"><AppIcon name="lock" :size="25" /></div>
        <h2>回到你的工作空间</h2>
        <p>登录同桌，与团队保持同步。</p>
        <label for="username">账号</label
        ><input
          id="username"
          v-model="loginForm.username"
          autocomplete="username"
          placeholder="输入团队账号"
          required
          maxlength="64"
        /><label for="password">密码</label
        ><input
          id="password"
          v-model="loginForm.password"
          type="password"
          autocomplete="current-password"
          placeholder="输入登录密码"
          required
          maxlength="72"
        />
        <p v-if="loginError" class="form-error" role="alert">{{ loginError }}</p>
        <button class="button primary login-submit" :disabled="busy">
          {{ busy ? '正在登录…' : '进入工作空间' }}<AppIcon name="arrow" :size="18" />
        </button>
        <div class="login-hint">
          <AppIcon name="info" :size="17" /><span>首次使用？请联系管理员获取团队账号。</span>
        </div>
      </form>
      <div class="login-security"><AppIcon name="lock" :size="14" />团队专属空间 · 登录后访问</div>
    </section>
  </main>
  <div
    v-else
    class="app-shell"
    @dragenter="onDragEnter"
    @dragleave="onDragLeave"
    @dragover.prevent
    @drop="onDrop"
  >
    <div v-if="mobileNav" class="sidebar-backdrop" @click="mobileNav = false"></div>
    <aside class="sidebar" :class="{ expanded: mobileNav }">
      <a class="brand" href="/" @click.prevent="navigate('all')"
        ><span class="brand-symbol"><i></i><i></i><i></i><i></i></span
        ><span>同桌<span class="brand-en">OFFICE SHARE</span></span></a
      >
      <button class="workspace-switch" @click="showMembers">
        <span class="workspace-avatar">T</span
        ><span
          ><strong>团队工作空间</strong
          ><small>{{ overview.members }} 位成员 · 共享协作</small></span
        ><AppIcon name="down" :size="15" />
      </button>
      <div class="nav-caption">工作空间</div>
      <nav class="main-nav" aria-label="工作空间导航">
        <button
          v-for="entry in nav"
          :key="entry.id"
          :class="{ active: view === entry.id }"
          @click="navigate(entry.id)"
        >
          <AppIcon :name="entry.icon" :size="20" /><span>{{ entry.label }}</span
          ><span v-if="entry.id === 'all'" class="nav-count">{{ activeItems.length }}</span>
        </button>
      </nav>
      <div class="sidebar-separator"></div>
      <div class="nav-caption">团队管理</div>
      <nav class="main-nav">
        <button @click="showMembers"><AppIcon name="users" /><span>团队成员</span></button
        ><button @click="modal = 'settings'">
          <AppIcon name="settings" /><span>个人设置</span>
        </button>
      </nav>
      <div class="sidebar-bottom">
        <div class="storage">
          <div class="storage-title">
            <AppIcon name="cloud" :size="18" /><span>共享存储空间</span
            ><small>{{ Math.round((overview.used / overview.quota) * 100) }}%</small>
          </div>
          <div class="storage-track"><i :style="{ width: `${storagePercent}%` }"></i></div>
          <p>
            <strong>{{ sizeLabel(overview.used) }}</strong> / {{ sizeLabel(overview.quota) }}
          </p>
        </div>
        <button class="help-button" @click="modal = 'help'">
          <AppIcon name="info" :size="18" />使用帮助<AppIcon name="external" :size="15" /></button
        ><button class="user-card" @click="modal = 'settings'">
          <span class="avatar">{{ user.name.slice(0, 1) }}</span
          ><span
            ><strong>{{ user.name }}</strong
            ><small>{{ user.role === 'admin' ? '空间管理员' : '团队成员' }}</small></span
          ><AppIcon name="more" />
        </button>
      </div>
    </aside>
    <div class="workspace-main">
      <header class="topbar">
        <button class="icon-button mobile-menu" aria-label="打开导航" @click="mobileNav = true">
          <AppIcon name="list" />
        </button>
        <div class="topbar-location">
          <AppIcon name="folder" :size="18" /><span>工作空间</span
          ><AppIcon name="right" :size="13" /><strong>团队共享</strong>
        </div>
        <div class="global-search">
          <AppIcon name="search" :size="18" /><input
            ref="searchInput"
            v-model="query"
            aria-label="搜索文件、图片或文字"
            placeholder="搜索文件、图片或文字…"
          /><kbd>{{ modifier }} K</kbd>
        </div>
        <div class="topbar-right">
          <span class="connection" :class="{ reconnecting: connection !== 'online' }"
            ><i></i>{{ connectionLabel }}</span
          ><button class="avatar small" aria-label="个人设置" @click="modal = 'settings'">
            {{ user.name.slice(0, 1) }}
          </button>
        </div>
      </header>
      <div class="content-wrap">
        <main class="main-content">
          <div class="page-heading">
            <div>
              <div class="section-kicker">YOUR SHARED DESK</div>
              <h1>{{ title }}<span class="title-dot"></span></h1>
              <p>{{ subtitle }}</p>
            </div>
            <div class="heading-actions">
              <button class="button secondary" @click="openModal('text')">
                <AppIcon name="text" :size="18" />分享文字</button
              ><button class="button primary" @click="fileInput.click()">
                <AppIcon name="plus" :size="18" />上传文件
              </button>
            </div>
          </div>
          <input
            ref="fileInput"
            class="visually-hidden"
            type="file"
            multiple
            aria-label="选择上传文件"
            @change="
              acceptFiles($event.target.files);
              $event.target.value = '';
            "
          />
          <div v-if="loadError" class="error-banner" role="alert">
            <AppIcon name="warning" />{{ loadError }}<button @click="refresh">重新加载</button>
          </div>
          <section
            v-if="view === 'all' && !query"
            class="upload-zone"
            @click.self="fileInput.click()"
          >
            <div class="upload-zone-icon"><AppIcon name="upload" :size="25" /></div>
            <div class="upload-zone-text">
              <h2>把文件拖到这里，让协作开始</h2>
              <p>
                支持文件、图片，单个文件最大 90 MB<span class="text-separator">|</span>或<button
                  class="text-button"
                  @click="fileInput.click()"
                >
                  选择文件
                </button>
              </p>
            </div>
            <div class="paste-shortcut">
              <div>
                <kbd>{{ modifier }}</kbd
                ><span>+</span><kbd>V</kbd>
              </div>
              <span>随手粘贴图片或文字</span>
            </div>
          </section>
          <section
            v-if="view === 'all' && !query && !parent && folders.length"
            class="folder-section"
          >
            <div class="section-title">
              <h2>常用文件夹</h2>
              <button class="text-button quiet" @click="openModal('folder')">
                <AppIcon name="plus" :size="15" />新建文件夹
              </button>
            </div>
            <div class="folder-grid">
              <button
                v-for="(folder, index) in folders"
                :key="folder.id"
                class="folder-card"
                @click="enterFolder(folder.id)"
              >
                <span class="folder-glyph" :class="`tone-${index % 3}`"
                  ><AppIcon name="folder" :size="29" /></span
                ><span
                  ><strong>{{ folder.name }}</strong
                  ><small
                    >{{
                      activeItems.filter((item) => item.parent === folder.id).length
                    }}
                    个项目</small
                  ></span
                ><AppIcon name="right" :size="16" />
              </button>
            </div>
          </section>
          <section class="files-section">
            <div class="files-heading">
              <div v-if="parent" class="breadcrumbs">
                <button @click="enterFolder()">团队共享</button
                ><template v-for="crumb in crumbs" :key="crumb.id"
                  ><AppIcon name="right" :size="12" /><button @click="enterFolder(crumb.id)">
                    {{ crumb.name }}
                  </button></template
                >
              </div>
              <h2 v-else>
                {{ query ? '搜索结果' : view === 'trash' ? '已删除的内容' : '共享内容'
                }}<span>{{ filtered.length }}</span>
              </h2>
              <div class="files-controls">
                <button
                  v-if="view !== 'trash'"
                  class="text-button quiet folder-create"
                  @click="openModal('folder')"
                >
                  <AppIcon name="plus" :size="16" />新建文件夹</button
                ><select v-model="sort" aria-label="排序方式">
                  <option value="new">最新共享</option>
                  <option value="name">按名称排序</option>
                  <option value="size">按大小排序</option>
                </select>
                <div class="view-switch">
                  <button
                    :class="{ selected: layout === 'list' }"
                    aria-label="列表视图"
                    @click="layout = 'list'"
                  >
                    <AppIcon name="list" :size="18" /></button
                  ><button
                    :class="{ selected: layout === 'grid' }"
                    aria-label="网格视图"
                    @click="layout = 'grid'"
                  >
                    <AppIcon name="grid" :size="18" />
                  </button>
                </div>
              </div>
            </div>
            <div class="file-tabs" role="tablist" aria-label="内容类型">
              <button
                v-for="entry in tabs"
                :key="entry.id"
                role="tab"
                :aria-selected="tab === entry.id"
                :class="{ active: tab === entry.id }"
                @click="
                  tab = entry.id;
                  selection = [];
                "
              >
                {{ entry.label }}<span>{{ entry.count }}</span>
              </button>
            </div>
            <div v-if="selection.length" class="selection-bar">
              <span>已选择 {{ selection.length }} 项</span
              ><button v-if="view !== 'trash'" class="text-button" @click="openModal('trash')">
                <AppIcon name="trash" :size="16" />移入回收站</button
              ><button class="text-button quiet" @click="selection = []">取消选择</button>
            </div>
            <div v-if="loading" class="loading-state">
              <span class="spinner"></span>正在整理共享内容…
            </div>
            <div v-else-if="!filtered.length" class="empty-state">
              <span
                ><AppIcon
                  :name="
                    view === 'trash'
                      ? 'trash'
                      : query
                        ? 'search'
                        : tab === 'image'
                          ? 'image'
                          : 'folder'
                  "
                  :size="34"
              /></span>
              <h3>
                {{
                  query
                    ? '没有找到相关内容'
                    : view === 'trash'
                      ? '回收站是空的'
                      : view === 'starred'
                        ? '把常用的内容收藏在这里'
                        : '这里，等待你的第一份分享'
                }}
              </h3>
              <p>
                {{
                  query
                    ? '试试其他关键词，或清除搜索条件。'
                    : view === 'starred'
                      ? '点击文件旁的星标，下次就能更快找到。'
                      : view === 'trash'
                        ? '被移除的共享内容会暂存在这里。'
                        : '拖入文件、粘贴截图，或分享一段文字。'
                }}
              </p>
              <button v-if="query" class="button secondary" @click="query = ''">清除搜索</button
              ><button
                v-else-if="view === 'all'"
                class="button secondary"
                @click="fileInput.click()"
              >
                <AppIcon name="upload" :size="17" />上传第一份文件
              </button>
            </div>
            <div v-else-if="layout === 'list'" class="table-scroll">
              <table class="file-table">
                <thead>
                  <tr>
                    <th class="check-cell">
                      <input
                        type="checkbox"
                        :checked="allSelected"
                        aria-label="选择全部内容"
                        @change="toggleAll"
                      />
                    </th>
                    <th class="name-column">名称</th>
                    <th class="owner-column">共享者</th>
                    <th class="time-column">共享时间</th>
                    <th class="size-column">大小</th>
                    <th class="actions-column"><span class="visually-hidden">操作</span></th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in filtered"
                    :key="item.id"
                    :class="{ 'row-selected': selection.includes(item.id) }"
                  >
                    <td class="check-cell">
                      <input
                        type="checkbox"
                        :checked="selection.includes(item.id)"
                        :aria-label="`选择 ${item.name}`"
                        @change="toggleSelection(item.id)"
                      />
                    </td>
                    <td>
                      <button class="file-name-button" @click="openItem(item)">
                        <FileMark :item="item" /><span
                          ><strong>{{ item.name }}</strong
                          ><small v-if="item.kind === 'text'">{{ item.content.slice(0, 50) }}</small
                          ><small v-else>{{ fileType(item) }}</small></span
                        >
                      </button>
                    </td>
                    <td class="owner-column">
                      <span class="table-owner"
                        ><span class="avatar tiny">{{ item.owner.slice(0, 1) }}</span
                        >{{ item.owner }}</span
                      >
                    </td>
                    <td class="time-column">{{ dateLabel(item.created) }}</td>
                    <td class="size-column">
                      {{ item.kind === 'folder' ? '—' : sizeLabel(item.size) }}
                    </td>
                    <td>
                      <div class="row-actions">
                        <template v-if="view !== 'trash'"
                          ><button
                            class="icon-button star-button"
                            :class="{ starred: item.starred }"
                            :aria-label="
                              item.starred ? `取消收藏 ${item.name}` : `收藏 ${item.name}`
                            "
                            :title="item.starred ? '取消收藏' : '收藏'"
                            @click="toggleStar(item)"
                          >
                            <AppIcon name="star" :size="17" /></button
                          ><button
                            v-if="item.kind === 'file' || item.kind === 'image'"
                            class="icon-button"
                            :aria-label="`下载 ${item.name}`"
                            title="下载"
                            @click="download(item)"
                          >
                            <AppIcon name="download" :size="17" /></button
                          ><button
                            v-else-if="item.kind === 'text'"
                            class="icon-button"
                            :aria-label="`复制 ${item.name}`"
                            title="复制文字"
                            @click="copy(item.content)"
                          >
                            <AppIcon name="copy" :size="17" /></button
                          ><button
                            class="icon-button optional-action"
                            :aria-label="`重命名 ${item.name}`"
                            title="重命名"
                            @click="openModal('rename', item)"
                          >
                            <AppIcon name="edit" :size="17" /></button
                          ><button
                            class="icon-button optional-action"
                            :aria-label="`删除 ${item.name}`"
                            title="移入回收站"
                            @click="openModal('trash', item)"
                          >
                            <AppIcon name="trash" :size="17" /></button></template
                        ><button
                          v-else
                          class="button small-button secondary"
                          @click="restore(item)"
                        >
                          <AppIcon name="restore" :size="16" />恢复
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="items-grid">
              <article v-for="item in filtered" :key="item.id" class="item-card">
                <button class="item-card-preview" :class="item.kind" @click="openItem(item)">
                  <img
                    v-if="item.kind === 'image'"
                    :src="`/api/files/${item.id}?preview=1`"
                    :alt="item.name"
                    loading="lazy"
                  />
                  <p v-else-if="item.kind === 'text'">{{ item.content }}</p>
                  <FileMark v-else :item="item" large />
                </button>
                <div class="item-card-details">
                  <button @click="openItem(item)">
                    <strong>{{ item.name }}</strong
                    ><small>{{ item.owner }} · {{ dateLabel(item.created) }}</small></button
                  ><button
                    v-if="view !== 'trash'"
                    class="icon-button star-button"
                    :class="{ starred: item.starred }"
                    :aria-label="`收藏 ${item.name}`"
                    @click="toggleStar(item)"
                  >
                    <AppIcon name="star" :size="18" /></button
                  ><button v-else class="icon-button" aria-label="恢复" @click="restore(item)">
                    <AppIcon name="restore" />
                  </button>
                </div>
              </article>
            </div>
            <div class="list-footer">
              <span>共 {{ filtered.length }} 项内容</span
              ><span><AppIcon name="lock" :size="13" />仅团队成员可见</span>
            </div>
          </section>
          <footer class="workspace-footer">
            <span>少一点来回传，多一点一起做。</span><span>同桌 OFFICE SHARE</span>
          </footer>
        </main>
        <aside class="right-panel">
          <div class="online-heading">
            <h2>一起在线</h2>
            <span class="online-count"><i></i>{{ overview.online }} 人</span>
          </div>
          <div class="online-summary">
            <span class="avatar">{{ user.name.slice(0, 1) }}</span>
            <div>
              <strong>{{ user.name }}<span>（你）</span></strong
              ><small>{{ connection === 'online' ? '正在工作空间中' : '正在连接工作空间' }}</small>
            </div>
            <button class="invite-mini" aria-label="查看和添加团队成员" @click="showMembers">
              <AppIcon name="plus" :size="17" />
            </button>
          </div>
          <button class="members-link" @click="showMembers">
            查看 {{ overview.members }} 位团队成员<AppIcon name="right" :size="13" />
          </button>
          <div class="right-divider"></div>
          <div class="section-title">
            <h2>空间动态</h2>
            <AppIcon name="clock" :size="17" />
          </div>
          <div v-if="!overview.activity.length" class="activity-empty">
            <p>每一份分享，都从这里开始。</p>
            <span>团队的新动态会实时显示。</span>
          </div>
          <div v-else class="activity-list">
            <div
              v-for="(activity, index) in overview.activity"
              :key="`${activity.created}-${index}`"
              class="activity-item"
            >
              <span class="activity-dot"></span>
              <div>
                <p>
                  <strong>{{ activity.name }}</strong> {{ activity.action }}
                </p>
                <span class="activity-target">{{ activity.target }}</span
                ><small>{{ dateLabel(activity.created) }}</small>
              </div>
            </div>
          </div>
          <div class="right-divider"></div>
          <section class="quick-guide">
            <span class="section-kicker">WORK A LITTLE SMARTER</span>
            <h2>分享，可以很简单。</h2>
            <div v-for="entry in guide" :key="entry.label" class="guide-row">
              <span><AppIcon :name="entry.icon" :size="19" /></span>
              <div>
                <h3>{{ entry.label }}</h3>
                <p>{{ entry.description }}</p>
              </div>
            </div>
            <button class="text-button" @click="modal = 'help'">
              了解使用方式<AppIcon name="arrow" :size="15" />
            </button>
          </section>
          <div class="right-bottom">
            <AppIcon name="cloud" :size="16" /><span>你的分享，团队实时可见。</span>
          </div>
        </aside>
      </div>
    </div>
    <div v-if="dragDepth > 0" class="drop-overlay">
      <div>
        <AppIcon name="upload" :size="50" />
        <h2>松开鼠标，即刻共享</h2>
        <p>文件将上传到「{{ parent ? title : '团队共享' }}」</p>
      </div>
    </div>
    <section v-if="transfers.length" class="transfer-panel" :class="{ collapsed: !transferOpen }">
      <button class="transfer-heading" @click="transferOpen = !transferOpen">
        <AppIcon :name="activeTransfers.length ? 'upload' : 'success'" :size="19" /><strong>{{
          activeTransfers.length
            ? `${activeTransfers.length} 个文件正在传输`
            : `已完成 ${completedCount} 个文件上传`
        }}</strong
        ><AppIcon name="down" :size="16" />
      </button>
      <div v-if="transferOpen" class="transfer-list">
        <div v-for="task in transfers" :key="task.id" class="transfer-item">
          <div>
            <span>{{ task.name }}</span
            ><small :class="task.status">{{
              task.status === 'done'
                ? '已完成'
                : task.status === 'error'
                  ? task.error
                  : task.status === 'waiting'
                    ? '等待上传'
                    : task.progress === 100
                      ? '正在保存…'
                      : `${task.progress}%`
            }}</small>
          </div>
          <div class="transfer-track">
            <i :style="{ width: `${task.progress}%` }" :class="task.status"></i>
          </div>
        </div>
        <button
          v-if="!activeTransfers.length"
          class="text-button quiet"
          @click="
            transfers = [];
            transferOpen = false;
          "
        >
          清除已完成记录
        </button>
      </div>
    </section>
  </div>

  <div
    v-if="modal || preview"
    class="modal-backdrop"
    @click.self="!busy && ((modal = null), (preview = null))"
  >
    <section
      class="dialog"
      :class="{ 'preview-dialog': preview && !modal, 'members-dialog': modal === 'members' }"
      role="dialog"
      aria-modal="true"
      :aria-label="modal ? modalTitles[modal] : preview.name"
    >
      <header class="dialog-heading">
        <div>
          <span class="section-kicker">OFFICE SHARE</span>
          <h2>{{ modal ? modalTitles[modal] : preview.name }}</h2>
        </div>
        <button
          class="icon-button"
          aria-label="关闭弹窗"
          :disabled="busy"
          @click="modal ? (modal = null) : (preview = null)"
        >
          <AppIcon name="close" :size="22" />
        </button>
      </header>
      <template v-if="modal === 'members'"
        ><p class="dialog-description">这个空间里的文件和文字，对所有团队成员可见。</p>
        <div class="member-list">
          <div v-for="member in members" :key="member.id" class="member-row">
            <span class="avatar">{{ member.name.slice(0, 1) }}</span>
            <div>
              <strong>{{ member.name }}</strong
              ><small>{{ member.username }}</small>
            </div>
            <span class="member-role">{{ member.role === 'admin' ? '管理员' : '成员' }}</span>
          </div>
        </div>
        <button v-if="user.role === 'admin'" class="button primary" @click="openModal('invite')">
          <AppIcon name="plus" :size="17" />添加团队成员
        </button></template
      >
      <template v-else-if="modal === 'settings'"
        ><div class="settings-profile">
          <span class="avatar">{{ user.name.slice(0, 1) }}</span>
          <div>
            <strong>{{ user.name }}</strong>
            <p>{{ user.username }} · {{ user.role === 'admin' ? '空间管理员' : '团队成员' }}</p>
          </div>
        </div>
        <button class="settings-row" @click="openModal('password')">
          <AppIcon name="lock" />修改登录密码<AppIcon name="right" :size="16" /></button
        ><button class="settings-row logout-row" @click="logout">
          <AppIcon name="logout" />退出登录<AppIcon name="right" :size="16" /></button
      ></template>
      <template v-else-if="modal === 'help'"
        ><div class="help-content">
          <h3>文件与图片</h3>
          <p>
            将文件拖入工作空间，或点击「上传文件」。支持同时上传多个文件，单个文件上限为 90
            MB。上传队列会显示真实进度。
          </p>
          <h3>快捷键与剪贴板</h3>
          <p>
            Windows 使用 Ctrl，Mac 使用 ⌘。在页面空白处按 {{ modifier }} + V
            可粘贴截图或文字；文字便签可点击复制。{{ modifier }} + K 快速搜索。
          </p>
          <p>
            从系统文件管理器复制文件后的网页粘贴取决于浏览器支持，文件拖拽和文件选择均可使用。图片一键复制支持
            PNG，其他格式可下载原图。
          </p>
          <h3>团队共享与回收站</h3>
          <p>
            所有成员都可以查看、下载、重命名和移除共享内容。收藏仅对自己生效。删除会移入回收站，文件夹恢复后其中的内容重新可见。
          </p>
          <h3>传输说明</h3>
          <p>
            当前版本采用流式上传，网络中断后请重新上传。支持 HTTP
            范围下载；上传断点续传将在后续版本加入。
          </p>
        </div></template
      >
      <form v-else-if="modal" class="dialog-form" @submit.prevent="saveModal">
        <template v-if="modal === 'trash'"
          ><div class="delete-notice">
            <AppIcon name="trash" :size="30" />
            <p>将 {{ form.ids.length }} 项内容移入回收站？</p>
            <small>所有团队成员将暂时看不到这些内容。你可以在回收站中恢复。</small>
          </div></template
        ><template v-else-if="modal === 'invite'"
          ><p class="dialog-description">创建独立账号，方便同事实时共享和下载。</p>
          <label for="member-name">成员姓名</label
          ><input
            id="member-name"
            v-model="form.name"
            placeholder="例如：林晓"
            required
            maxlength="30" /><label for="member-username">登录账号</label
          ><input
            id="member-username"
            v-model="form.username"
            placeholder="3–32 位字母、数字、下划线"
            pattern="[a-zA-Z0-9_-]{3,32}"
            required
            autocomplete="off" /><label for="member-password">初始密码</label
          ><input
            id="member-password"
            v-model="form.password"
            type="password"
            placeholder="至少 12 位，成员登录后可修改"
            required
            minlength="12"
            maxlength="72"
            autocomplete="new-password" /></template
        ><template v-else-if="modal === 'password'"
          ><label for="old-password">当前密码</label
          ><input
            id="old-password"
            v-model="form.old"
            type="password"
            required
            autocomplete="current-password"
          /><label for="new-password">新密码</label
          ><input
            id="new-password"
            v-model="form.new"
            type="password"
            required
            minlength="12"
            maxlength="72"
            placeholder="至少 12 位"
            autocomplete="new-password"
          />
          <p class="field-hint">修改后会退出该账号的所有登录会话。</p></template
        ><template v-else
          ><label for="item-name">{{
            modal === 'folder'
              ? '文件夹名称'
              : modal === 'text' || modal === 'edit'
                ? '标题'
                : '名称'
          }}</label
          ><input
            id="item-name"
            v-model="form.name"
            :disabled="modal === 'edit'"
            :placeholder="modal === 'folder' ? '给资料一个好找的名字' : '这份分享，关于什么？'"
            required
            maxlength="150"
          /><template v-if="modal === 'text' || modal === 'edit'"
            ><label for="item-content">文字内容</label
            ><textarea
              id="item-content"
              v-model="form.content"
              placeholder="写点什么，或者直接粘贴…"
              rows="8"
              required
              maxlength="10000"
            ></textarea>
            <p class="field-hint">支持纯文字和链接，最多 30,000 字节。</p></template
          ></template
        >
        <p v-if="formError" class="form-error" role="alert">{{ formError }}</p>
        <footer class="dialog-actions">
          <button class="button secondary" type="button" :disabled="busy" @click="modal = null">
            取消</button
          ><button
            class="button"
            :class="modal === 'trash' ? 'danger' : 'primary'"
            :disabled="busy"
          >
            {{
              busy
                ? '正在保存…'
                : modal === 'trash'
                  ? '移入回收站'
                  : modal === 'text'
                    ? '发布到共享空间'
                    : '确认保存'
            }}<AppIcon v-if="modal === 'text'" name="send" :size="16" />
          </button>
        </footer>
      </form>
      <template v-else-if="preview"
        ><div class="preview-meta">
          <span class="avatar tiny">{{ preview.owner.slice(0, 1) }}</span
          >{{ preview.owner }}<span>·</span>{{ dateLabel(preview.created) }}<span>·</span
          >{{ sizeLabel(preview.size) }}
        </div>
        <div v-if="preview.kind === 'image'" class="image-preview">
          <img :src="`/api/files/${preview.id}?preview=1`" :alt="preview.name" />
        </div>
        <pre v-else-if="preview.kind === 'text'" class="text-preview" tabindex="0">{{
          preview.content
        }}</pre>
        <div v-else class="file-preview">
          <FileMark :item="preview" large />
          <h3>{{ preview.name }}</h3>
          <p>下载到电脑后，使用对应的应用打开。</p>
        </div>
        <div class="preview-actions">
          <button class="button secondary" @click="copyLink(preview)">
            <AppIcon name="link" :size="17" />复制链接</button
          ><button
            v-if="preview.kind === 'text'"
            class="button secondary"
            @click="openModal('edit', preview)"
          >
            <AppIcon name="edit" :size="17" />编辑</button
          ><button
            v-if="preview.kind === 'image' && preview.mime === 'image/png'"
            class="button secondary"
            @click="copyImage(preview)"
          >
            <AppIcon name="copy" :size="17" />复制图片</button
          ><button
            class="button primary"
            @click="preview.kind === 'text' ? copy(preview.content) : download(preview)"
          >
            <AppIcon :name="preview.kind === 'text' ? 'copy' : 'download'" :size="17" />{{
              preview.kind === 'text' ? '复制文字' : '下载文件'
            }}
          </button>
        </div></template
      >
    </section>
  </div>
  <div v-if="toast" class="toast" :class="toast.kind" role="status">
    <AppIcon :name="toast.kind === 'error' ? 'warning' : 'success'" :size="19" />{{ toast.message }}
  </div>
</template>

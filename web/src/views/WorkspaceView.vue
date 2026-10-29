<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { api } from '../api/client.js';
import { collaboration, itemImageURL } from '../api/collaboration.js';
import { useWorkspace } from '../composables/useWorkspace.js';
import { compareItems, itemUpdatedAt } from '../utils/item-order.js';
import { sizeLabel, dateLabel } from '../utils/format.js';
import { fileVisualLabel as fileType, fileVisualKind } from '../utils/file-presentation.js';
import { nameClipboardFiles } from '../utils/clipboard-files.js';
import { transferLabel } from '../utils/transfer-feedback.js';
import AppIcon from '../components/AppIcon.vue';
import FileMark from '../components/FileMark.vue';
import PendingButton from '../components/PendingButton.vue';
import UploadProgress from '../components/UploadProgress.vue';
import WorkspaceSkeleton from '../components/WorkspaceSkeleton.vue';
import AccountMenu from '../components/common/AccountMenu.vue';
import BaseDialog from '../components/common/BaseDialog.vue';
import PinsDialog from '../components/common/PinsDialog.vue';
import ItemPreviewDialog from '../components/common/ItemPreviewDialog.vue';
import ShareDialog from '../components/shares/ShareDialog.vue';
import PreferencesDialog from '../components/settings/PreferencesDialog.vue';
import MembersDialog from '../components/settings/MembersDialog.vue';
const props = defineProps({ account: { type: Object, required: true } });
const emit = defineEmits(['logout']);
const toast = ref('');
let toastTimer;
function notify(message) {
  toast.value = message;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (toast.value = ''), 4500);
}
const workspace = useWorkspace(notify);
const {
  items,
  space,
  overview,
  loading,
  loadError,
  connection,
  transfers,
  refresh,
  connect,
  disconnect,
  upload,
  toggleStar,
  retryUpload,
  retryFailed,
  clearCompleted,
  cancelUpload,
  activeTransfers,
  accessDenied,
} = workspace;
const spaces = ref([]),
  pins = ref([]),
  preferences = ref({ shareMode: 'members', shareDays: 7, allowDownload: true, view: 'list' });
const view = ref('files'),
  parent = ref(''),
  query = ref(''),
  sort = ref('updated'),
  layout = ref('list'),
  selection = ref([]),
  detailID = ref('');
const narrow = ref(window.innerWidth <= 760);
const spacePeople = ref([]);
function resized() {
  narrow.value = window.innerWidth <= 760;
}
const mobileNav = ref(false),
  transferOpen = ref(false),
  dialog = ref(null),
  shareItem = ref(null),
  preview = ref(null),
  busy = ref(false),
  error = ref('');
const form = ref({}),
  fileInput = ref(null),
  searchInput = ref(null),
  sharedItems = ref([]),
  sharedPath = ref([]),
  bootstrapError = ref('');
let contextSequence = 0;
let internalCopy = null;
let alive = true;
let dragDepth = 0;
const dragging = ref(false);
const isMac = /Mac|iPhone|iPad/.test(navigator.platform);
const currentSpace = computed(
  () =>
    spaces.value.find((entry) => entry.id === space.value) || {
      id: space.value,
      name: '工作空间',
      kind: 'team',
      role: 'viewer',
    },
);
const roleLabel = computed(
  () =>
    ({ owner: '仅自己可见', manager: '空间管理员', editor: '协作者', viewer: '只读成员' })[
      currentSpace.value.role
    ] || '只读',
);
const writable = computed(
  () => view.value === 'files' && ['owner', 'manager', 'editor'].includes(currentSpace.value.role),
);
const personal = computed(() => spaces.value.find((entry) => entry.kind === 'personal'));
const teams = computed(() => spaces.value.filter((entry) => entry.kind === 'team'));
const source = computed(() => (view.value === 'shared' ? sharedItems.value : items.value));
function trashed(item) {
  let node = item;
  const seen = new Set();
  while (node && !seen.has(node.id)) {
    if (node.deleted) return true;
    seen.add(node.id);
    node = items.value.find((entry) => entry.id === node.parent);
  }
  return false;
}
const activeItems = computed(() => items.value.filter((item) => !trashed(item)));
const folders = computed(() => activeItems.value.filter((item) => item.kind === 'folder'));
const pinned = computed(() =>
  pins.value.map((id) => folders.value.find((item) => item.id === id)).filter(Boolean),
);
const crumbs = computed(() => {
  if (view.value === 'shared') return sharedPath.value;
  const result = [];
  let id = parent.value;
  const seen = new Set();
  while (id && !seen.has(id)) {
    seen.add(id);
    const item = items.value.find((entry) => entry.id === id);
    if (!item) break;
    result.unshift(item);
    id = item.parent;
  }
  return result;
});
const filtered = computed(() =>
  source.value
    .filter((item) => {
      if (view.value === 'trash') {
        if (!item.deleted) return false;
      } else if (view.value !== 'shared' && trashed(item)) return false;
      if (view.value === 'files' && !query.value && item.parent !== parent.value) return false;
      if (view.value === 'starred' && !item.starred) return false;
      if (view.value === 'recent' && itemUpdatedAt(item) < Date.now() - 7 * 86400000) return false;
      const needle = query.value.toLocaleLowerCase();
      return (
        !needle ||
        `${item.name} ${item.content || ''} ${item.owner || ''}`
          .toLocaleLowerCase()
          .includes(needle)
      );
    })
    .sort((a, b) => compareItems(a, b, sort.value)),
);
const selected = computed(() => filtered.value.filter((item) => selection.value.includes(item.id)));
const allSelected = computed(
  () =>
    filtered.value.length > 0 && filtered.value.every((item) => selection.value.includes(item.id)),
);
const detail = computed(() =>
  ['members', 'settings'].includes(view.value)
    ? null
    : source.value.find((item) => item.id === detailID.value) || null,
);
const title = computed(() =>
  view.value === 'files'
    ? crumbs.value.at(-1)?.name ||
      (currentSpace.value.kind === 'personal' ? '我的文件' : '团队空间')
    : {
        recent: '最近更新',
        starred: '已收藏',
        trash: '回收站',
        shared: sharedPath.value.at(-1)?.name || '与我共享',
      }[view.value],
);
const subtitle = computed(() =>
  view.value === 'shared'
    ? '指定给你的资料，仅开放分享范围。'
    : currentSpace.value.kind === 'personal'
      ? '保留思考的过程，个人文件默认仅自己可见。'
      : `${currentSpace.value.name.replace(/团队空间$/, '团队')}共同维护的工作资料。`,
);
const quotaPercent = computed(() =>
  Math.min(100, ((overview.value.used || 0) / (overview.value.quota || 1)) * 100),
);
const renameExtension = computed(() =>
  form.value.item?.kind !== 'folder' ? form.value.item?.name.match(/\.[^.]+$/)?.[0] || '' : '',
);
const navigation = [
  { id: 'shared', label: '与我共享', icon: 'share' },
  { id: 'recent', label: '最近使用', icon: 'clock' },
  { id: 'starred', label: '已收藏', icon: 'star' },
  { id: 'trash', label: '回收站', icon: 'trash' },
];
function resetView() {
  parent.value = '';
  query.value = '';
  selection.value = [];
  detailID.value = '';
  mobileNav.value = false;
}
async function chooseSpace(id) {
  if (busy.value) return;
  contextSequence++;
  resetView();
  view.value = 'files';
  pins.value = [];
  await workspace.changeSpace(id);
  await loadPins();
  await loadPeople();
  openInitialDetail();
}
async function loadPins() {
  const sid = space.value;
  try {
    const result = await collaboration.pins(sid);
    if (alive && sid === space.value) pins.value = result;
  } catch (failure) {
    if (alive && sid === space.value) notify(failure.message);
  }
}
async function loadPeople() {
  const sid = space.value;
  spacePeople.value = [];
  if (currentSpace.value.kind === 'personal') {
    spacePeople.value = [props.account];
    return;
  }
  try {
    const people = await collaboration.spaceMembers(sid);
    if (alive && sid === space.value) spacePeople.value = people;
  } catch {
    /* A membership change is handled by the workspace refresh. */
  }
}
function openInitialDetail() {
  if (window.innerWidth > 1150)
    detailID.value =
      activeItems.value.find((item) => fileVisualKind(item) === 'pdf')?.id ||
      activeItems.value.find((item) => item.kind !== 'folder')?.id ||
      '';
}
function folderPath(folder) {
  const names = [];
  const seen = new Set();
  let node = folder;
  while (node && !seen.has(node.id)) {
    seen.add(node.id);
    names.unshift(node.name);
    node = items.value.find((item) => item.id === node.parent);
  }
  return names.length > 1 ? names.slice(0, -1).join(' / ') : '空间根目录';
}
function inspectRow(event, item) {
  if (event.target.closest('button, input, a')) return;
  detailID.value = item.id;
}
async function navigate(id) {
  contextSequence++;
  resetView();
  view.value = id;
  if (id === 'shared') await loadShared();
}
async function loadShared(folder = '') {
  const turn = ++contextSequence;
  try {
    const result = await collaboration.sharedWithMe(folder);
    if (alive && turn === contextSequence) sharedItems.value = result;
  } catch (failure) {
    if (turn === contextSequence) {
      sharedItems.value = [];
      notify(failure.message);
    }
  }
}
function enter(item) {
  if (item.kind !== 'folder') {
    preview.value = item;
    return;
  }
  selection.value = [];
  query.value = '';
  detailID.value = '';
  if (view.value === 'shared') {
    sharedPath.value.push(item);
    loadShared(item.id);
  } else {
    parent.value = item.id;
    view.value = 'files';
  }
}
function breadcrumb(index) {
  query.value = '';
  selection.value = [];
  if (view.value === 'shared') {
    sharedPath.value = sharedPath.value.slice(0, index + 1);
    loadShared(sharedPath.value.at(-1)?.id || '');
  } else parent.value = index < 0 ? '' : crumbs.value[index].id;
}
function select(item, checked) {
  selection.value = checked
    ? [...new Set([...selection.value, item.id])]
    : selection.value.filter((id) => id !== item.id);
}
function openForm(type, item = null) {
  error.value = '';
  dialog.value = type;
  form.value = {};
  if (type === 'rename') {
    const ext = item.kind !== 'folder' ? item.name.match(/\.[^.]+$/)?.[0] || '' : '';
    form.value = { item, name: ext ? item.name.slice(0, -ext.length) : item.name };
  } else if (type === 'actions') form.value = { item };
  else if (type === 'trash') form.value = { ids: item ? [item.id] : [...selection.value] };
}
async function saveForm() {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    if (dialog.value === 'folder' || dialog.value === 'text') {
      const kind = dialog.value;
      let name = (form.value.name || '').trim();
      if (kind === 'text' && !/\.(txt|md)$/i.test(name)) name += '.txt';
      await api.create({
        space: space.value,
        parent: parent.value,
        name,
        kind,
        content: form.value.content || '',
      });
    } else if (dialog.value === 'rename') {
      let name = (form.value.name || '').trim();
      if (
        renameExtension.value &&
        !name.toLowerCase().endsWith(renameExtension.value.toLowerCase())
      )
        name += renameExtension.value;
      await api.update(form.value.item.id, { action: 'rename', name });
    } else if (dialog.value === 'trash') {
      for (const id of form.value.ids) await api.update(id, { action: 'trash' });
      selection.value = [];
      detailID.value = '';
    } else if (dialog.value === 'space') {
      const result = await collaboration.createSpace(form.value.name);
      spaces.value = await collaboration.spaces();
      dialog.value = null;
      busy.value = false;
      await chooseSpace(result.id);
      notify('团队空间已创建');
      return;
    }
    dialog.value = null;
    await refresh();
    await loadPins();
    notify('已保存');
  } catch (failure) {
    error.value = failure.message;
  } finally {
    busy.value = false;
  }
}
async function restore(item) {
  try {
    await api.update(item.id, { action: 'restore' });
    await refresh();
    await loadPins();
    notify('已恢复。若上级仍在回收站，恢复上级后可见。');
  } catch (failure) {
    notify(failure.message);
  }
}
async function pinFolder(item) {
  try {
    const ids = pins.value.includes(item.id)
      ? pins.value.filter((id) => id !== item.id)
      : [...pins.value, item.id];
    pins.value = await collaboration.savePins(space.value, ids);
    dialog.value = null;
    notify('置顶入口已更新');
  } catch (failure) {
    error.value = failure.message;
  }
}
function share(item) {
  preview.value = null;
  dialog.value = null;
  shareItem.value = item;
}
function download(item) {
  const anchor = document.createElement('a');
  anchor.href = `/api/files/${encodeURIComponent(item.id)}`;
  anchor.download = item.name;
  anchor.click();
}
function copyFiles(event = null) {
  if (!selection.value.length) return;
  const marker = `OFFICE_SHARE:${crypto.randomUUID()}`;
  internalCopy = { marker, ids: [...selection.value], uid: props.account.id };
  if (event?.clipboardData) {
    event.preventDefault();
    event.clipboardData.setData('text/plain', marker);
  } else
    navigator.clipboard?.writeText(marker).catch(() => notify('已复制到应用内，可使用“粘贴副本”'));
  notify(`已复制 ${selection.value.length} 项，到目标目录粘贴副本`);
}
async function pasteFiles() {
  if (!internalCopy || internalCopy.uid !== props.account.id || !writable.value) return;
  busy.value = true;
  try {
    for (const id of internalCopy.ids) await collaboration.copy(id, space.value, parent.value);
    await refresh();
    notify('副本已保存，原文件格式保持不变');
  } catch (failure) {
    notify(failure.message);
  } finally {
    busy.value = false;
  }
}
function editing(target) {
  return !!target.closest('input,textarea,select,[contenteditable="true"]');
}
function copyEvent(event) {
  if (
    document.querySelector('dialog[open]') ||
    editing(event.target) ||
    window.getSelection()?.toString()
  )
    return;
  copyFiles(event);
}
function pasteEvent(event) {
  if (document.querySelector('dialog[open]') || editing(event.target)) return;
  if (event.clipboardData?.files?.length) {
    event.preventDefault();
    acceptFiles(nameClipboardFiles(event.clipboardData.files));
    return;
  }
  const text = event.clipboardData?.getData('text/plain');
  if (text && text === internalCopy?.marker) {
    event.preventDefault();
    pasteFiles();
  } else if (text && writable.value) {
    event.preventDefault();
    openForm('text');
    form.value = {
      name: text.split('\n')[0].slice(0, 40) || '文字便签',
      content: text.slice(0, 30000),
    };
  }
}
function keyEvent(event) {
  if (event.key === 'Escape') {
    mobileNav.value = false;
    detailID.value = '';
    return;
  }
  if (document.querySelector('dialog[open]')) return;
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault();
    searchInput.value?.focus();
  }
  if (
    (event.metaKey || event.ctrlKey) &&
    event.key.toLowerCase() === 'a' &&
    event.target.closest('#fileArea') &&
    !editing(event.target)
  ) {
    event.preventDefault();
    selection.value = filtered.value.map((item) => item.id);
  }
}
function acceptFiles(files) {
  if (!writable.value) {
    notify('请先进入有上传权限的文件目录。');
    return;
  }
  upload([...files], parent.value);
  transferOpen.value = true;
}
function dragEnter(event) {
  if ([...event.dataTransfer.types].includes('Files') && !document.querySelector('dialog[open]')) {
    event.preventDefault();
    dragDepth++;
    dragging.value = true;
  }
}
function dragLeave() {
  dragDepth = Math.max(0, dragDepth - 1);
  if (!dragDepth) dragging.value = false;
}
function drop(event) {
  event.preventDefault();
  dragDepth = 0;
  dragging.value = false;
  if (!document.querySelector('dialog[open]')) acceptFiles(event.dataTransfer.files);
}
async function logout() {
  await api.logout();
  disconnect();
  emit('logout');
}
async function reloadSpaces() {
  spaces.value = await collaboration.spaces();
  await refresh();
  await loadPeople();
}
async function boot() {
  bootstrapError.value = '';
  try {
    const [list, pref] = await Promise.all([collaboration.spaces(), collaboration.preferences()]);
    if (!alive) return;
    spaces.value = list;
    preferences.value = pref;
    layout.value = pref.view;
    const params = new URLSearchParams(location.search);
    const initial =
      list.find((entry) => entry.id === params.get('space')) ||
      list.find((entry) => entry.id === 'team') ||
      list[0];
    space.value = initial.id;
    await connect(props.account);
    await loadPins();
    await loadPeople();
    openInitialDetail();
    const linked = params.get('item');
    if (linked) {
      const item = items.value.find((entry) => entry.id === linked) || (await api.item(linked));
      preview.value = item;
    }
  } catch (failure) {
    if (alive) bootstrapError.value = failure.message;
  }
}
watch(accessDenied, async (denied) => {
  if (!denied) return;
  preview.value = null;
  shareItem.value = null;
  dialog.value = null;
  internalCopy = null;
  try {
    spaces.value = await collaboration.spaces();
    if (!spaces.value.some((entry) => entry.id === space.value) && spaces.value[0])
      await chooseSpace(spaces.value[0].id);
  } catch (failure) {
    notify(failure.message);
  }
});
watch(sharedItems, (list) => {
  if (
    preview.value &&
    view.value === 'shared' &&
    !list.some((item) => item.id === preview.value.id)
  )
    preview.value = null;
});
watch(items, () => {
  if (preview.value && view.value !== 'shared')
    preview.value = activeItems.value.find((item) => item.id === preview.value.id) || null;
  selection.value = selection.value.filter((id) => source.value.some((item) => item.id === id));
  if (view.value === 'shared') loadShared(sharedPath.value.at(-1)?.id || '');
});
onMounted(() => {
  window.addEventListener('resize', resized);
  document.addEventListener('copy', copyEvent);
  document.addEventListener('paste', pasteEvent);
  document.addEventListener('keydown', keyEvent);
  boot();
});
onUnmounted(() => {
  window.removeEventListener('resize', resized);
  alive = false;
  contextSequence++;
  clearTimeout(toastTimer);
  internalCopy = null;
  disconnect();
  document.removeEventListener('copy', copyEvent);
  document.removeEventListener('paste', pasteEvent);
  document.removeEventListener('keydown', keyEvent);
});
</script>

<template>
  <div
    class="app office-app"
    @dragenter="dragEnter"
    @dragleave="dragLeave"
    @dragover.prevent
    @drop="drop"
  >
    <div class="sidebar-scrim" :class="{ open: mobileNav }" @click="mobileNav = false"></div>
    <aside
      class="sidebar"
      :class="{ open: mobileNav }"
      :inert="narrow && !mobileNav"
      aria-label="主导航"
    >
      <div class="brand">
        <div class="brand-mark"><AppIcon name="logo" :size="23" /></div>
        <div>
          <div class="brand-name">同桌</div>
          <div class="brand-sub">OFFICE SHARE</div>
        </div>
      </div>
      <div class="company">
        <span class="avatar">同</span>
        <div>
          <div class="company-title">文件协作工作空间</div>
          <div class="tiny muted company-sub">个人资料与团队协作</div>
        </div>
      </div>
      <div class="nav-label">工作空间</div>
      <nav class="nav-group">
        <button
          v-if="personal"
          class="nav-item"
          :class="{ active: view === 'files' && space === personal.id }"
          @click="chooseSpace(personal.id)"
        >
          <AppIcon name="person" />我的文件</button
        ><button
          v-for="team in teams"
          :key="team.id"
          class="nav-item"
          :class="{ active: view === 'files' && space === team.id }"
          @click="chooseSpace(team.id)"
        >
          <AppIcon name="users" /><span class="nav-name">{{
            teams.length === 1 ? '团队空间' : team.name
          }}</span
          ><span class="count">{{ String(teams.indexOf(team) + 1).padStart(2, '0') }}</span></button
        ><button
          v-for="entry in navigation"
          :key="entry.id"
          class="nav-item"
          :class="{ active: view === entry.id }"
          @click="navigate(entry.id)"
        >
          <AppIcon :name="entry.icon" />{{ entry.label }}
        </button>
      </nav>
      <div class="nav-label">管理</div>
      <nav class="nav-group">
        <button
          class="nav-item"
          :class="{ active: view === 'members' }"
          @click="
            navigate('members');
            mobileNav = false;
          "
        >
          <AppIcon name="users" />成员管理</button
        ><button
          class="nav-item"
          :class="{ active: view === 'settings' }"
          @click="
            navigate('settings');
            mobileNav = false;
          "
        >
          <AppIcon name="settings" />设置
        </button>
      </nav>
      <div class="sidebar-bottom">
        <div class="quota">
          <div class="quota-heading">
            <span>存储用量</span><span>{{ quotaPercent.toFixed(1) }}%</span>
          </div>
          <div class="meter"><span :style="{ width: `${quotaPercent}%` }"></span></div>
          <div class="tiny muted">
            {{ sizeLabel(overview.used || 0) }} / {{ sizeLabel(overview.quota || 0) }}
          </div>
        </div>
      </div>
    </aside>
    <div class="shell">
      <header class="topbar">
        <button
          class="icon-btn mobile-toggle"
          aria-label="打开导航"
          @click="mobileNav = !mobileNav"
        >
          <AppIcon name="list" /></button
        ><label class="search"
          ><AppIcon name="search" :size="18" /><input
            ref="searchInput"
            v-model="query"
            aria-label="搜索当前空间"
            :placeholder="narrow ? '搜索文件' : '搜索当前视图中的文件'"
          /><kbd id="searchKey">{{ isMac ? '⌘ K' : 'Ctrl K' }}</kbd></label
        >
        <div class="top-spacer"></div>
        <button
          class="btn top-transfer"
          aria-label="传输任务"
          @click="transferOpen = !transferOpen"
        >
          <AppIcon name="upload" /><span class="label">传输任务</span
          ><span class="tag">{{ activeTransfers.length }}</span></button
        ><span class="divider"></span
        ><AccountMenu
          :user="account"
          @settings="navigate('settings')"
          @members="navigate('members')"
          @create-space="openForm('space')"
          @logout="logout"
        />
      </header>
      <div class="body-layout" :class="{ 'with-detail': detail }">
        <main class="workspace" aria-label="文件工作区">
          <template v-if="['members', 'settings'].includes(view)">
            <div class="breadcrumbs">
              <AppIcon :name="view === 'settings' ? 'settings' : 'users'" />{{
                view === 'settings' ? '账号与偏好' : '管理 / 成员'
              }}
            </div>
            <div class="page-title">
              <div>
                <h1>{{ view === 'settings' ? '设置' : '成员管理' }}</h1>
                <p>
                  {{
                    view === 'settings'
                      ? '让工作空间符合你的使用习惯。'
                      : '管理企业账号、团队成员与访问权限。'
                  }}
                </p>
              </div>
            </div>
            <PreferencesDialog
              embedded
              v-if="view === 'settings'"
              :preferences="preferences"
              :user="account"
              @close="navigate('files')"
              @saved="
                preferences = $event;
                layout = $event.view;
              "
              @password-changed="
                disconnect();
                emit('logout');
              "
            />
            <MembersDialog
              embedded
              v-if="view === 'members'"
              :user="account"
              :space="currentSpace"
              @close="navigate('files')"
              @changed="reloadSpaces"
            /> </template
          ><template v-else>
            <div class="breadcrumbs">
              <AppIcon name="folder" :size="15" /><button @click="breadcrumb(-1)">
                {{
                  view === 'shared'
                    ? '与我共享'
                    : currentSpace.kind === 'personal'
                      ? '我的文件'
                      : '团队空间'
                }}</button
              ><template v-if="view !== 'shared' && currentSpace.kind === 'team'"
                ><AppIcon name="right" :size="12" /><button @click="breadcrumb(-1)">
                  {{ currentSpace.name }}
                </button></template
              ><template v-for="(crumb, index) in crumbs" :key="crumb.id"
                ><AppIcon name="right" :size="12" /><button @click="breadcrumb(index)">
                  {{ crumb.name }}
                </button></template
              >
            </div>
            <div class="page-title">
              <div>
                <h1>{{ title }}</h1>
                <p>{{ subtitle }}</p>
              </div>
              <div v-if="view === 'files'" class="title-actions">
                <button
                  class="btn outline"
                  aria-label="新建文件夹"
                  :disabled="!writable || busy"
                  @click="openForm('folder')"
                >
                  <AppIcon name="plus" /><span class="secondary-label">新建</span></button
                ><button
                  class="btn primary"
                  :disabled="!writable || busy"
                  @click="fileInput.click()"
                >
                  <AppIcon name="upload" />上传文件
                </button>
              </div>
            </div>
            <div class="space-info">
              <div class="space-people">
                <span class="avatars" v-if="view !== 'shared'"
                  ><span
                    v-for="person in spacePeople.slice(0, 3)"
                    :key="person.id"
                    class="avatar"
                    :title="person.name"
                    >{{ person.name?.slice(0, 1) }}</span
                  ></span
                >
                <span class="tiny muted">{{
                  view === 'shared'
                    ? '指定共享'
                    : `${spacePeople.length || 1} 位${currentSpace.kind === 'team' ? '团队' : ''}成员`
                }}</span>
                <span class="tag blue">{{ view === 'shared' ? '只读访问' : roleLabel }}</span>
              </div>
              <span
                class="space-note"
                :title="connection === 'online' ? '实时同步已连接' : '正在恢复实时连接…'"
                >统一保存，有序协作</span
              >
            </div>
            <p v-if="view === 'files' && !writable" class="readonly-hint">
              当前空间为只读，可预览、下载和整理自己的置顶入口。
            </p>
            <div v-if="bootstrapError || loadError" class="preview-note" role="alert">
              {{ bootstrapError || loadError }}
              <button class="btn outline" @click="bootstrapError ? boot() : refresh()">重试</button>
            </div>
            <input
              ref="fileInput"
              type="file"
              multiple
              class="visually-hidden"
              aria-label="选择上传文件"
              @change="
                acceptFiles($event.target.files);
                $event.target.value = '';
              "
            />
            <template v-if="view === 'files' && !parent && !query"
              ><div class="section-line">
                <h2>置顶文件夹</h2>
                <div class="section-actions">
                  <span class="tiny muted">{{ pinned.length }} 个置顶</span
                  ><button @click="dialog = 'pins'">管理置顶</button>
                </div>
              </div>
              <div v-if="pinned.length" class="folder-grid">
                <div v-for="folder in pinned" :key="folder.id" class="folder-card-wrap">
                  <button class="folder-card" @click="enter(folder)">
                    <AppIcon name="folder" weight="fill" :size="28" />
                    <div>
                      <div class="folder-name" :title="folder.name">{{ folder.name }}</div>
                      <div class="folder-meta">
                        {{ activeItems.filter((item) => item.parent === folder.id).length }} 项资料
                      </div>
                      <div class="folder-pin-label" :title="folderPath(folder)">
                        {{ folderPath(folder) }}
                      </div>
                    </div></button
                  ><button
                    class="icon-btn folder-menu"
                    :aria-label="`管理置顶文件夹 ${folder.name}`"
                    @click="openForm('actions', folder)"
                  >
                    <AppIcon name="more" />
                  </button>
                </div>
              </div>
              <div v-else class="preview-note" style="margin-top: 0">
                将常用文件夹置顶，快速回到工作现场。点击“管理置顶”选择目录。
              </div></template
            >
            <div class="toolbar">
              <div class="tabs">
                <button class="tab active">
                  {{ view === 'files' ? '全部文件' : '当前列表' }}
                  <span class="muted tiny">{{ filtered.length }}</span></button
                ><button v-if="view === 'files'" class="tab" @click="navigate('recent')">
                  最近更新</button
                ><button v-if="view === 'files'" class="tab" @click="navigate('starred')">
                  已收藏
                </button>
              </div>
              <div class="tools">
                <label
                  ><span class="sr-only">排序方式</span
                  ><select v-model="sort">
                    <option value="updated">按更新时间</option>
                    <option value="name">按名称排序</option>
                    <option value="size">按文件大小</option>
                  </select></label
                >
                <div class="view-switch">
                  <button
                    class="icon-btn"
                    :class="{ active: layout === 'list' }"
                    aria-label="列表视图"
                    :aria-pressed="layout === 'list'"
                    @click="layout = 'list'"
                  >
                    <AppIcon name="list" :size="16" /></button
                  ><button
                    class="icon-btn"
                    :class="{ active: layout === 'grid' }"
                    aria-label="网格视图"
                    :aria-pressed="layout === 'grid'"
                    @click="layout = 'grid'"
                  >
                    <AppIcon name="grid" :size="16" />
                  </button>
                </div>
              </div>
            </div>
            <div v-if="selection.length" class="bulk-bar">
              <span>已选择 {{ selection.length }} 项</span
              ><button class="btn" @click="selection = []">取消选择</button
              ><span class="push"></span
              ><button class="btn" :disabled="busy" @click="copyFiles()">复制</button
              ><button
                v-if="view !== 'trash'"
                class="btn"
                :disabled="busy || selected.some((item) => !item.capabilities?.write)"
                @click="openForm('trash')"
              >
                移入回收站
              </button>
            </div>
            <div id="fileArea" tabindex="0" aria-label="文件列表">
              <WorkspaceSkeleton v-if="loading" :layout="layout" /><template
                v-else-if="filtered.length"
                ><table v-if="layout === 'list'" class="file-table">
                  <thead>
                    <tr>
                      <th>
                        <input
                          type="checkbox"
                          :checked="allSelected"
                          aria-label="选择当前列表全部文件"
                          @change="
                            selection = $event.target.checked ? filtered.map((item) => item.id) : []
                          "
                        />
                      </th>
                      <th>文件名称</th>
                      <th>所有者</th>
                      <th>更新时间</th>
                      <th>大小</th>
                      <th><span class="sr-only">更多操作</span></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="item in filtered"
                      :key="item.id"
                      :class="{ selected: selection.includes(item.id) }"
                      @click="inspectRow($event, item)"
                    >
                      <td>
                        <input
                          type="checkbox"
                          :checked="selection.includes(item.id)"
                          :aria-label="`选择 ${item.name}`"
                          @change="select(item, $event.target.checked)"
                        />
                      </td>
                      <td>
                        <div class="file-cell">
                          <FileMark :item="item" />
                          <div class="file-name-wrap">
                            <button class="file-name" :title="item.name" @click="enter(item)">
                              {{ item.name }}
                            </button>
                            <div class="file-sub">
                              {{ fileType(item) }}{{ item.starred ? ' · 已收藏' : '' }} ·
                              {{
                                view === 'shared'
                                  ? '指定共享'
                                  : currentSpace.kind === 'personal'
                                    ? '仅自己'
                                    : '团队共享'
                              }}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td>{{ item.owner }}</td>
                      <td>{{ dateLabel(itemUpdatedAt(item)) }}</td>
                      <td class="table-size">
                        {{ item.kind === 'folder' ? '—' : sizeLabel(item.size) }}
                      </td>
                      <td>
                        <button
                          class="icon-btn row-more"
                          :aria-label="`${item.name} 的更多操作`"
                          @click="openForm('actions', item)"
                        >
                          <AppIcon name="more" />
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
                <div v-else class="file-grid">
                  <article
                    v-for="item in filtered"
                    :key="item.id"
                    class="grid-file"
                    :class="{ selected: selection.includes(item.id) }"
                  >
                    <div class="grid-art" @click="enter(item)">
                      <img
                        v-if="item.kind === 'image'"
                        :src="itemImageURL(item)"
                        alt=""
                        loading="lazy"
                      /><FileMark v-else :item="item" large />
                    </div>
                    <button class="file-name" @click="enter(item)">{{ item.name }}</button>
                    <div class="file-sub">
                      {{ fileType(item) }} ·
                      {{ item.kind === 'folder' ? '文件夹' : sizeLabel(item.size) }}
                    </div>
                    <button
                      class="icon-btn"
                      :aria-label="`${item.name} 的更多操作`"
                      @click="openForm('actions', item)"
                    >
                      <AppIcon name="more" />
                    </button>
                  </article></div
              ></template>
              <div v-else-if="!loadError && !bootstrapError" class="empty">
                <AppIcon :name="query ? 'search' : 'folder'" :size="40" />
                <h3>{{ query ? '没有找到匹配资料' : '这里还没有资料' }}</h3>
                <p>{{ query ? '试试其他文件名。' : '上传文件，或创建一份文字便签。' }}</p>
                <button v-if="writable" class="btn outline" @click="fileInput.click()">
                  上传文件
                </button>
              </div>
            </div>
            <div class="file-footer">
              <span
                >{{ filtered.length }} 项资料 ·
                {{ sizeLabel(filtered.reduce((sum, item) => sum + (item.size || 0), 0)) }}</span
              ><span class="copy-hint"
                ><AppIcon name="upload" />拖入文件，或按
                <kbd>{{ isMac ? '⌘' : 'Ctrl' }} V</kbd> 粘贴</span
              >
            </div>
            <div v-if="writable" style="display: flex; gap: 10px; margin-top: 20px">
              <button class="btn" @click="openForm('text')">
                <AppIcon name="text" :size="17" />新建文字便签</button
              ><button v-if="internalCopy" class="btn outline" :disabled="busy" @click="pasteFiles">
                粘贴副本
              </button>
            </div>
          </template>
        </main>
        <aside v-if="detail" class="inspector" aria-label="文件详情">
          <div class="inspector-head">
            <span>文件详情</span
            ><button class="icon-btn" aria-label="关闭详情" @click="detailID = ''">
              <AppIcon name="close" />
            </button>
          </div>
          <div class="preview-art">
            <img
              v-if="detail.kind === 'image'"
              class="preview-image"
              :src="itemImageURL(detail)"
              :alt="detail.name"
            />
            <div v-else-if="fileVisualKind(detail) === 'pdf'" class="paper document-cover">
              <div class="paper-logo">OFFICE / 同桌</div>
              <div class="paper-shape" aria-hidden="true"></div>
              <div class="paper-title">
                {{ detail.name.replace(/(?:\s*\d{4})?\.pdf$/i, '')
                }}<br v-if="/\d{4}\.pdf$/i.test(detail.name)" /><span
                  v-if="/\d{4}\.pdf$/i.test(detail.name)"
                  >{{ detail.name.match(/\d{4}(?=\.pdf$)/i)?.[0] }}</span
                >
              </div>
              <div class="paper-caption">
                {{ detail.owner }} · {{ dateLabel(itemUpdatedAt(detail)) }}
              </div>
            </div>
            <div v-else class="detail-file-art">
              <FileMark :item="detail" />
              <p class="tiny muted">{{ fileType(detail) }}</p>
            </div>
          </div>
          <h2 class="detail-title">{{ detail.name }}</h2>
          <p class="detail-sub">{{ fileType(detail) }} · {{ sizeLabel(detail.size) }}</p>
          <div class="detail-buttons">
            <button class="btn primary" @click="enter(detail)">
              {{ detail.kind === 'folder' ? '打开目录' : '预览文件' }}</button
            ><button class="btn outline" :disabled="view === 'trash'" @click="share(detail)">
              <AppIcon name="link" />分享
            </button>
          </div>
          <section class="detail-section">
            <h3>基本信息</h3>
            <div class="detail-row">
              <span>所有者</span><strong>{{ detail.owner }}</strong>
            </div>
            <div class="detail-row">
              <span>所在空间</span
              ><strong>{{
                view === 'shared'
                  ? '指定共享'
                  : currentSpace.kind === 'personal'
                    ? '我的文件'
                    : '团队空间'
              }}</strong>
            </div>
            <div class="detail-row">
              <span>更新时间</span><strong>{{ dateLabel(itemUpdatedAt(detail)) }}</strong>
            </div>
            <div class="detail-row">
              <span>访问权限</span
              ><strong>{{ detail.capabilities?.write ? '可编辑' : '只读' }}</strong>
            </div>
          </section>
          <section class="detail-section">
            <h3>访问与协作</h3>
            <div class="detail-access">
              <AppIcon :name="currentSpace.kind === 'personal' ? 'lock' : 'users'" /><span>{{
                view === 'shared'
                  ? '指定成员可访问'
                  : currentSpace.kind === 'personal'
                    ? '仅自己可见'
                    : '团队成员可访问'
              }}</span>
            </div>
            <p class="permission-explain">
              {{
                currentSpace.kind === 'personal'
                  ? '个人空间默认私有；分享仅开放当前资料。'
                  : `空间权限继承自「${currentSpace.name}」。分享链接按单独设置控制访问，不开放其他资料。`
              }}
            </p>
            <button class="btn manage-access" :disabled="view === 'trash'" @click="share(detail)">
              {{ detail.capabilities?.manage ? '管理访问权限' : '查看分享方式'
              }}<AppIcon name="right" />
            </button>
          </section>
          <section class="detail-section">
            <h3>资料操作</h3>
            <div class="detail-operations">
              <button class="btn" @click="toggleStar(detail)">
                <AppIcon name="star" />{{ detail.starred ? '取消收藏' : '收藏' }}
              </button>
              <a
                v-if="detail.kind !== 'folder' && detail.capabilities?.download !== false"
                class="btn"
                :href="`/api/files/${encodeURIComponent(detail.id)}`"
                :download="detail.name"
                ><AppIcon name="download" />下载</a
              >
            </div>
          </section>
          <div class="detail-event">
            <span class="event-dot"></span
            ><span
              >{{ detail.owner }} 更新了这份资料<br />{{ dateLabel(itemUpdatedAt(detail)) }}</span
            >
          </div>
        </aside>
      </div>
    </div>
    <section v-if="transferOpen" class="transfer-panel" aria-label="传输队列">
      <div class="transfer-head">
        <span>传输任务 · {{ transfers.length }}</span
        ><button class="icon-btn" aria-label="收起传输面板" @click="transferOpen = false">
          <AppIcon name="close" />
        </button>
      </div>
      <div class="transfer-body">
        <article v-for="task in transfers" :key="task.id" class="transfer-item">
          <div class="transfer-top">
            <div class="transfer-title">
              <div class="transfer-name">{{ task.name }}</div>
              <div class="transfer-meta">
                {{ spaces.find((entry) => entry.id === task.space)?.name || '原目标空间' }} ·
                {{ sizeLabel(task.file.size) }}
              </div>
              <div class="transfer-meta">{{ transferLabel(task) }}</div>
            </div>
            <button
              v-if="['error', 'cancelled'].includes(task.status)"
              class="btn"
              @click="retryUpload(task)"
            >
              重试</button
            ><button
              v-else-if="['waiting', 'uploading', 'processing'].includes(task.status)"
              class="icon-btn"
              :aria-label="`停止 ${task.name}`"
              @click="cancelUpload(task)"
            >
              <AppIcon name="close" /></button
            ><AppIcon v-else name="check" :size="16" />
          </div>
          <UploadProgress :task="task" />
          <p v-if="task.error" class="dialog-error">{{ task.error }}</p>
        </article>
        <p v-if="!transfers.length" class="empty">暂无传输任务</p>
      </div>
      <div class="transfer-end">
        <button @click="retryFailed">重试失败项</button
        ><button @click="clearCompleted">清除完成</button>
      </div>
    </section>
    <div v-if="toast" class="toast" role="status" aria-live="polite">{{ toast }}</div>
    <div v-if="dragging" class="drop-overlay">
      <AppIcon name="upload" :size="42" />
      <h2>松开文件，上传到当前目录</h2>
    </div>
    <ShareDialog
      v-if="shareItem"
      :item="shareItem"
      :defaults="preferences"
      @close="shareItem = null"
      @saved="refresh"
    />
    <ItemPreviewDialog
      v-if="preview"
      :item="preview"
      @close="preview = null"
      @saved="refresh"
      @share="share"
      @notify="notify"
    />
    <PinsDialog
      v-if="dialog === 'pins'"
      :space="space"
      :folders="folders"
      :pins="pins"
      @saved="pins = $event"
      @close="dialog = null"
    />
    <BaseDialog v-if="dialog === 'actions'" :title="form.item.name" @close="dialog = null"
      ><div class="menu-actions">
        <template v-if="view !== 'trash'"
          ><button
            class="btn"
            @click="
              enter(form.item);
              dialog = null;
            "
          >
            {{ form.item.kind === 'folder' ? '打开文件夹' : '预览文件' }}</button
          ><button
            class="btn"
            @click="
              detailID = form.item.id;
              dialog = null;
            "
          >
            查看详情</button
          ><button
            class="btn"
            @click="
              toggleStar(form.item);
              dialog = null;
            "
          >
            {{ form.item.starred ? '取消收藏' : '添加收藏' }}</button
          ><button
            v-if="form.item.kind === 'folder' && view !== 'shared'"
            class="btn"
            @click="pinFolder(form.item)"
          >
            {{ pins.includes(form.item.id) ? '取消置顶' : '置顶文件夹' }}</button
          ><button class="btn" @click="share(form.item)">分享与权限</button
          ><button v-if="form.item.kind !== 'folder'" class="btn" @click="download(form.item)">
            下载原文件</button
          ><button
            class="btn"
            @click="
              selection = [form.item.id];
              copyFiles();
              dialog = null;
            "
          >
            复制文件副本</button
          ><button
            class="btn"
            :disabled="!form.item.capabilities?.write"
            @click="openForm('rename', form.item)"
          >
            重命名</button
          ><button
            class="btn danger"
            :disabled="!form.item.capabilities?.write"
            @click="openForm('trash', form.item)"
          >
            移入回收站
          </button></template
        ><button
          v-else
          class="btn"
          :disabled="!form.item.capabilities?.write"
          @click="
            restore(form.item);
            dialog = null;
          "
        >
          恢复到原位置
        </button>
      </div>
      <p v-if="error" class="dialog-error" role="alert">{{ error }}</p></BaseDialog
    >
    <BaseDialog
      v-if="['folder', 'text', 'rename', 'trash', 'space'].includes(dialog)"
      :title="
        {
          folder: '新建文件夹',
          text: '新建文字便签',
          rename: '重命名',
          trash: '移入回收站',
          space: '创建团队空间',
        }[dialog]
      "
      :wide="dialog === 'text'"
      :busy="busy"
      @close="dialog = null"
      ><form id="resource-form" @submit.prevent="saveForm">
        <template v-if="dialog === 'trash'"
          ><p>将 {{ form.ids.length }} 项资料移入回收站？</p>
          <p class="dialog-note">目录内容一同隐藏；可从回收站恢复。</p></template
        ><template v-else
          ><label class="form-field"
            >{{ dialog === 'folder' ? '文件夹名称' : dialog === 'space' ? '空间名称' : '文件名称'
            }}<span v-if="dialog === 'rename' && renameExtension" class="filename-input"
              ><input
                v-model="form.name"
                aria-label="文件名称"
                required
                maxlength="150"
                autofocus
              /><span class="filename-extension">{{ renameExtension }}</span></span
            ><input v-else v-model="form.name" required maxlength="150" autofocus /></label
          ><label v-if="dialog === 'text'" class="form-field"
            >正文<textarea v-model="form.content" rows="12" maxlength="30000"></textarea>
          </label>
          <p v-if="dialog === 'rename' && renameExtension" class="dialog-note">
            只修改名称，原始扩展名与文件字节不变。
          </p></template
        >
        <p v-if="error" class="dialog-error" role="alert">{{ error }}</p>
      </form>
      <template #footer
        ><button class="btn outline" :disabled="busy" @click="dialog = null">取消</button
        ><PendingButton class="btn primary" type="submit" form="resource-form" :busy="busy">{{
          dialog === 'trash' ? '移入回收站' : '保存'
        }}</PendingButton></template
      ></BaseDialog
    >
  </div>
</template>

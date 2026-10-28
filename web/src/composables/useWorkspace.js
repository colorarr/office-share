import { computed, ref } from 'vue';
import { api, uploadFile } from '../api/client.js';

export function useWorkspace(
  notify,
  { client = api, sendFile = uploadFile, createEventSource = (url) => new EventSource(url) } = {},
) {
  const user = ref(null);
  const space = ref('team');
  const items = ref([]);
  const overview = ref({ used: 0, quota: 10 * 1024 ** 3, online: 0, members: 0, activity: [] });
  const transfers = ref([]);
  const connection = ref('connecting');
  const loading = ref(false);
  const refreshing = ref(false);
  const loadError = ref('');
  const accessDenied = ref(false);
  const pendingStars = ref(new Set());
  let events;
  let refreshTimer;
  let running = 0;
  let generation = 0;
  let refreshSequence = 0;
  const pending = [];
  const controllers = new Map();

  async function refresh() {
    const currentGeneration = generation;
    const sequence = ++refreshSequence;
    refreshing.value = Boolean(user.value && !loading.value);
    const isCurrent = () =>
      currentGeneration === generation && sequence === refreshSequence && user.value;
    try {
      const [files, stats] = await Promise.all([
        client.items(space.value),
        client.overview(space.value),
      ]);
      if (!isCurrent()) return;
      items.value = files;
      overview.value = stats;
      loadError.value = '';
      accessDenied.value = false;
    } catch (error) {
      if (isCurrent()) {
        loadError.value = error.message;
        if ([403, 404].includes(error.status)) {
          accessDenied.value = true;
          items.value = [];
          overview.value = {};
        }
      }
    } finally {
      if (currentGeneration === generation && sequence === refreshSequence) {
        refreshing.value = false;
        loading.value = false;
      }
    }
  }

  async function connect(account) {
    const currentGeneration = generation;
    user.value = account;
    loading.value = true;
    await refresh();
    if (currentGeneration !== generation || !user.value) return;
    events?.close();
    events = createEventSource('/api/events');
    events.onopen = () => (connection.value = 'online');
    events.onerror = () => (connection.value = 'reconnecting');
    events.addEventListener('change', () => {
      clearTimeout(refreshTimer);
      refreshTimer = setTimeout(refresh, 180);
    });
  }

  function disconnect() {
    generation++;
    events?.close();
    clearTimeout(refreshTimer);
    user.value = null;
    items.value = [];
    pending.splice(0);
    for (const controller of controllers.values()) controller.abort();
    controllers.clear();
    transfers.value = [];
    pendingStars.value.clear();
    loadError.value = '';
    loading.value = false;
    refreshing.value = false;
  }

  async function toggleStar(item) {
    if (!user.value || pendingStars.value.has(item.id)) return false;
    const currentGeneration = generation;
    pendingStars.value.add(item.id);
    try {
      await client.update(item.id, { action: 'star', starred: !item.starred });
      if (currentGeneration !== generation) return false;
      await refresh();
      return true;
    } catch (error) {
      if (currentGeneration === generation) notify(error.message, 'error');
      return false;
    } finally {
      if (currentGeneration === generation) pendingStars.value.delete(item.id);
    }
  }

  function processQueue() {
    while (running < 3 && pending.length) {
      const task = pending.shift();
      void runTask(task);
    }
  }

  async function runTask(task) {
    const taskGeneration = generation;
    const controller = new AbortController();
    controllers.set(task.id, controller);
    running++;
    task.status = 'uploading';
    task.attempts++;
    const attempt = task.attempts;
    try {
      await sendFile(
        task.file,
        task.parent,
        (value) => {
          if (
            taskGeneration !== generation ||
            task.attempts !== attempt ||
            task.status !== 'uploading'
          )
            return;
          task.progress = value;
        },
        {
          signal: controller.signal,
          space: task.space,
          uploadID: task.id,
          onUploaded: () => {
            if (
              taskGeneration !== generation ||
              task.attempts !== attempt ||
              task.status !== 'uploading'
            )
              return;
            task.progress = 100;
            task.status = 'processing';
          },
        },
      );
      if (taskGeneration !== generation) return;
      task.status = 'done';
      task.progress = 100;
      void refresh();
    } catch (error) {
      if (taskGeneration !== generation) return;
      task.status = error.name === 'AbortError' ? 'cancelled' : 'error';
      task.error = error.message;
      if (task.status === 'error') notify(`${task.name}：${error.message}`, 'error');
    } finally {
      controllers.delete(task.id);
      running--;
      processQueue();
    }
  }

  function retryUpload(task) {
    if (
      !user.value ||
      !transfers.value.includes(task) ||
      !['error', 'cancelled'].includes(task.status)
    )
      return false;
    task.error = '';
    task.progress = 0;
    task.status = 'waiting';
    pending.push(task);
    processQueue();
    return true;
  }

  function retryFailed() {
    for (const task of transfers.value.filter((entry) => entry.status === 'error')) {
      retryUpload(task);
    }
  }

  function clearCompleted() {
    transfers.value = transfers.value.filter(
      (task) => !['done', 'cancelled'].includes(task.status),
    );
  }

  function upload(files, parent = '') {
    for (const file of files) {
      if (file.size > 90 * 1024 ** 2) {
        notify(`${file.name} 超过单文件 90 MB 上限`, 'error');
        continue;
      }
      const task = ref({
        id: crypto.randomUUID(),
        file,
        parent,
        space: space.value,
        name: file.name,
        progress: 0,
        status: 'waiting',
        error: '',
        attempts: 0,
      }).value;
      transfers.value.push(task);
      pending.push(task);
    }
    processQueue();
  }

  function cancelUpload(task) {
    if (!transfers.value.includes(task)) return;
    if (task.status === 'waiting') {
      const index = pending.indexOf(task);
      if (index >= 0) pending.splice(index, 1);
      task.status = 'cancelled';
    } else if (['uploading', 'processing'].includes(task.status)) {
      controllers.get(task.id)?.abort();
    }
  }

  async function changeSpace(id) {
    space.value = id;
    items.value = [];
    await refresh();
  }

  const activeTransfers = computed(() =>
    transfers.value.filter((task) => ['waiting', 'uploading', 'processing'].includes(task.status)),
  );

  const failedTransfers = computed(() => transfers.value.filter((task) => task.status === 'error'));

  return {
    user,
    accessDenied,
    space,
    changeSpace,
    cancelUpload,
    items,
    overview,
    transfers,
    connection,
    loading,
    refreshing,
    loadError,
    pendingStars,
    toggleStar,
    activeTransfers,
    failedTransfers,
    refresh,
    connect,
    disconnect,
    upload,
    retryUpload,
    retryFailed,
    clearCompleted,
  };
}

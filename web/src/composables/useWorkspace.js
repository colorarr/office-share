import { computed, ref } from 'vue';
import { api, uploadFile } from '../api/client';

export function useWorkspace(notify) {
  const user = ref(null);
  const items = ref([]);
  const overview = ref({ used: 0, quota: 10 * 1024 ** 3, online: 0, members: 0, activity: [] });
  const transfers = ref([]);
  const connection = ref('connecting');
  const loading = ref(false);
  const loadError = ref('');
  let events;
  let refreshTimer;
  let running = 0;
  let generation = 0;
  const pending = [];

  async function refresh() {
    const currentGeneration = generation;
    try {
      const [files, stats] = await Promise.all([api.items(), api.overview()]);
      if (currentGeneration !== generation || !user.value) return;
      items.value = files;
      overview.value = stats;
      loadError.value = '';
    } catch (error) {
      loadError.value = error.message;
    }
  }

  async function connect(account) {
    user.value = account;
    loading.value = true;
    await refresh();
    loading.value = false;
    events?.close();
    events = new EventSource('/api/events');
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
    transfers.value = [];
  }

  async function process() {
    if (running >= 3 || !pending.length) return;
    const task = pending.shift();
    running++;
    task.status = 'uploading';
    try {
      await uploadFile(task.file, task.parent, (value) => (task.progress = value));
      task.status = 'done';
      task.progress = 100;
      await refresh();
    } catch (error) {
      task.status = 'error';
      task.error = error.message;
      notify(`${task.name}：${error.message}`, 'error');
    } finally {
      running--;
      process();
    }
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
        name: file.name,
        progress: 0,
        status: 'waiting',
      }).value;
      transfers.value.push(task);
      pending.push(task);
    }
    for (let index = 0; index < 3; index++) process();
  }

  const activeTransfers = computed(() =>
    transfers.value.filter((task) => ['waiting', 'uploading'].includes(task.status)),
  );

  return {
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
  };
}

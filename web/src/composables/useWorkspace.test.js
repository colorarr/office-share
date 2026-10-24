import assert from 'node:assert/strict';
import test from 'node:test';
import { setImmediate } from 'node:timers/promises';
import { useWorkspace } from './useWorkspace.js';

async function settle() {
  for (let index = 0; index < 5; index++) await setImmediate();
}
function setup(sendFile) {
  const notices = [];
  const workspace = useWorkspace((...args) => notices.push(args), {
    client: { items: async () => [], overview: async () => ({}) },
    sendFile,
  });
  workspace.user.value = { id: 'member' };
  return { workspace, notices };
}

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  return { promise, resolve, reject };
}

test('only the first connection loads with placeholders; background refresh retains content', async () => {
  const initial = deferred();
  const background = deferred();
  let calls = 0;
  const workspace = useWorkspace(() => {}, {
    client: {
      items: () => (++calls === 1 ? initial.promise : background.promise),
      overview: async () => ({}),
    },
    createEventSource: () => ({ close() {}, addEventListener() {} }),
  });
  const connecting = workspace.connect({ id: 'member' });
  assert.equal(workspace.loading.value, true);
  initial.resolve([{ id: 'initial' }]);
  await connecting;
  assert.equal(workspace.loading.value, false);
  const refreshing = workspace.refresh();
  assert.equal(workspace.loading.value, false);
  assert.equal(workspace.items.value[0].id, 'initial');
  background.resolve([{ id: 'updated' }]);
  await refreshing;
  assert.equal(workspace.items.value[0].id, 'updated');
  workspace.disconnect();
});

test('empty and failed first loads both finish the loading state', async () => {
  for (const failure of [false, true]) {
    const workspace = useWorkspace(() => {}, {
      client: {
        items: async () => {
          if (failure) throw new Error('load failed');
          return [];
        },
        overview: async () => ({}),
      },
      createEventSource: () => ({ close() {}, addEventListener() {} }),
    });
    await workspace.connect({ id: 'member' });
    assert.equal(workspace.loading.value, false);
    assert.equal(workspace.items.value.length, 0);
    assert.equal(workspace.loadError.value, failure ? 'load failed' : '');
    workspace.disconnect();
  }
});

test('older refresh responses cannot replace a newer list or overview', async () => {
  const old = deferred();
  let calls = 0;
  const workspace = useWorkspace(() => {}, {
    client: {
      items: () => (++calls === 1 ? old.promise : Promise.resolve([{ id: 'new' }])),
      overview: async () => ({ used: calls }),
    },
  });
  workspace.user.value = { id: 'member' };
  const previous = workspace.refresh();
  await workspace.refresh();
  old.resolve([{ id: 'old' }]);
  await previous;
  assert.deepEqual(workspace.items.value, [{ id: 'new' }]);
  assert.equal(workspace.overview.value.used, 2);
});

test('stale failures do not replace current success or leak across logout', async () => {
  const old = deferred();
  let calls = 0;
  const workspace = useWorkspace(() => {}, {
    client: {
      items: () => (++calls === 1 ? old.promise : Promise.resolve([])),
      overview: async () => ({}),
    },
  });
  workspace.user.value = { id: 'member' };
  const previous = workspace.refresh();
  await workspace.refresh();
  old.reject(new Error('stale failure'));
  await previous;
  assert.equal(workspace.loadError.value, '');
  const pending = deferred();
  const other = useWorkspace(() => {}, {
    client: { items: () => pending.promise, overview: async () => ({}) },
  });
  other.user.value = { id: 'member' };
  const request = other.refresh();
  other.disconnect();
  pending.reject(new Error('previous account failure'));
  await request;
  assert.equal(other.loadError.value, '');
});

test('disconnect while connecting does not recreate an event stream', async () => {
  const pending = deferred();
  let created = 0;
  const workspace = useWorkspace(() => {}, {
    client: { items: () => pending.promise, overview: async () => ({}) },
    createEventSource: () => {
      created++;
      return { close() {}, addEventListener() {} };
    },
  });
  const connecting = workspace.connect({ id: 'member' });
  workspace.disconnect();
  pending.resolve([]);
  await connecting;
  assert.equal(created, 0);
  assert.equal(workspace.user.value, null);
});

test('favorite requests send an explicit target and suppress repeated clicks', async () => {
  const pending = deferred();
  const calls = [];
  const workspace = useWorkspace(() => {}, {
    client: {
      update: (id, body) => {
        calls.push({ id, body });
        return pending.promise;
      },
      items: async () => [{ id: 'note', starred: true }],
      overview: async () => ({}),
    },
  });
  workspace.user.value = { id: 'member' };
  const item = { id: 'note', starred: false };
  const saving = workspace.toggleStar(item);
  assert.equal(workspace.pendingStars.value.has('note'), true);
  assert.equal(await workspace.toggleStar(item), false);
  assert.deepEqual(calls, [{ id: 'note', body: { action: 'star', starred: true } }]);
  pending.resolve({ ok: true });
  assert.equal(await saving, true);
  assert.equal(workspace.pendingStars.value.size, 0);
  assert.equal(workspace.items.value[0].starred, true);
});

test('favorite failures clear pending state and leave the action retryable', async () => {
  const notices = [];
  let fail = true;
  const workspace = useWorkspace((...args) => notices.push(args), {
    client: {
      update: async (_id, body) => {
        assert.equal(body.starred, false);
        if (fail) throw new Error('temporary failure');
      },
      items: async () => [],
      overview: async () => ({}),
    },
  });
  workspace.user.value = { id: 'member' };
  const item = { id: 'note', starred: true };
  assert.equal(await workspace.toggleStar(item), false);
  assert.equal(workspace.pendingStars.value.size, 0);
  assert.equal(notices.length, 1);
  fail = false;
  assert.equal(await workspace.toggleStar(item), true);
});

test('failed upload retries the original file in the original folder', async () => {
  const calls = [];
  const { workspace } = setup(async (file, parent, progress) => {
    calls.push({ file, parent });
    progress(47);
    if (calls.length === 1) throw new Error('网络连接中断');
  });
  const file = new File(['image'], '粘贴图片_独立名称.png', { type: 'image/png' });
  workspace.upload([file], 'original-folder');
  await settle();
  const task = workspace.transfers.value[0];
  assert.equal(task.status, 'error');
  assert.equal(workspace.failedTransfers.value.length, 1);
  assert.equal(task.attempts, 1);
  assert.equal(workspace.retryUpload(task), true);
  assert.equal(workspace.retryUpload(task), false);
  await settle();
  assert.equal(task.status, 'done');
  assert.equal(task.progress, 100);
  assert.equal(task.error, '');
  assert.equal(task.attempts, 2);
  assert.equal(workspace.transfers.value.length, 1);
  assert.deepEqual(
    calls.map((entry) => entry.parent),
    ['original-folder', 'original-folder'],
  );
  assert.equal(calls[1].file, file);
  assert.equal(calls[1].file.name, calls[0].file.name);
});

test('clearing completed uploads preserves failed files for retry', async () => {
  const { workspace } = setup(async (file) => {
    if (file.name === 'failed.txt') throw new Error('请求失败');
  });
  workspace.upload([new File(['a'], 'done.txt'), new File(['b'], 'failed.txt')]);
  await settle();
  workspace.clearCompleted();
  assert.equal(workspace.transfers.value.length, 1);
  assert.equal(workspace.transfers.value[0].name, 'failed.txt');
  assert.equal(workspace.transfers.value[0].file.name, 'failed.txt');
});

test('retry-all retries failed tasks only and respects the three-upload concurrency limit', async () => {
  let fail = true;
  let active = 0;
  let peak = 0;
  let calls = 0;
  const { workspace } = setup(async (_file, _parent, progress) => {
    calls++;
    active++;
    peak = Math.max(peak, active);
    await setImmediate();
    active--;
    if (fail) throw new Error('服务器暂忙');
    progress(100);
  });
  const files = Array.from({ length: 7 }, (_, i) => new File([String(i)], `${i}.txt`));
  workspace.upload(files);
  await settle();
  assert.equal(workspace.failedTransfers.value.length, 7);
  fail = false;
  workspace.retryFailed();
  workspace.retryFailed();
  await settle();
  assert.equal(calls, 14);
  assert.equal(peak, 3);
  assert.equal(workspace.failedTransfers.value.length, 0);
  assert.ok(workspace.transfers.value.every((task) => task.status === 'done'));
});

test('disconnect aborts old requests, drops queued tasks, and suppresses stale failure notices', async () => {
  let calls = 0;
  const { workspace, notices } = setup((_file, _parent, _progress, { signal }) => {
    calls++;
    return new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('Cancelled', 'AbortError')));
    });
  });
  workspace.upload(Array.from({ length: 5 }, (_, i) => new File(['x'], `${i}.txt`)));
  const oldTask = workspace.transfers.value[0];
  assert.equal(calls, 3);
  workspace.disconnect();
  await settle();
  assert.equal(calls, 3);
  assert.equal(workspace.transfers.value.length, 0);
  assert.equal(notices.length, 0);
  workspace.user.value = { id: 'different-member' };
  assert.equal(workspace.retryUpload(oldTask), false);
});

test('a retry that fails again stays available for another retry', async () => {
  const { workspace } = setup(async () => {
    throw new Error('仍然离线');
  });
  workspace.upload([new File(['x'], 'image.png')]);
  await settle();
  const task = workspace.transfers.value[0];
  workspace.retryUpload(task);
  await settle();
  assert.equal(task.status, 'error');
  assert.equal(task.attempts, 2);
  assert.equal(task.error, '仍然离线');
});

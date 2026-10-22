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

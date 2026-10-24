import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { createServer } from 'vite';

let server;
let WorkspaceSkeleton;
let PendingButton;
let UploadProgress;

before(async () => {
  server = await createServer({
    root: fileURLToPath(new URL('../../', import.meta.url)),
    optimizeDeps: { noDiscovery: true, include: [] },
    server: { middlewareMode: true, hmr: false, ws: false },
    appType: 'custom',
  });
  ({ default: WorkspaceSkeleton } = await server.ssrLoadModule(
    '/src/components/WorkspaceSkeleton.vue',
  ));
  ({ default: PendingButton } = await server.ssrLoadModule('/src/components/PendingButton.vue'));
  ({ default: UploadProgress } = await server.ssrLoadModule('/src/components/UploadProgress.vue'));
});
after(async () => server?.close());

test('list skeleton reuses table geometry and renders six non-interactive rows', async () => {
  const html = await renderToString(createSSRApp(WorkspaceSkeleton));
  assert.match(html, /aria-hidden="true"/);
  assert.doesNotMatch(html, /role="status"/);
  assert.match(html, /class="file-table"/);
  assert.equal((html.match(/<tr\b/g) || []).length, 7);
  assert.equal((html.match(/class="file-thumbnail skeleton-block"/g) || []).length, 6);
  assert.doesNotMatch(html, /<(button|input|a)\b/);
});

test('grid skeleton reuses the real card structure and preview dimensions', async () => {
  const html = await renderToString(createSSRApp(WorkspaceSkeleton, { layout: 'grid' }));
  assert.match(html, /class="items-grid"/);
  assert.equal((html.match(/<article\b/g) || []).length, 6);
  assert.equal((html.match(/class="item-card-preview skeleton-block"/g) || []).length, 6);
  assert.equal((html.match(/class="item-card-details"/g) || []).length, 6);
  assert.doesNotMatch(html, /<table\b|<(button|input|a)\b/);
});

test('pending buttons disable immediately, reserve both labels, and delay the spinner', async () => {
  const html = await renderToString(
    createSSRApp(PendingButton, { busy: true, type: 'submit', pendingLabel: '正在保存…' }),
  );
  assert.match(html, /aria-disabled="true"/);
  assert.doesNotMatch(html, /\sdisabled(?:\s|=|>)/);
  assert.match(html, /type="submit"/);
  assert.match(html, /aria-busy="true"/);
  assert.equal((html.match(/\bpending-button-label\b/g) || []).length, 2);
  assert.match(html, /正在保存/);
  assert.doesNotMatch(html, /class="pending-spinner"/);
});

test('upload progress has real values only during measurable phases', async () => {
  const task = { name: 'report.pdf', status: 'uploading', progress: 42, attempts: 1 };
  const html = await renderToString(createSSRApp(UploadProgress, { task }));
  assert.match(html, /role="progressbar"/);
  assert.match(html, /aria-label="report.pdf上传进度"/);
  assert.match(html, /aria-valuenow="42"/);
  assert.match(html, /scaleX\(0.42\)/);
  for (const state of [
    { status: 'processing', progress: 100 },
    { status: 'uploading', progress: null },
  ]) {
    const pending = await renderToString(
      createSSRApp(UploadProgress, { task: { ...task, ...state } }),
    );
    assert.doesNotMatch(pending, /aria-valuenow=/);
    assert.match(pending, /is-indeterminate/);
  }
  const failed = await renderToString(
    createSSRApp(UploadProgress, { task: { ...task, status: 'error', progress: null } }),
  );
  assert.doesNotMatch(failed, /is-indeterminate/);
  assert.match(failed, /上传失败/);
});

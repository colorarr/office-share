import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createSSRApp } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { createServer } from 'vite';

let server;
let WorkspaceSkeleton;

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
});
after(async () => server?.close());

test('list skeleton reuses table geometry and renders six non-interactive rows', async () => {
  const html = await renderToString(createSSRApp(WorkspaceSkeleton));
  assert.match(html, /role="status"/);
  assert.match(html, /正在加载共享内容/);
  assert.match(html, /aria-hidden="true"/);
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

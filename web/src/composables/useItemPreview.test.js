import assert from 'node:assert/strict';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { setImmediate } from 'node:timers/promises';
import { createSSRApp, effectScope, nextTick, reactive, ref, ssrContextKey } from 'vue';
import { createServer } from 'vite';
import { useItemPreview } from './useItemPreview.js';

test('opened preview follows the current item after a workspace refresh', () => {
  const items = ref([{ id: 'note', name: 'old title', content: 'old text' }]);
  const preview = useItemPreview(items);
  preview.value = items.value[0];
  items.value = [{ id: 'note', name: 'new title', content: 'updated by another member' }];
  assert.equal(preview.value.name, 'new title');
  assert.equal(preview.value.content, 'updated by another member');
  // App.vue copies the current preview into an independent edit form.
  const draft = ref({ id: preview.value.id, content: preview.value.content });
  draft.value.content = 'my unsaved draft';
  items.value = [{ id: 'note', content: 'another remote update' }];
  assert.equal(draft.value.content, 'my unsaved draft');
  assert.equal(preview.value.content, 'another remote update');
  preview.value = null;
  assert.equal(preview.value, null);
});

test('a preview never retains a record missing from the current workspace', () => {
  const items = ref([{ id: 'note', content: 'old text' }]);
  const preview = useItemPreview(items);
  preview.value = items.value[0];
  items.value = [];
  assert.equal(preview.value, null);
});

test('unrelated list refreshes do not reload an unchanged document preview', async () => {
  const server = await createServer({
    root: fileURLToPath(new URL('../../', import.meta.url)),
    optimizeDeps: { noDiscovery: true, include: [] },
    server: { middlewareMode: true, hmr: false, ws: false },
    appType: 'custom',
  });
  const originalFetch = globalThis.fetch;
  const scope = effectScope();
  let requests = 0;
  try {
    const { default: DocumentPreview } = await server.ssrLoadModule(
      '/src/components/DocumentPreview.vue',
    );
    globalThis.fetch = async () => {
      requests++;
      return new Response('hello');
    };
    const props = reactive({
      item: { id: 'file', name: 'guide.txt', kind: 'file', mime: 'text/plain', size: 5 },
    });
    // Run the real component's watcher without a browser or a network connection.
    const app = createSSRApp({});
    app.provide(ssrContextKey, { modules: new Set() });
    scope.run(() =>
      app.runWithContext(() => DocumentPreview.setup(props, { expose() {}, emit() {} })),
    );
    await setImmediate();
    assert.equal(requests, 1);
    props.item = { ...props.item };
    await nextTick();
    await setImmediate();
    assert.equal(requests, 1);
    props.item = { ...props.item, id: 'different-file' };
    await nextTick();
    await setImmediate();
    assert.equal(requests, 2);
  } finally {
    scope.stop();
    globalThis.fetch = originalFetch;
    await server.close();
  }
});

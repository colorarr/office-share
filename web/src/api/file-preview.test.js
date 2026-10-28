import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { readTextPreview } from './file-preview.js';
import { TEXT_PREVIEW_BYTES } from '../utils/text-preview.js';
const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

test('authenticated Range request fetches the document without a download interaction', async () => {
  const body = '点击文档就能看到正文';
  globalThis.fetch = async (url, options) => {
    assert.equal(url, '/api/files/test-id');
    assert.equal(options.headers.Range, `bytes=0-${TEXT_PREVIEW_BYTES - 1}`);
    assert.equal(options.credentials, 'same-origin');
    return new Response(body, { status: 206 });
  };
  const result = await readTextPreview({
    id: 'test-id',
    size: new TextEncoder().encode(body).length,
  });
  assert.equal(result.content, body);
  assert.equal(result.truncated, false);
});
test('empty files are rendered as empty documents', async () => {
  globalThis.fetch = async () => new Response('', { status: 416 });
  assert.equal((await readTextPreview({ id: 'empty', size: 0 })).content, '');
});
test('large documents are bounded even if a proxy ignores Range', async () => {
  globalThis.fetch = async () => new Response('a'.repeat(TEXT_PREVIEW_BYTES + 100));
  const result = await readTextPreview({ id: 'large', size: TEXT_PREVIEW_BYTES + 100 });
  assert.equal(result.content.length, TEXT_PREVIEW_BYTES);
  assert.equal(result.truncated, true);
});
test('truncated UTF-8 characters do not cause incorrect legacy decoding', async () => {
  const content = '中'.repeat(TEXT_PREVIEW_BYTES);
  globalThis.fetch = async () => new Response(content);
  const result = await readTextPreview({ id: 'large', size: TEXT_PREVIEW_BYTES * 3 });
  assert.equal(result.encoding, 'UTF-8');
  assert.equal(result.content, '中'.repeat(Math.floor(TEXT_PREVIEW_BYTES / 3)));
});
test('permission and missing-file errors are readable and retryable', async () => {
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ error: '请先登录' }), { status: 401 });
  await assert.rejects(readTextPreview({ id: 'private', size: 1 }), /请先登录/);
});

test('preview-only recipients use the restricted text endpoint, never original-byte Range', async () => {
  globalThis.fetch = async (url, options) => {
    assert.equal(url, '/api/items/restricted/preview');
    assert.equal(options.headers, undefined);
    return Response.json({ text: '受限预览', truncated: true });
  };
  const result = await readTextPreview({ id: 'restricted', capabilities: { download: false } });
  assert.equal(result.content, '受限预览');
  assert.equal(result.truncated, true);
});

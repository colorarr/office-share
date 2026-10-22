import test from 'node:test';
import assert from 'node:assert/strict';
import { supportsTextPreview, decodePreviewBytes } from './text-preview.js';

test('text documents are previewable by filename or MIME; binary documents stay downloads', () => {
  for (const name of [
    '使用说明.txt',
    'README.MD',
    'data.csv',
    'config.json',
    'app.go',
    'page.html',
  ]) {
    assert.equal(
      supportsTextPreview({ name, kind: 'file', mime: 'application/octet-stream' }),
      true,
    );
  }
  assert.equal(
    supportsTextPreview({ name: 'no-extension', kind: 'file', mime: 'text/plain; charset=utf-8' }),
    true,
  );
  assert.equal(supportsTextPreview({ name: 'report.docx', kind: 'file' }), false);
  assert.equal(supportsTextPreview({ name: 'binary.zip', kind: 'file' }), false);
  assert.equal(supportsTextPreview({ name: 'note.txt', kind: 'folder' }), false);
});
test('UTF-8 Chinese text is preserved, including line breaks', () => {
  const content = '同桌 · 使用说明\n点击就能读取。';
  assert.equal(decodePreviewBytes(new TextEncoder().encode(content)).content, content);
});
test('Windows GBK text is decoded as Chinese', () => {
  assert.equal(decodePreviewBytes(new Uint8Array([0xd6, 0xd0, 0xce, 0xc4])).content, '中文');
});
test('UTF-16 BOM is supported', () => {
  assert.equal(
    decodePreviewBytes(new Uint8Array([0xff, 0xfe, 0x2d, 0x4e, 0x87, 0x65])).content,
    '中文',
  );
});
test('markup is returned as literal text and not executed', () => {
  const content = '<script>alert("test")</script>';
  assert.equal(decodePreviewBytes(new TextEncoder().encode(content)).content, content);
});
test('binary content is rejected even with a text filename', () => {
  assert.throws(() => decodePreviewBytes(new Uint8Array([0, 1, 2, 3])), /二进制数据/);
});

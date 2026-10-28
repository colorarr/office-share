import assert from 'node:assert/strict';
import test from 'node:test';
import { fileVisualKind, fileVisualLabel } from './file-presentation.js';

test('approved file icons recognize real formats without rewriting filenames or MIME', () => {
  for (const [name, expected] of [
    ['品牌.PDF', 'pdf'],
    ['排期.xlsx', 'sheet'],
    ['访谈.md', 'text'],
    ['交付.zip', 'file'],
  ]) {
    const item = Object.freeze({ kind: 'file', name, mime: 'application/octet-stream' });
    assert.equal(fileVisualKind(item), expected);
    assert.equal(item.name, name);
  }
  assert.equal(fileVisualLabel({ kind: 'file', name: '排期.xlsx' }), '电子表格');
  assert.equal(fileVisualKind({ kind: 'folder', name: 'PDF' }), 'folder');
});

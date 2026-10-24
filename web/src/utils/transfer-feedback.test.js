import assert from 'node:assert/strict';
import test from 'node:test';
import { transferLabel } from './transfer-feedback.js';

test('upload feedback distinguishes queue, bytes, confirmation, result and retry', () => {
  assert.equal(transferLabel({ status: 'waiting', attempts: 0 }), '等待上传');
  assert.equal(transferLabel({ status: 'waiting', attempts: 1 }), '等待重新上传');
  assert.equal(transferLabel({ status: 'uploading', progress: null }), '正在传输…');
  assert.equal(transferLabel({ status: 'uploading', progress: 42, attempts: 2 }), '42% · 重新上传');
  assert.match(transferLabel({ status: 'processing', progress: 100 }), /等待保存确认/);
  assert.equal(transferLabel({ status: 'done' }), '上传完成');
  assert.equal(transferLabel({ status: 'error' }), '上传失败');
});

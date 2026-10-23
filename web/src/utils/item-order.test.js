import test from 'node:test';
import assert from 'node:assert/strict';
import { compareItems, itemUpdatedAt } from './item-order.js';

const entries = [
  { id: 'old-edited', name: '旧文档', created: 10, updated: 300, size: 10 },
  { id: 'new-upload', name: '新文件', created: 200, updated: 200, size: 20 },
  { id: 'legacy', name: '历史文件', created: 100, size: 30 },
];

test('recent updates put an edited old item ahead of newly created items', () => {
  assert.deepEqual(
    [...entries].sort(compareItems).map((item) => item.id),
    ['old-edited', 'new-upload', 'legacy'],
  );
});

test('legacy records fall back to creation time without changing creation dates', () => {
  assert.equal(itemUpdatedAt(entries[2]), 100);
  assert.equal(itemUpdatedAt({ created: 100, updated: 0 }), 100);
  assert.equal(entries[0].created, 10);
});

test('explicit size and name sorting remain available', () => {
  assert.equal([...entries].sort((a, b) => compareItems(a, b, 'size'))[0].id, 'legacy');
  const sorted = [...entries].sort((a, b) => compareItems(a, b, 'name'));
  assert.deepEqual(
    sorted.map((item) => item.name),
    entries.map((item) => item.name).sort((a, b) => a.localeCompare(b, 'zh-CN')),
  );
});

test('equal timestamps sort deterministically', () => {
  const a = { id: 'a', created: 10, updated: 20 };
  const b = { id: 'b', created: 10, updated: 20 };
  assert.deepEqual(
    [a, b].sort(compareItems).map((item) => item.id),
    ['b', 'a'],
  );
});

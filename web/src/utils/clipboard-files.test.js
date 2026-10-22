import assert from 'node:assert/strict';
import test from 'node:test';
import { nameClipboardFiles } from './clipboard-files.js';

const date = new Date(2026, 9, 22, 14, 5, 6, 7);

test('clipboard images get timestamped names and retain original bytes and MIME', async () => {
  const original = new File(['clipboard-data'], 'image.png', {
    type: 'image/png',
    lastModified: 123456,
  });
  const [renamed] = nameClipboardFiles([original], date, () => 'a1b2c3d4-1111');
  assert.equal(renamed.name, '粘贴图片_20261022_140506_007_a1b2c3d4.png');
  assert.equal(renamed.type, original.type);
  assert.equal(renamed.lastModified, original.lastModified);
  assert.equal(await renamed.text(), await original.text());
  assert.equal(original.name, 'image.png');
});

test('multiple images pasted in the same millisecond get distinct names', () => {
  let sequence = 0;
  const files = [
    new File(['1'], 'image', { type: 'image/png' }),
    new File(['2'], 'image', { type: 'image/png' }),
  ];
  const renamed = nameClipboardFiles(files, date, () => `${++sequence}abcdefg-1111`);
  assert.notEqual(renamed[0].name, renamed[1].name);
});

test('extension follows image MIME rather than the browser filename', () => {
  const [file] = nameClipboardFiles(
    [new File(['jpeg'], 'image.png', { type: 'image/jpeg' })],
    date,
    () => 'abcdef12',
  );
  assert.match(file.name, /\.jpg$/);
});

test('non-image clipboard files keep their original identity and name', () => {
  const file = new File(['report'], '会议记录.txt', { type: 'text/plain' });
  assert.equal(nameClipboardFiles([file], date)[0], file);
});

test('unknown image formats retain an existing safe extension', () => {
  const file = new File(['custom'], 'image.custom', { type: 'image/x-custom' });
  assert.match(nameClipboardFiles([file], date, () => 'abcdef12')[0].name, /\.custom$/);
});

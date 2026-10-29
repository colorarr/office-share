import { decodePreviewBytes, TEXT_PREVIEW_BYTES } from '../utils/text-preview.js';

export async function readTextPreview(item, signal) {
  if (item.capabilities?.download === false) {
    const response = await fetch(`/api/items/${encodeURIComponent(item.id)}/preview`, {
      credentials: 'same-origin',
      signal,
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || '读取预览失败');
    return { content: data.text || '', encoding: 'UTF-8', truncated: !!data.truncated };
  }

  const response = await fetch(`/api/files/${encodeURIComponent(item.id)}`, {
    credentials: 'same-origin',
    signal,
    headers: { Range: `bytes=0-${TEXT_PREVIEW_BYTES - 1}` },
  });
  // A zero-byte file has no satisfiable byte range.
  if (response.status === 416 && item.size === 0) {
    return { content: '', encoding: 'UTF-8', truncated: false };
  }
  if (!response.ok) {
    let message = '读取文档失败，请重试';
    try {
      message = (await response.json()).error || message;
    } catch {
      /* Proxy response. */
    }
    throw new Error(message);
  }
  const reader = response.body.getReader();
  const chunks = [];
  let length = 0;
  try {
    while (length < TEXT_PREVIEW_BYTES) {
      const { done, value } = await reader.read();
      if (done) break;
      const chunk = value.subarray(0, TEXT_PREVIEW_BYTES - length);
      chunks.push(chunk);
      length += chunk.length;
    }
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.length;
  }
  const truncated = item.size > length;
  // A partial UTF-8 character at the preview boundary should not trigger legacy decoding.
  let decoded;
  if (truncated) {
    for (let trim = 0; trim <= 3 && trim <= bytes.length; trim++) {
      try {
        const content = new TextDecoder('utf-8', { fatal: true }).decode(
          bytes.subarray(0, bytes.length - trim),
        );
        decoded = decodePreviewBytes(new TextEncoder().encode(content));
        break;
      } catch {
        /* Try removing an incomplete trailing character before fallback. */
      }
    }
  }
  return { ...(decoded || decodePreviewBytes(bytes)), truncated };
}

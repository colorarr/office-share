export const TEXT_PREVIEW_BYTES = 256 * 1024;

const textExtensions = new Set([
  'txt',
  'md',
  'markdown',
  'csv',
  'tsv',
  'log',
  'json',
  'jsonl',
  'yaml',
  'yml',
  'xml',
  'html',
  'htm',
  'css',
  'scss',
  'less',
  'js',
  'mjs',
  'cjs',
  'jsx',
  'ts',
  'tsx',
  'vue',
  'go',
  'py',
  'java',
  'sql',
  'sh',
  'bash',
  'zsh',
  'ini',
  'conf',
  'config',
  'toml',
  'env',
  'properties',
  'rs',
  'c',
  'h',
  'cpp',
  'hpp',
  'rb',
  'php',
  'bat',
  'ps1',
  'svg',
]);

export function supportsTextPreview(item) {
  if (item.kind !== 'file') return false;
  const extension = item.name.split('.').pop().toLowerCase();
  const mime = (item.mime || '').split(';')[0];
  return (
    mime.startsWith('text/') ||
    textExtensions.has(extension) ||
    ['application/json', 'application/xml', 'application/javascript'].includes(mime)
  );
}

export function decodePreviewBytes(bytes) {
  let encoding = 'UTF-8';
  let content;
  if (bytes[0] === 0xff && bytes[1] === 0xfe) {
    encoding = 'UTF-16 LE';
    content = new TextDecoder('utf-16le').decode(bytes);
  } else if (bytes[0] === 0xfe && bytes[1] === 0xff) {
    encoding = 'UTF-16 BE';
    content = new TextDecoder('utf-16be').decode(bytes);
  } else {
    try {
      content = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
    } catch {
      encoding = 'GB18030';
      content = new TextDecoder('gb18030').decode(bytes);
    }
  }
  // Treat claimed text files containing binary controls as downloads, never executable markup.
  if (/[\x00-\x08\x0b\x0c\x0e-\x1f]/.test(content)) {
    throw new Error('文件包含二进制数据，请下载原文件查看。');
  }
  return { content, encoding };
}

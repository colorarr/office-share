const imageExtensions = {
  'image/png': 'png',
  'image/jpeg': 'jpg',
  'image/gif': 'gif',
  'image/webp': 'webp',
  'image/avif': 'avif',
  'image/bmp': 'bmp',
  'image/svg+xml': 'svg',
  'image/tiff': 'tiff',
  'image/heic': 'heic',
  'image/heif': 'heif',
  'image/x-icon': 'ico',
};

// Clipboard names such as image.png carry no useful identity. Keep ordinary uploads unchanged.
export function nameClipboardFiles(files, now = new Date(), createId = () => crypto.randomUUID()) {
  const pad = (value, length = 2) => String(value).padStart(length, '0');
  const date = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}`;
  const time = `${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
  const milliseconds = pad(now.getMilliseconds(), 3);

  return Array.from(files, (file) => {
    if (!file.type.startsWith('image/')) return file;
    const originalExtension = file.name.match(/\.([a-z0-9]{1,10})$/i)?.[1].toLowerCase();
    const extension = imageExtensions[file.type] || originalExtension || 'img';
    const suffix = createId().replaceAll('-', '').slice(0, 8);
    const name = `粘贴图片_${date}_${time}_${milliseconds}_${suffix}.${extension}`;
    return new File([file], name, { type: file.type, lastModified: file.lastModified });
  });
}

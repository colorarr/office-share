export function sizeLabel(bytes = 0) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
  return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
}

export function dateLabel(value) {
  const date = new Date(value);
  const today = new Date();
  const time = date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  if (date.toDateString() === today.toDateString()) return `今天 ${time}`;
  return `${date.getMonth() + 1}月${date.getDate()}日 ${time}`;
}

export function fileType(item) {
  if (item.kind === 'folder') return '文件夹';
  if (item.kind === 'text') return '文字便签';
  if (item.kind === 'image') return '图片';
  const extension = item.name.split('.').pop();
  return item.name.includes('.') ? extension.toUpperCase() : '文件';
}

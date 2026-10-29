// Presentation follows the approved HTML; never changes the stored filename or MIME.
export function fileVisualKind(item) {
  if (['folder', 'image', 'text'].includes(item.kind)) return item.kind;
  const extension = item.name?.split('.').pop().toLowerCase();
  if (extension === 'pdf') return 'pdf';
  if (['xls', 'xlsx', 'csv', 'ods'].includes(extension)) return 'sheet';
  if (['txt', 'md', 'json', 'log', 'js', 'ts', 'css', 'html'].includes(extension)) return 'text';
  return 'file';
}
export function fileVisualLabel(item) {
  return {
    folder: '文件夹',
    image: '图片',
    text: '文本资料',
    pdf: 'PDF 文档',
    sheet: '电子表格',
    file: '文件',
  }[fileVisualKind(item)];
}

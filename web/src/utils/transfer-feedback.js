export function transferLabel(task) {
  if (task.status === 'done') return '上传完成';
  if (task.status === 'error') return '上传失败';
  if (task.status === 'waiting') return task.attempts ? '等待重新上传' : '等待上传';
  if (task.status === 'processing') return '已传输，等待保存确认…';
  if (task.progress === null) return '正在传输…';
  return `${task.progress}% · ${task.attempts > 1 ? '重新上传' : '正在传输'}`;
}

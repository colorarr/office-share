export function itemUpdatedAt(item) {
  return item.updated || item.created || 0;
}

export function compareItems(a, b, order = 'updated') {
  if (order === 'name') return a.name.localeCompare(b.name, 'zh-CN');
  if (order === 'size' && a.size !== b.size) return b.size - a.size;
  return (
    itemUpdatedAt(b) - itemUpdatedAt(a) ||
    (b.created || 0) - (a.created || 0) ||
    b.id.localeCompare(a.id)
  );
}

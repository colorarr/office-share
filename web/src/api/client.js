export async function request(path, options = {}) {
  const response = await fetch(`/api${path}`, {
    credentials: 'same-origin',
    ...options,
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...options.headers,
    },
  });
  let data;
  try {
    data = await response.json();
  } catch {
    throw new Error('网络响应异常，请稍后重试');
  }
  if (!response.ok) {
    if (response.status === 401 && path !== '/login') {
      window.dispatchEvent(new Event('session-expired'));
    }
    throw new Error(data.error || '请求失败，请重试');
  }
  return data;
}

export const api = {
  me: () => request('/me'),
  login: (data) => request('/login', { method: 'POST', body: JSON.stringify(data) }),
  logout: () => request('/logout', { method: 'POST' }),
  items: () => request('/items'),
  overview: () => request('/overview'),
  create: (data) => request('/items', { method: 'POST', body: JSON.stringify(data) }),
  update: (id, data) => request(`/items/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  members: () => request('/members'),
  invite: (data) => request('/members', { method: 'POST', body: JSON.stringify(data) }),
  password: (data) => request('/password', { method: 'POST', body: JSON.stringify(data) }),
};

export function uploadFile(file, parent, progress, { signal } = {}) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const abort = () => xhr.abort();
    const cleanup = () => signal?.removeEventListener('abort', abort);
    const succeed = (value) => {
      cleanup();
      resolve(value);
    };
    const fail = (error) => {
      cleanup();
      reject(error);
    };
    xhr.open('POST', `/api/upload?parent=${encodeURIComponent(parent)}`);
    xhr.timeout = 30 * 60 * 1000;
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) progress(Math.round((event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      let data;
      try {
        data = JSON.parse(xhr.responseText);
      } catch {
        fail(new Error('上传响应异常，请点击重试'));
        return;
      }
      if (xhr.status >= 200 && xhr.status < 300) succeed(data);
      else fail(new Error(data.error || '上传失败，请点击重试'));
    };
    xhr.onerror = () => fail(new Error('网络连接中断，请检查网络后重试'));
    xhr.ontimeout = () => fail(new Error('上传超时，请点击重试'));
    xhr.onabort = () => fail(new DOMException('上传已取消', 'AbortError'));
    if (signal?.aborted) {
      fail(new DOMException('上传已取消', 'AbortError'));
      return;
    }
    signal?.addEventListener('abort', abort, { once: true });
    const form = new FormData();
    form.append('file', file);
    try {
      xhr.send(form);
    } catch (error) {
      fail(error);
    }
  });
}

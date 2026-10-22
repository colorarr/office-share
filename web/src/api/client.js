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

export function uploadFile(file, parent, progress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
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
        reject(new Error('上传响应异常，请重试'));
        return;
      }
      if (xhr.status >= 200 && xhr.status < 300) resolve(data);
      else reject(new Error(data.error || '上传失败，请重试'));
    };
    xhr.onerror = () => reject(new Error('网络连接中断，请重新上传'));
    xhr.ontimeout = () => reject(new Error('上传超时，请重新上传'));
    const form = new FormData();
    form.append('file', file);
    xhr.send(form);
  });
}

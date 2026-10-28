import { request } from './client.js';

const body = (value) => JSON.stringify(value);
const id = (value) => encodeURIComponent(value);
export const collaboration = {
  spaces: () => request('/spaces'),
  createSpace: (name) => request('/spaces', { method: 'POST', body: body({ name }) }),
  pins: (space) => request(`/pins?space=${id(space)}`),
  savePins: (space, ids) =>
    request(`/pins?space=${id(space)}`, { method: 'PUT', body: body({ ids }) }),
  preferences: () => request('/preferences'),
  savePreferences: (data) => request('/preferences', { method: 'PUT', body: body(data) }),
  spaceMembers: (space) => request(`/spaces/${id(space)}/members`),
  setMemberRole: (space, user, role) =>
    request(`/spaces/${id(space)}/members/${id(user)}`, {
      method: role ? 'PUT' : 'DELETE',
      ...(role ? { body: body({ role }) } : {}),
    }),
  setDisabled: (user, disabled) =>
    request(`/members/${id(user)}`, { method: 'PATCH', body: body({ disabled }) }),
  shares: (item) => request(`/items/${id(item)}/shares`),
  saveShare: (item, data) =>
    request(`/items/${id(item)}/shares`, { method: 'POST', body: body(data) }),
  revokeShare: (share) => request(`/shares/${id(share)}`, { method: 'DELETE' }),
  shared: (token, item = '') => request(`/shared/${id(token)}${item ? `/items/${id(item)}` : ''}`),
  sharedPreview: (token, item) => request(`/shared/${id(token)}/items/${id(item)}/preview`),
  sharedWithMe: (parent = '') => request(`/shared-with-me${parent ? `?parent=${id(parent)}` : ''}`),
  copy: (item, space, parent) =>
    request(`/items/${id(item)}/copy`, { method: 'POST', body: body({ space, parent }) }),
};

export function sharedFileURL(token, item, preview = false) {
  return `/api/shared/${id(token)}/items/${id(item)}/${preview ? 'preview' : 'content'}`;
}

export function itemImageURL(item) {
  return item.capabilities?.download === false
    ? `/api/items/${id(item.id)}/preview`
    : `/api/files/${id(item.id)}?preview=1`;
}

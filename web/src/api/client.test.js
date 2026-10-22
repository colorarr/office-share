import assert from 'node:assert/strict';
import test, { afterEach } from 'node:test';
import { uploadFile } from './client.js';

const previousXHR = globalThis.XMLHttpRequest;
class FakeXHR {
  static instances = [];
  upload = {};
  open(method, url) {
    this.method = method;
    this.url = url;
  }
  send(form) {
    this.form = form;
    FakeXHR.instances.push(this);
  }
  abort() {
    this.onabort?.();
  }
  respond(status, data) {
    this.status = status;
    this.responseText = typeof data === 'string' ? data : JSON.stringify(data);
    this.onload();
  }
}
afterEach(() => {
  globalThis.XMLHttpRequest = previousXHR;
  FakeXHR.instances = [];
});
function begin(options = {}) {
  globalThis.XMLHttpRequest = FakeXHR;
  const file = new File(['contents'], '截图_123.png', { type: 'image/png' });
  const progress = [];
  const promise = uploadFile(file, 'folder/with space', (value) => progress.push(value), options);
  return { file, promise, progress, xhr: FakeXHR.instances.at(-1) };
}

test('upload sends original filename and folder, reports progress, and parses success', async () => {
  const { file, promise, progress, xhr } = begin();
  assert.equal(xhr.url, '/api/upload?parent=folder%2Fwith%20space');
  assert.equal(xhr.form.get('file').name, file.name);
  xhr.upload.onprogress({ lengthComputable: true, loaded: 1, total: 2 });
  xhr.respond(201, { id: 'uploaded' });
  assert.deepEqual(await promise, { id: 'uploaded' });
  assert.deepEqual(progress, [50]);
});

test('server failure preserves actionable error message', async () => {
  const { promise, xhr } = begin();
  xhr.respond(503, { error: '当前传输较多，请稍后重试' });
  await assert.rejects(promise, /当前传输较多/);
});

test('HTML proxy errors become a retryable upload message', async () => {
  const { promise, xhr } = begin();
  xhr.respond(502, '<html>Bad Gateway</html>');
  await assert.rejects(promise, /上传响应异常，请点击重试/);
});

test('network errors and timeout reject with retry instructions', async () => {
  const network = begin();
  network.xhr.onerror();
  await assert.rejects(network.promise, /检查网络后重试/);
  const timeout = begin();
  timeout.xhr.ontimeout();
  await assert.rejects(timeout.promise, /上传超时，请点击重试/);
});

test('disconnect signal aborts the actual HTTP request', async () => {
  const controller = new AbortController();
  const { promise } = begin({ signal: controller.signal });
  controller.abort();
  await assert.rejects(promise, { name: 'AbortError' });
});

test('already aborted requests never send a file', async () => {
  const controller = new AbortController();
  controller.abort();
  const { promise } = begin({ signal: controller.signal });
  await assert.rejects(promise, { name: 'AbortError' });
  assert.equal(FakeXHR.instances.length, 0);
});

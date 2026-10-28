import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
const source = (relative) => readFileSync(new URL(relative, import.meta.url), 'utf8');
const normalize = (value) => value.replace(/\s+/g, '').replaceAll('"', "'");

test('production imports only approved workspace styles, without legacy cascade or font replacement', () => {
  const main = source('../main.js');
  assert.match(main, /styles\/workspace\.css/);
  assert.doesNotMatch(main, /styles\/main\.css|fontsource/);
});
test('approved prototype CSS remains the exact normalized base stylesheet', () => {
  const html = source('../../../docs/preview/office-share.html');
  const css = source('../styles/workspace.css');
  assert.equal(
    normalize(css.split('/* Real-application')[0]),
    normalize(html.match(/<style>([\s\S]*?)<\/style>/)[1]),
  );
  assert.doesNotMatch(css.split('/* Real-application')[1], /\.sidebar\s+\.nav-group\s*\{/);
});
test('production SVGs preserve every path in the approved icon set', () => {
  const html = source('../../../docs/preview/office-share.html');
  const icon = source('./AppIcon.vue');
  for (const match of html.matchAll(/<symbol id="i-([^"]+)"[^>]*>([\s\S]*?)<\/symbol>/g)) {
    assert.ok(normalize(icon).includes(normalize(match[2])), match[1]);
  }
  assert.doesNotMatch(icon, /v-html/);
});
test('workspace preserves approved layout, custom account menu and editable pin entry', () => {
  const view = source('../views/WorkspaceView.vue');
  for (const hook of [
    'body-layout',
    'space-info',
    'avatars',
    'folder-pin-label',
    'inspector',
    'paper-shape',
    'detail-section',
  ])
    assert.ok(view.includes(hook), hook);
  assert.match(view, /openInitialDetail\(\)/);
  assert.match(view, /:inert="narrow && !mobileNav"/);
  assert.match(view, /管理置顶/);
  assert.doesNotMatch(view, /prototype-strip|仅限企业成员|userSelect/);
  const account = source('./common/AccountMenu.vue');
  assert.doesNotMatch(account, /<select/);
  assert.match(account, /role="menu"/);
});
test('preview download and inline PDF obey the actual server capabilities', () => {
  const preview = source('./common/ItemPreviewDialog.vue');
  assert.match(preview, /v-if="item.capabilities\?\.download !== false"/);
  const document = source('./DocumentPreview.vue');
  assert.match(
    document,
    /fileVisualKind\(item\) === 'pdf' && item.capabilities\?\.download !== false/,
  );
});

test('mobile search uses a compact prompt and does not compete with a flexible spacer', () => {
  const view = source('../views/WorkspaceView.vue');
  const css = source('../styles/workspace.css');
  assert.match(view, /:placeholder="narrow \? '搜索文件' : '搜索当前视图中的文件'"/);
  assert.match(css, /\.office-app \.topbar \.top-spacer\s*\{\s*display: none;/);
});

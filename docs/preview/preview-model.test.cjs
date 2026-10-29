const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const html = fs.readFileSync(__dirname + "/office-share.html", "utf8");
const modelSource = html.match(
  /<script id="preview-model">([\s\S]*?)<\/script>/,
)[1];
const context = vm.createContext({});
vm.runInContext(modelSource, context);
const fresh = () => new context.PreviewModel();

test("HTML scripts compile", () => {
  for (const match of html.matchAll(
    /<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/g,
  )) {
    new vm.Script(match[1]);
  }
});
test("personal space does not leak to other accounts", () => {
  const m = fresh();
  assert.equal(
    m.visible("personal").some((i) => i.id === "draft"),
    true,
  );
  m.uid = "xu";
  assert.equal(
    m.visible("personal").some((i) => i.id === "draft"),
    false,
  );
  assert.equal(
    m.visible("recent").some((i) => i.id === "draft"),
    false,
  );
  assert.equal(m.available(m.items.find((i) => i.id === "draft")), false);
});
test("system admin does not automatically read another personal space", () => {
  const m = fresh();
  assert.equal(m.user.admin, true);
  assert.equal(m.available(m.items.find((i) => i.id === "xu-note")), false);
});
test("team viewer is read-only but owns their personal space", () => {
  const m = fresh();
  m.uid = "xu";
  assert.equal(m.canRead(m.items.find((i) => i.id === "brand")), true);
  assert.throws(() => m.create({ name: "forbidden", space: "team" }));
  assert.throws(() => m.rename("brand", "changed.pdf"));
  assert.throws(() => m.trash(["brand"]));
  assert.equal(
    m.create({ name: "my note", kind: "text", space: "xu" }).space,
    "xu",
  );
});
test("team editor can upload and rename but not manage access", () => {
  const m = fresh();
  m.uid = "chen";
  assert.equal(m.canWrite("team"), true);
  assert.equal(m.canManage(m.items.find((i) => i.id === "brand")), false);
  assert.equal(m.rename("brand", "资料.pdf").name, "资料.pdf");
});
test("disabled account loses access and mutation abilities", () => {
  const m = fresh();
  m.user.active = false;
  assert.equal(m.visible("team").length, 0);
  assert.throws(() => m.create({ name: "draft", space: "lin" }));
});
test("same-name create preserves both without case collisions", () => {
  const m = fresh();
  const a = m.create({ name: "plan.pdf", kind: "pdf", space: "team" });
  const b = m.create({ name: "PLAN.pdf", kind: "pdf", space: "team" });
  assert.equal(a.name, "plan.pdf");
  assert.equal(b.name, "PLAN (2).pdf");
  assert.equal(m.items.filter((i) => i.id === a.id).length, 1);
});
test("invalid path names and cross-space parent are rejected", () => {
  const m = fresh();
  assert.throws(() => m.create({ name: "../bad", space: "team" }));
  assert.throws(() =>
    m.create({ name: "bad", space: "lin", parent: "design" }),
  );
  assert.throws(() =>
    m.create({ name: "bad", space: "team", parent: "brand" }),
  );
});
test("trash hides descendants and restoration brings them back", () => {
  const m = fresh();
  m.trash(["design"]);
  assert.equal(m.visible("team", "design").length, 0);
  assert.equal(m.available(m.items.find((i) => i.id === "logo")), false);
  assert.equal(
    m.visible("trash").some((i) => i.id === "design"),
    true,
  );
  m.trash(["design"], true);
  assert.equal(
    m.visible("team", "design").some((i) => i.id === "logo"),
    true,
  );
});
test("restore conflicts preserve two names", () => {
  const m = fresh();
  m.trash(["brand"]);
  m.create({ name: "品牌视觉规范 2026.pdf", space: "team" });
  m.trash(["brand"], true);
  assert.equal(
    m.items.find((i) => i.id === "brand").name,
    "品牌视觉规范 2026 (2).pdf",
  );
});
test("folder copy clones descendants and does not reuse grants", () => {
  const m = fresh();
  m.items.find((i) => i.id === "logo").grants = { xu: "viewer" };
  const [copy] = m.copy(["design"], "lin", "");
  const child = m.items.find((i) => i.parent === copy.id);
  assert.equal(child.name, "标志使用说明.md");
  assert.equal(child.space, "lin");
  assert.equal(child.owner, "lin");
  assert.equal(child.grants, undefined);
});
test("copying into own descendant uses snapshot without infinite recursion", () => {
  const m = fresh();
  const [copy] = m.copy(["design"], "team", "design");
  assert.equal(m.items.filter((i) => i.parent === copy.id).length, 1);
});
test("selected folder and child are copied only once", () => {
  const m = fresh();
  const copied = m.copy(["design", "logo"], "lin", "");
  assert.equal(copied.length, 1);
});
test("copy validates source rights and destination rights", () => {
  const m = fresh();
  m.uid = "xu";
  assert.throws(() => m.copy(["draft"], "xu", ""));
  assert.throws(() => m.copy(["brand"], "team", ""));
  assert.equal(m.copy(["brand"], "xu", "")[0].owner, "xu");
});
test("stars are per account and never override resource permissions", () => {
  const m = fresh();
  m.toggleStar("draft");
  m.uid = "xu";
  m.toggleStar("draft");
  assert.equal(
    m.visible("starred").some((i) => i.id === "draft"),
    false,
  );
  m.toggleStar("brand");
  assert.equal(
    m.visible("starred").some((i) => i.id === "brand"),
    true,
  );
});
test("explicit personal share exposes one file only", () => {
  const m = fresh();
  m.items.find((i) => i.id === "draft").grants = { xu: "viewer" };
  m.uid = "xu";
  assert.equal(
    m.visible("shared").some((i) => i.id === "draft"),
    true,
  );
  assert.equal(
    m.available(m.items.find((i) => i.id === "personal-folder")),
    false,
  );
  assert.throws(() => m.rename("draft", "rename.md"));
});
test("search stays within visible spaces", () => {
  const m = fresh();
  m.uid = "xu";
  assert.equal(m.visible("recent", "", "草稿").length, 0);
  assert.equal(m.visible("team", "", "标志").length, 1);
});
test("cyclic parent chain is not available", () => {
  const m = fresh();
  m.items.find((i) => i.id === "design").parent = "design";
  assert.equal(m.available(m.items.find((i) => i.id === "logo")), false);
});

test("pins include new nested folders and preserve explicit order", () => {
  const m = fresh();
  const nested = m.create({
    name: "交付目录",
    space: "team",
    parent: "design",
  });
  m.setPins("team", [nested.id, "project", "project", "design"]);
  assert.deepEqual(
    Array.from(m.pinned("team"), (item) => item.id),
    [nested.id, "project", "design"],
  );
  m.rename(nested.id, "正式交付");
  assert.equal(m.pinned("team")[0].name, "正式交付");
});
test("pin removal does not delete or hide the actual folder", () => {
  const m = fresh();
  m.togglePin("design");
  assert.equal(
    m.pinned("team").some((item) => item.id === "design"),
    false,
  );
  assert.equal(
    m.visible("team").some((item) => item.id === "design"),
    true,
  );
});
test("pins are independent per account and read-only members can curate shortcuts", () => {
  const m = fresh();
  m.uid = "xu";
  m.setPins("team", ["project"]);
  assert.deepEqual(
    Array.from(m.pinned("team"), (item) => item.id),
    ["project"],
  );
  m.uid = "lin";
  assert.equal(m.pinned("team").length, 3);
});
test("pins reject foreign private folders and files", () => {
  const m = fresh();
  assert.throws(() => m.setPins("team", ["brand"]));
  assert.throws(() => m.setPins("team", ["personal-folder"]));
  m.uid = "xu";
  assert.throws(() => m.setPins("lin", ["personal-folder"]));
});
test("deleted pinned folders disappear and return on restore", () => {
  const m = fresh();
  m.trash(["design"]);
  assert.equal(
    m.pinned("team").some((i) => i.id === "design"),
    false,
  );
  m.trash(["design"], true);
  assert.equal(
    m.pinned("team").some((i) => i.id === "design"),
    true,
  );
});
test("public links allow guests without exposing the personal file to normal lists", () => {
  const m = fresh();
  const share = m.saveShare("draft", {
    mode: "public",
    days: 7,
    allowDownload: true,
  });
  assert.equal(m.linkAccess(share.token).allowed, true);
  m.uid = "xu";
  assert.equal(
    m.visible("recent").some((i) => i.id === "draft"),
    false,
  );
  assert.equal(m.linkAccess(share.token).canDownload, true);
});
test("specific links require a listed active identity, even for other team members", () => {
  const m = fresh();
  const s = m.saveShare("brand", {
    mode: "specific",
    recipients: ["xu"],
    days: 7,
  });
  assert.equal(m.linkAccess(s.token).allowed, false);
  assert.equal(m.linkAccess(s.token, "xu").allowed, true);
  assert.equal(m.linkAccess(s.token, "chen").allowed, false);
  m.users.find((u) => u.id === "xu").active = false;
  assert.equal(m.linkAccess(s.token, "xu").allowed, false);
});
test("specific personal shares appear only for invited accounts", () => {
  const m = fresh();
  m.saveShare("draft", { mode: "specific", recipients: ["xu"], days: 7 });
  m.uid = "xu";
  assert.equal(
    m.visible("shared").some((i) => i.id === "draft"),
    true,
  );
  m.uid = "chen";
  assert.equal(
    m.visible("shared").some((i) => i.id === "draft"),
    false,
  );
});
test("share expiry, revocation and token recreation are enforced", () => {
  const m = fresh();
  const s = m.saveShare("brand", { mode: "public", days: 1 }, 1000);
  assert.equal(m.linkAccess(s.token, null, null, 1001).allowed, true);
  assert.equal(
    m.linkAccess(s.token, null, null, 1000 + 86400000).allowed,
    false,
  );
  m.revokeShare("brand");
  assert.equal(m.linkAccess(s.token, null, null, 1001).allowed, false);
  const next = m.saveShare("brand", { mode: "public", days: 1 }, 2000);
  assert.notEqual(next.token, s.token);
});
test("changing public to specific invalidates the previous public token", () => {
  const m = fresh();
  const before = m.saveShare("brand", { mode: "public", days: 7 });
  const after = m.saveShare("brand", {
    mode: "specific",
    days: 7,
    recipients: ["xu"],
  });
  assert.notEqual(before.token, after.token);
  assert.equal(m.linkAccess(before.token).allowed, false);
  assert.equal(m.linkAccess(after.token, "xu").allowed, true);
});
test("download toggle is independent of view access", () => {
  const m = fresh();
  const s = m.saveShare("brand", {
    mode: "public",
    days: 0,
    allowDownload: false,
  });
  assert.equal(m.linkAccess(s.token).allowed, true);
  assert.equal(m.linkAccess(s.token).canDownload, false);
});
test("folder links stay inside their subtree and disappear if the parent is trashed", () => {
  const m = fresh();
  const s = m.saveShare("design", { mode: "public", days: 7 });
  assert.equal(m.linkAccess(s.token, null, "logo").allowed, true);
  assert.equal(m.linkAccess(s.token, null, "schedule").allowed, false);
  m.trash(["design"]);
  assert.equal(m.linkAccess(s.token, null, "logo").allowed, false);
});
test("share settings require content-management rights and selected recipients", () => {
  const m = fresh();
  assert.throws(() =>
    m.saveShare("brand", { mode: "specific", days: 7, recipients: [] }),
  );
  m.uid = "chen";
  assert.throws(() => m.saveShare("brand", { mode: "public", days: 7 }));
});
test("renaming keeps file extensions, while folder names stay flexible", () => {
  const m = fresh();
  assert.throws(() => m.rename("brand", "wrong.xlsx"));
  assert.throws(() => m.rename("schedule", "wrong.txt"));
  assert.equal(m.rename("brand", "正式规范.pdf").name, "正式规范.pdf");
  assert.equal(m.rename("design", "design.v2").name, "design.v2");
});
test("copies retain the exact original file object and do not rewrite bytes", () => {
  const m = fresh();
  const original = {
    bytes: Buffer.from([0, 255, 17, 10]),
    type: "application/pdf",
  };
  m.items.find((i) => i.id === "brand").file = original;
  const [copy] = m.copy(["brand"], "lin", "");
  assert.equal(copy.file, original);
  assert.equal(copy.name.endsWith(".pdf"), true);
});
test("embedded binary fixtures have true PDF, XLSX and ZIP signatures", () => {
  const source = html.match(
    /<script id="preview-fixtures">([\s\S]*?)<\/script>/,
  )[1];
  const scope = vm.createContext({});
  vm.runInContext(source, scope);
  const data = scope.PreviewFixtures;
  assert.equal(
    Buffer.from(data.brand.data, "base64").subarray(0, 5).toString(),
    "%PDF-",
  );
  assert.equal(
    Buffer.from(data.guide.data, "base64").subarray(0, 5).toString(),
    "%PDF-",
  );
  assert.equal(
    Buffer.from(data.schedule.data, "base64").readUInt32LE(0),
    0x04034b50,
  );
  assert.equal(
    Buffer.from(data.archive.data, "base64").readUInt32LE(0),
    0x04034b50,
  );
  assert.equal(
    Buffer.from(data.schedule.data, "base64").includes(
      Buffer.from("xl/workbook.xml"),
    ),
    true,
  );
});

test("issuer disable or manager demotion invalidates existing links", () => {
  const m = fresh();
  const share = m.saveShare("brand", { mode: "public", days: 7 });
  m.user.role = "viewer";
  assert.equal(m.linkAccess(share.token).allowed, false);
  m.user.role = "manager";
  m.user.active = false;
  assert.equal(m.linkAccess(share.token).allowed, false);
});

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("V1 declares Today, records, cabinet, product, account, authentication, and administration pages", async () => {
  const pages = JSON.parse(await readFile(new URL("../src/pages.json", import.meta.url), "utf8"));
  assert.deepEqual(pages.pages.map((item) => item.path), [
    "pages/today/index",
    "pages/auth/index",
    "pages/records/index",
    "pages/cabinet/index",
    "pages/product/add",
    "pages/product/detail",
    "pages/me/index",
    "pages/admin/invitations",
  ]);
});

test("client environment example contains no secret-shaped variables", async () => {
  const env = await readFile(new URL("../.env.example", import.meta.url), "utf8");
  assert.match(env, /VITE_API_BASE_URL/);
  assert.doesNotMatch(env, /(SECRET|PASSWORD|PRIVATE_KEY|DATABASE_URL|API_KEY)\s*=/);
});

test("Today uses the deterministic API instead of preview domain data", async () => {
  const page = await readFile(new URL("../src/pages/today/index.vue", import.meta.url), "utf8");
  assert.match(page, /getToday/);
  assert.match(page, /createIntake/);
  assert.match(page, /undoIntake/);
  assert.doesNotMatch(page, /预览任务|不是已实现的服用计划/);
});

test("Phase 3 Add flow separates fake recognition from confirmed product data", async () => {
  const page = await readFile(new URL("../src/pages/product/add.vue", import.meta.url), "utf8");
  const api = await readFile(new URL("../src/services/api.ts", import.meta.url), "utf8");
  assert.match(page, /正面、成分表、有效期/);
  assert.match(page, /开发假识别候选，不代表图片真实内容/);
  assert.match(page, /确认并加入补充柜/);
  assert.match(page, /原始文字证据/);
  assert.match(page, /查看 OCR/);
  assert.match(page, /ingredientAmount>0/);
  assert.match(api, /uploadRecognitionSet/);
  assert.match(api, /confirmRecognitionSet/);
  assert.match(api, /ocrEvidence/);
  assert.match(api, /selectedRoute/);
});

test("authentication UI tells users that demo data is not migrated", async () => {
  const page = await readFile(new URL("../src/pages/auth/index.vue", import.meta.url), "utf8");
  assert.match(page, /演示数据不会迁移/);
  assert.match(page, /24 小时无活动后删除/);
});

test("V1 records, product maintenance, and account deletion paths call real APIs", async () => {
  const records = await readFile(new URL("../src/pages/records/index.vue", import.meta.url), "utf8");
  const detail = await readFile(new URL("../src/pages/product/detail.vue", import.meta.url), "utf8");
  const me = await readFile(new URL("../src/pages/me/index.vue", import.meta.url), "utf8");
  const api = await readFile(new URL("../src/services/api.ts", import.meta.url), "utf8");
  assert.match(records, /listIntakes/);
  assert.match(records, /source:\s*recordDate\.value\s*===\s*to\.value\s*\?\s*"ad_hoc"\s*:\s*"backfill"/);
  assert.match(detail, /updateProduct/);
  assert.match(detail, /addBatch/);
  assert.match(me, /deleteAccount/);
  assert.match(api, /\/account/);
});

test("legacy schedule times are not presented as notification authorization", async () => {
  const add = await readFile(new URL("../src/pages/product/add.vue", import.meta.url), "utf8");
  const detail = await readFile(new URL("../src/pages/product/detail.vue", import.meta.url), "utf8");
  const me = await readFile(new URL("../src/pages/me/index.vue", import.meta.url), "utf8");
  assert.match(add, /计划时点/);
  assert.match(detail, /计划时点/);
  assert.match(me, /不是通知授权/);
  assert.match(me, /站内提醒设置与事件中心尚未开放/);
  assert.doesNotMatch(detail, /应用内提醒时间/);
});

test("semantic layout elements use border-box sizing", async () => {
  const app = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
  for (const element of ["main", "section", "nav"]) {
    assert.match(app, new RegExp(`(?:^|\\n)${element},?`, "m"));
  }
  assert.match(app, /box-sizing:\s*border-box/);
});

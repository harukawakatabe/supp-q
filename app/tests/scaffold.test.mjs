import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("Phase 1 declares Today, authentication, and invitation administration pages", async () => {
  const pages = JSON.parse(await readFile(new URL("../src/pages.json", import.meta.url), "utf8"));
  assert.deepEqual(pages.pages.map((item) => item.path), [
    "pages/today/index",
    "pages/auth/index",
    "pages/admin/invitations",
  ]);
});

test("client environment example contains no secret-shaped variables", async () => {
  const env = await readFile(new URL("../.env.example", import.meta.url), "utf8");
  assert.match(env, /VITE_API_BASE_URL/);
  assert.doesNotMatch(env, /(SECRET|PASSWORD|PRIVATE_KEY|DATABASE_URL|API_KEY)\s*=/);
});

test("preview domain data remains visibly labelled after identity work", async () => {
  const page = await readFile(new URL("../src/pages/today/index.vue", import.meta.url), "utf8");
  assert.match(page, /Phase 1 身份闭环/);
  assert.match(page, /不是已实现的服用计划/);
});

test("authentication UI tells users that demo data is not migrated", async () => {
  const page = await readFile(new URL("../src/pages/auth/index.vue", import.meta.url), "utf8");
  assert.match(page, /演示数据不会迁移/);
  assert.match(page, /24 小时无活动后删除/);
});

test("semantic layout elements use border-box sizing", async () => {
  const app = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
  for (const element of ["main", "section", "nav"]) {
    assert.match(app, new RegExp(`(?:^|\\n)${element},?`, "m"));
  }
  assert.match(app, /box-sizing:\s*border-box/);
});

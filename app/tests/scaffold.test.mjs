import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("Today is the only declared Phase 0 page", async () => {
  const pages = JSON.parse(await readFile(new URL("../src/pages.json", import.meta.url), "utf8"));
  assert.equal(pages.pages.length, 1);
  assert.equal(pages.pages[0].path, "pages/today/index");
});

test("client environment example contains no secret-shaped variables", async () => {
  const env = await readFile(new URL("../.env.example", import.meta.url), "utf8");
  assert.match(env, /VITE_API_BASE_URL/);
  assert.doesNotMatch(env, /(SECRET|PASSWORD|PRIVATE_KEY|DATABASE_URL|API_KEY)\s*=/);
});

test("preview data is visibly labelled", async () => {
  const page = await readFile(new URL("../src/pages/today/index.vue", import.meta.url), "utf8");
  assert.match(page, /Phase 0 预览/);
  assert.match(page, /不是已实现的真实用药计划/);
});

test("semantic layout elements use border-box sizing", async () => {
  const app = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
  for (const element of ["main", "section", "nav"]) {
    assert.match(app, new RegExp(`(?:^|\\n)${element},?`, "m"));
  }
  assert.match(app, /box-sizing:\s*border-box/);
});

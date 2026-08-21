import { expect, test, type Browser, type Page } from "@playwright/test";

const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64");
const mailpitURL = process.env.SUPPQ_MAILPIT_URL ?? "http://127.0.0.1:8025";

async function emailCode(email: string, notBefore: number) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    const mailbox = await fetch(`${mailpitURL}/api/v1/messages`).then(response => response.json()) as {
      messages: Array<{ Created: string; Snippet: string; To: Array<{ Address: string }> }>;
    };
    const message = mailbox.messages.find(item =>
      item.To.some(recipient => recipient.Address === email) && new Date(item.Created).getTime() >= notBefore,
    );
    const code = message?.Snippet.match(/\b\d{6}\b/)?.[0];
    if (code) return code;
    await new Promise(resolve => setTimeout(resolve, 200));
  }
  throw new Error(`Mailpit did not receive a fresh code for ${email}`);
}

async function pageAPI<T>(page: Page, path: string, method: string, data?: object): Promise<T> {
  return page.evaluate(async ({ path, method, data }) => {
    const response = await fetch(`/api/v1${path}`, {
      method,
      credentials: "include",
      headers: data ? { "Content-Type": "application/json" } : undefined,
      body: data ? JSON.stringify(data) : undefined,
    });
    const payload = response.status === 204 ? {} : await response.json();
    if (!response.ok) throw new Error(`${method} ${path} failed: ${response.status} ${JSON.stringify(payload)}`);
    return payload;
  }, { path, method, data }) as Promise<T>;
}

async function settledPage(page: Page) {
  const errors: string[] = [];
  page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
  page.on("pageerror", error => errors.push(error.message));
  await page.goto("/#/pages/today/index");
  await expect(page.getByText("今日计划")).toBeVisible();
  return errors;
}

async function demoIdentity(browser: Browser) {
  const context = await browser.newContext();
  const page = await context.newPage();
  await settledPage(page);
  const identity = await page.evaluate(async () => {
    const session = await fetch("/api/v1/session", { credentials: "include" }).then(response => response.json());
    const products = await fetch("/api/v1/products", { credentials: "include" }).then(response => response.json());
    return { userId: session.actor.userId as string, productIds: (products.items as Array<{ id: string }>).map(item => item.id) };
  });
  return { context, page, identity };
}

test("anonymous demos are isolated and all H5 main tabs render", async ({ browser }) => {
  const first = await demoIdentity(browser);
  const second = await demoIdentity(browser);
  expect(first.identity.userId).not.toBe(second.identity.userId);
  expect(first.identity.productIds).not.toEqual(second.identity.productIds);
  expect(first.identity.productIds).toHaveLength(3);

  await first.page.getByRole("button", { name: /记录/ }).click();
  await expect(first.page.getByText("服用记录", { exact: true }).first()).toBeVisible();
  await first.page.getByRole("button", { name: /补充柜/ }).click();
  await expect(first.page.getByText("补充柜", { exact: true }).first()).toBeVisible();
  await first.page.getByRole("button", { name: /我的/ }).click();
  await expect(first.page.getByText("独立演示访客")).toBeVisible();

  await first.context.close();
  await second.context.close();
});

test("manual product, restock, record, and undo survive page navigation", async ({ page }) => {
  const errors = await settledPage(page);
  await page.getByRole("button", { name: /添加/ }).click();
  await page.getByRole("button", { name: /直接手工填写/ }).click();
  await page.getByRole("textbox").first().fill("E2E 维生素 C");
  await page.getByRole("button", { name: "保存并开始计划" }).click();
  await expect(page.getByText("E2E 维生素 C")).toBeVisible();

  await page.getByRole("button", { name: /补充柜/ }).click();
  await page.getByText("E2E 维生素 C").click();
  await expect(page.getByText(/计划版本 1/)).toBeVisible();
  await page.getByRole("button", { name: "补货" }).click();
  await expect(page.getByText("新增库存批次")).toBeVisible();
  await page.getByRole("spinbutton").first().fill("5");
  await page.getByRole("button", { name: "确认入库" }).click();
  await expect(page.getByText("补货已入库，风险与可用天数已重新计算。")).toBeVisible();

  await page.goto("/#/pages/records/index");
  await page.getByRole("button", { name: "补记" }).click();
  await page.locator('input[type="text"]').last().fill("E2E 补记");
  await page.getByRole("button", { name: "确认补记并扣减库存" }).click();
  await expect(page.getByText("E2E 补记")).toBeVisible();
  await page.getByRole("button", { name: "撤销" }).last().click();
  await expect(page.getByText("已撤销").last()).toBeVisible();
  expect(errors).toEqual([]);
});

test("three-image fake recognition remains candidate-only until human confirmation", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-chromium", "one upload acceptance is enough; responsive layout is covered separately");
  const errors = await settledPage(page);
  await page.getByRole("button", { name: /添加/ }).click();
  await page.getByRole("button", { name: /拍照或上传三张图/ }).click();
  for (const role of ["产品正面", "成分表", "有效期"]) {
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByRole("button").filter({ hasText: role }).click();
    const chooser = await chooserPromise;
    await chooser.setFiles({ name: `${role}.png`, mimeType: "image/png", buffer: png });
  }
  await page.getByRole("button", { name: "上传并开始识别" }).click();
  await expect(page.getByText("开发假识别候选，不代表图片真实内容")).toBeVisible({ timeout: 30_000 });
  await page.getByRole("textbox").first().fill("E2E 人工确认产品");
  await page.getByRole("button", { name: "确认并加入补充柜" }).click();
  await expect(page.getByText("E2E 人工确认产品")).toBeVisible();
  expect(errors).toEqual([]);
});

test("invitation registration and account deletion complete through the H5 boundary", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-chromium", "one destructive identity acceptance is enough");
  await settledPage(page);

  const adminRequestedAt = Date.now() - 1000;
  await pageAPI(page, "/auth/email-code/request", "POST", { email: "admin@suppq.local", invitation: "" });
  const adminCode = await emailCode("admin@suppq.local", adminRequestedAt);
  await pageAPI(page, "/auth/email-code/verify", "POST", { email: "admin@suppq.local", code: adminCode, invitation: "", password: "" });
  const invitation = await pageAPI<{ secret: string }>(page, "/admin/invitations", "POST", {
    kind: "generic_code",
    maxUses: 1,
    expiresAt: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
  });
  await pageAPI(page, "/auth/logout", "POST");

  const email = `e2e-delete-${Date.now()}@example.test`;
  const password = `E2E-safe-${Date.now()}-password`;
  const userRequestedAt = Date.now() - 1000;
  await pageAPI(page, "/auth/email-code/request", "POST", { email, invitation: invitation.secret });
  const userCode = await emailCode(email, userRequestedAt);
  await pageAPI(page, "/auth/email-code/verify", "POST", { email, code: userCode, invitation: invitation.secret, password });

  await page.goto("/#/pages/me/index");
  await expect(page.getByText(email)).toBeVisible();
  await page.getByRole("button", { name: "永久删除账户" }).click();
  await page.getByRole("textbox").fill(email);
  await page.getByRole("button", { name: "确认永久删除" }).click();
  await expect(page.getByText("独立演示空间")).toBeVisible({ timeout: 10_000 });
});

test("mobile and desktop pages have no horizontal overflow", async ({ page }) => {
  await settledPage(page);
  for (const path of ["/pages/today/index", "/pages/records/index", "/pages/cabinet/index", "/pages/me/index"]) {
    await page.goto(`/#${path}`);
    await page.waitForLoadState("networkidle");
    const dimensions = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, content: document.documentElement.scrollWidth }));
    expect(dimensions.content).toBeLessThanOrEqual(dimensions.viewport);
  }
});

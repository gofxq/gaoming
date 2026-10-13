import { expect, test } from "@playwright/test";

test("an unauthenticated dashboard visit requires login instead of showing an empty host list", async ({ page }) => {
  await page.route("**/master/api/v1/**", (route) => route.fulfill({ json: { authenticated: false, available: false, items: [] } }));
  await page.goto("/default");
  await expect(page).toHaveURL(/\/default\/login\?/, { timeout: 3000 });
  await expect(page.getByRole("heading", { name: "登录高明" })).toBeVisible();
});

test("an unauthenticated member-management visit offers the login entry", async ({ page }) => {
  await page.route("**/master/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    await route.fulfill({ json: path.endsWith("/auth/session") ? { authenticated: false } : { items: [] } });
  });
  await page.goto("/default/users");
  await expect(page).toHaveURL(/\/default\/login\?/);
  await expect(page.getByText("安全访问", { exact: true })).toBeVisible();
});

test("failed login preserves username and shows a retryable error", async ({ page }) => {
  await page.route("**/master/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/auth/login")) { await route.fulfill({ status: 401, json: { error: "unauthorized" } }); return; }
    await route.fulfill({ json: { authenticated: false, available: false } });
  });
  await page.goto("/team/login");
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page.getByLabel("密码", { exact: true }).fill("correct-password");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("用户名或密码不正确");
  await expect(page.getByLabel("用户名", { exact: true })).toHaveValue("admin");
  await expect(page.getByRole("button", { name: "登录", exact: true })).toBeEnabled();
});

test("successful login rejects an external return target", async ({ page }) => {
  let loggedIn = false;
  await page.route("**/master/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/auth/login")) loggedIn = true;
    if (path.endsWith("/stream/hosts")) { await route.abort(); return; }
    await route.fulfill({ json: { authenticated: loggedIn, user: loggedIn ? { id: 1, tenant_code: "team", display_name: "Admin", role: "admin", status: "active" } : undefined, items: [] } });
  });
  await page.goto("/team/login?return_to=%2F%2Fevil.example");
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page.getByLabel("密码", { exact: true }).fill("correct-password");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page).toHaveURL("http://127.0.0.1:4173/team");
});

test("initial setup explains password length before submitting", async ({ page }) => {
  await page.route("**/master/api/v1/**", (route) => route.fulfill({ json: { authenticated: false } }));
  await page.goto("/team/setup");
  await page.getByLabel("引导凭据", { exact: true }).fill("s".repeat(32));
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page.getByLabel("显示名称", { exact: true }).fill("Admin");
  await page.getByLabel("密码", { exact: true }).fill("short");
  await page.getByRole("button", { name: "创建并登录" }).click();
  await expect(page.getByRole("alert")).toContainText("12～72 字节");
});

test("a restricted share consumes its fragment credential and hides data after disconnection", async ({ page }) => {
  let exchangeCount = 0;
  await page.route("**/master/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/public/shares/exchange")) { exchangeCount++; await route.fulfill({ json: { fields: ["status"] } }); return; }
    if (path.endsWith("/public/shares/session")) { await route.fulfill({ json: { fields: ["status"], mode: "share", host_count: 1 } }); return; }
    if (path.endsWith("/public/shares/hosts")) { await route.fulfill({ json: { items: [{ host_uid: "granted-host", overall_state: 4 }] } }); return; }
    if (path.endsWith("/public/shares/stream/hosts")) {
      await route.fulfill({ contentType: "text/event-stream", body: 'event: sync\ndata: {"items":[{"host_uid":"granted-host","overall_state":4}]}\n\n' }); return;
    }
    await route.fulfill({ json: { authenticated: false } });
  });
  await page.goto(`/share#token=${"A".repeat(43)}`);
  await expect(page).toHaveURL("http://127.0.0.1:4173/share");
  await expect(page.getByRole("heading", { name: "只读主机视图" })).toBeVisible();
  // This finite response deliberately closes the stream after sync. The page
  // must hide the granted host when access can no longer be revalidated.
  await expect(page.getByRole("alert")).toContainText("已隐藏分享数据");
  await expect(page.getByRole("heading", { name: "granted-host" })).not.toBeVisible();
  expect(exchangeCount).toBe(1);
});

test("an invalidated share has a dedicated state without showing an empty private dashboard", async ({ page }) => {
  await page.route("**/master/api/v1/**", (route) => route.fulfill({ status: 401, json: { error: "unauthorized" } }));
  await page.goto("/share");
  await expect(page.getByRole("alert")).toContainText("分享已过期、被撤销或不可用");
  await expect(page.getByText("当前授权范围内没有可用主机。")).not.toBeVisible();
});

test("share exchange can be retried after a transient network failure", async ({ page }) => {
  let exchanges = 0;
  await page.route("**/master/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/public/shares/exchange")) {
      exchanges++;
      await route.fulfill({ status: exchanges === 1 ? 503 : 200, json: exchanges === 1 ? { error: "temporary outage" } : { fields: ["status"] } }); return;
    }
    if (path.endsWith("/public/shares/session")) { await route.fulfill({ json: { fields: ["status"] } }); return; }
    if (path.endsWith("/public/shares/hosts")) { await route.fulfill({ json: { items: [] } }); return; }
    if (path.endsWith("/public/shares/stream/hosts")) { await route.abort(); return; }
    await route.fulfill({ json: { authenticated: false } });
  });
  await page.goto(`/share#token=${"A".repeat(43)}`);
  await expect(page.getByRole("alert")).toContainText("无法读取分享");
  await page.getByRole("button", { name: "重新验证" }).click();
  // Successful exchange reaches the stream; the synthetic aborted stream then
  // displays a distinct, safe disconnect state rather than the old rejection.
  await expect(page.getByRole("alert")).toContainText("实时连接已中断", { timeout: 3000 });
  expect(exchanges).toBe(2);
});

test("a malformed new share does not reuse a previously authorized share cookie", async ({ page }) => {
  await page.route("**/master/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/public/shares/session")) { await route.fulfill({ json: { fields: ["status"] } }); return; }
    if (path.endsWith("/public/shares/hosts")) { await route.fulfill({ json: { items: [{ host_uid: "previous-share-host" }] } }); return; }
    if (path.endsWith("/public/shares/stream/hosts")) { await route.abort(); return; }
    await route.fulfill({ json: { authenticated: false } });
  });
  await page.goto("/share#token=malformed");
  await expect(page).toHaveURL("http://127.0.0.1:4173/share");
  await expect(page.getByRole("alert")).toContainText("分享已过期、被撤销或不可用", { timeout: 3000 });
});

test("an administrator creates a scoped share and revokes it with CSRF protection", async ({ page }) => {
  const records: { id: string; title: string; host_uids: string[]; fields: string[]; expires_at: string; revoked_at?: string }[] = [];
  await page.route("**/master/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/auth/session")) { await route.fulfill({ json: { authenticated: true, user: { id: 1, tenant_code: "team", display_name: "Admin", role: "admin", status: "active" } } }); return; }
    if (path.endsWith("/auth/csrf")) { await route.fulfill({ json: { token: "csrf" } }); return; }
    if (path.endsWith("/stream/hosts")) { await route.abort(); return; }
    if (path.endsWith("/shares") && route.request().method() === "POST") {
      if (route.request().headers()["x-csrf-token"] !== "csrf") { await route.fulfill({ status: 403, json: { error: "invalid CSRF" } }); return; }
      const request = route.request().postDataJSON();
      const record = { id: "share-1", title: request.title, host_uids: request.host_uids, fields: request.fields, expires_at: new Date(Date.now() + 3600000).toISOString() };
      records.push(record); await route.fulfill({ json: { ...record, token: "A".repeat(43) } }); return;
    }
    if (path.endsWith("/shares/share-1") && route.request().method() === "DELETE") {
      if (route.request().headers()["x-csrf-token"] !== "csrf") { await route.fulfill({ status: 403, json: { error: "invalid CSRF" } }); return; }
      records[0].revoked_at = new Date().toISOString(); await route.fulfill({ status: 204, body: "" }); return;
    }
    await route.fulfill({ json: { items: path.endsWith("/shares") ? records : [{ host_uid: "host-1", hostname: "测试主机" }] } });
  });
  await page.goto("/team/shares");
  await page.getByLabel("分享说明").fill("状态分享");
  await page.getByLabel("测试主机").check();
  await page.getByRole("button", { name: "创建分享", exact: true }).click();
  await expect(page.getByRole("textbox", { name: /一次性显示的分享链接/ })).toHaveValue(/\/share#token=/);
  await page.getByRole("button", { name: "撤销分享", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("分享已撤销");
  await expect(page.getByRole("button", { name: "撤销分享", exact: true })).toBeDisabled();
  await expect(page.getByRole("textbox", { name: /一次性显示的分享链接/ })).not.toBeVisible();
});

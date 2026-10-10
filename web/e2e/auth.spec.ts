import { expect, test } from "@playwright/test";

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

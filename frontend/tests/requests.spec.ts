import { expect, test } from "@playwright/test";

const created = "2026-11-14T10:00:00Z";
const employee = { id: "employee", email: "owner@example.com", full_name: "Мастер", created_at: created, updated_at: created };
const client = { id: "client", telegram_id: 42, telegram_username: "client", name: "Клиент", phone: "+79001112233", created_at: created };
const request = {
  id: "request-1", number: 1, status: "new", service_id: null, service_name: "Диагностика",
  answers: [], message: "", phone: client.phone, note: "", created_at: created, updated_at: created, client,
};

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => { resolve = done; });
  return { promise, resolve };
}

function list(name: string) {
  return {
    items: [{ ...request, service_name: name }], total: 1,
    counts: { new: 1, in_progress: 0, closed: 0, rejected: 0, cancelled: 0 },
  };
}

test.beforeEach(async ({ context, page }) => {
  await context.addCookies([{ name: "session", value: "test-session", url: "http://127.0.0.1:3101" }]);
  await page.route("**/api/backend/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const body = path === "/api/backend/me" ? employee
      : path === "/api/backend/autoservice" ? { id: "garage", name: "Гараж" }
      : path === "/api/backend/autoservice/bot" ? { status: "running", username: "garage_bot", recipients: [] }
      : request;
    await route.fulfill({ json: body });
  });
});

test("saving a note preserves changes typed while the request is pending", async ({ page }) => {
  const started = deferred();
  const release = deferred();
  let saves = 0;
  await page.route("**/api/backend/requests/request-1/note", async (route) => {
    const { note } = route.request().postDataJSON();
    saves++;
    if (saves === 1) {
      started.resolve();
      await release.promise;
    }
    await route.fulfill({ json: { ...request, note } });
  });

  await page.goto("/requests/request-1");
  const note = page.getByRole("textbox", { name: "Заметка мастера" });
  const save = page.getByRole("button", { name: "Сохранить заметку", exact: true });
  await note.fill("Версия А");
  await save.click();
  await started.promise;
  await note.fill("Версия Б");
  release.resolve();
  await expect(save).toBeEnabled();
  await expect(note).toHaveValue("Версия Б");
  await expect(page.getByText("Заметка сохранена", { exact: true })).not.toBeVisible();

  await save.click();
  await expect(page.getByText("Заметка сохранена", { exact: true })).toBeVisible();
  await expect(note).toHaveValue("Версия Б");
  await expect(save).toBeDisabled();
});

for (const oldStatus of [200, 500]) {
  test(`a delayed ${oldStatus} response cannot overwrite the selected requests filter`, async ({ page }) => {
    const started = deferred();
    const release = deferred();
    await page.route("**/api/backend/requests?*", async (route) => {
      const url = new URL(route.request().url());
      if (url.searchParams.get("status") === "new") {
        await route.fulfill({ json: list("Новый результат") });
      } else {
        started.resolve();
        await release.promise;
        await route.fulfill({ status: oldStatus, json: oldStatus === 200 ? list("Старый результат") : { error: { code: "internal" } } });
      }
    });

    await page.goto("/requests");
    await started.promise;
    await page.getByRole("button", { name: /^Новые/ }).click();
    await expect(page.locator("tbody").getByText("Новый результат")).toBeVisible();
    const response = page.waitForResponse((res) => new URL(res.url()).pathname === "/api/backend/requests" && !new URL(res.url()).searchParams.has("status"));
    release.resolve();
    await response;
    await page.waitForLoadState("networkidle");
    await expect(page.locator("tbody").getByText("Новый результат")).toBeVisible();
    await expect(page.getByText("Старый результат")).not.toBeVisible();
    await expect(page.getByText("Внутренняя ошибка сервера", { exact: true })).not.toBeVisible();
  });
}

import { test, expect } from '@playwright/test'

// 运行前需要启动 Server（默认 http://localhost:8080）并准备 admin/admin123 账号。
test.beforeEach(async ({ page }) => {
  await page.goto('/login')
  await page.evaluate(() => localStorage.clear())
})

test('login and navigate main pages', async ({ page }) => {
  await page.goto('/login')

  await page.locator('input[autocomplete="username"]').fill('admin')
  await page.locator('input[type="password"]').fill(process.env.E2E_ADMIN_PASSWORD || 'admin123')
  await page.locator('button[type="submit"]').click()

  await expect(page).toHaveURL(/\/agents$/)
  await expect(page.getByText('探针列表', { exact: false })).toBeVisible()

  await page.goto('/alert/rules')
  await expect(page.getByText('告警规则', { exact: false }).first()).toBeVisible()

  await page.goto('/alert/records')
  await expect(page.getByText('告警记录', { exact: false }).first()).toBeVisible()
})

test('agent detail loads charts when an agent exists', async ({ page }) => {
  await page.locator('input[autocomplete="username"]').fill('admin')
  await page.locator('input[type="password"]').fill(process.env.E2E_ADMIN_PASSWORD || 'admin123')
  await page.locator('button[type="submit"]').click()
  await expect(page).toHaveURL(/\/agents$/)

  const cards = page.locator('[data-testid="agent-card"]')
  if ((await cards.count()) === 0) {
    test.skip(true, 'no registered agents available for detail test')
  }

  await cards.first().click()
  await expect(page).toHaveURL(/\/agents\/.+/)
  await expect(page.getByTestId('metrics-chart')).toHaveCount(4, { timeout: 15_000 })
})

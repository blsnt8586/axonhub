import { expect, test } from '@playwright/test'
import {
  apiBaseURL,
  enableCommercialRegistration,
  injectAuthSession,
  registerCommercialUser,
  signInViaApi,
  uniqueCommercialSlug,
} from './commercial-smoke.utils'

test.describe('personal API key lifecycle', () => {
  test('isolates users and enforces secret, IP, status, rotation, and archive behavior', async ({ page, request }) => {
    test.setTimeout(90_000)
    const pageErrors: string[] = []
    const consoleErrors: string[] = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    page.on('console', (message) => {
      if (message.type() === 'error') consoleErrors.push(message.text())
    })

    const admin = await signInViaApi(request)
    await enableCommercialRegistration(request, admin.token)
    const slug = uniqueCommercialSlug('personal-key')
    const password = 'SmokePass123'
    const firstEmail = `${slug}-first@example.com`
    const secondEmail = `${slug}-second@example.com`
    await registerCommercialUser(request, { email: firstEmail, password, firstName: 'First', lastName: 'Key' })
    await registerCommercialUser(request, { email: secondEmail, password, firstName: 'Second', lastName: 'Key' })
    const first = await signInViaApi(request, { email: firstEmail, password })
    const second = await signInViaApi(request, { email: secondEmail, password })
    const firstWorkspacesResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${first.token}` },
    })
    expect(firstWorkspacesResponse.ok(), await firstWorkspacesResponse.text()).toBeTruthy()
    const firstWorkspaces = await firstWorkspacesResponse.json()
    const firstProjectId = firstWorkspaces.workspaces[0]?.id
    expect(firstProjectId).toBeTruthy()

    const firstKeysResponse = await request.get(
      `${apiBaseURL()}/admin/account/api-keys?projectId=${encodeURIComponent(firstProjectId!)}`,
      { headers: { Authorization: `Bearer ${first.token}` } }
    )
    expect(firstKeysResponse.ok(), await firstKeysResponse.text()).toBeTruthy()
    const firstKeys = await firstKeysResponse.json()
    expect(firstKeys.apiKeys).toHaveLength(1)
    expect(firstKeys.apiKeys[0].type).toBe('personal')
    expect(firstKeys.apiKeys[0].maskedKey).not.toContain('secret')

    const denied = await request.patch(
      `${apiBaseURL()}/admin/account/api-keys?projectId=${encodeURIComponent(firstProjectId!)}&keyId=${encodeURIComponent(firstKeys.apiKeys[0].id)}`,
      {
        headers: { Authorization: `Bearer ${second.token}` },
        data: { name: 'Cross User Mutation' },
      }
    )
    expect(denied.status()).toBe(403)

    await injectAuthSession(page, first)
    await page.goto('/project/api-keys', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('personal-api-keys-page')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByText('Default API Key')).toBeVisible()
    await expect(page.getByText(/ah-\.\.\.[a-f0-9]{4}/i)).toBeVisible()

    await page.getByTestId('create-personal-api-key').click()
    await page.locator('#personal-key-name').fill('Restricted Browser Key')
    await page.locator('#personal-key-ip').fill('203.0.113.10')
    await page.locator('#personal-key-request-limit').fill('5')
    await page.getByRole('button', { name: /^Save$|^保存$/ }).click()
    await page.waitForTimeout(500)
    expect(pageErrors).toEqual([])
    expect(consoleErrors).toEqual([])
    const secretInput = page.getByTestId('personal-api-key-secret')
    await expect(secretInput).toBeVisible({ timeout: 20_000 })
    const firstSecret = await secretInput.inputValue()
    expect(firstSecret).toMatch(/^ah-/)

    const deniedByIP = await request.post(`${apiBaseURL()}/v1/chat/completions`, {
      headers: { Authorization: `Bearer ${firstSecret}` },
      data: { model: 'gpt-not-configured', messages: [{ role: 'user', content: 'hello' }] },
    })
    expect(deniedByIP.status()).toBe(401)

    await page.getByRole('button', { name: /^Done$|^完成$/ }).click()
    await expect(secretInput).toHaveCount(0)
    await expect(page.getByText(firstSecret, { exact: true })).toHaveCount(0)

    const restrictedCard = page.locator('article').filter({ hasText: 'Restricted Browser Key' })
    await restrictedCard.getByRole('button', { name: /^Edit$|^编辑$/ }).click()
    await page.getByLabel(/Status|状态/).click()
    await page.getByRole('option', { name: /Disabled|已禁用/ }).click()
    await page.getByRole('button', { name: /^Save$|^保存$/ }).click()
    await expect(restrictedCard.getByText(/Disabled|已禁用/)).toBeVisible()

    await restrictedCard.getByRole('button', { name: /^Rotate$|^轮换$/ }).click()
    await page.getByRole('button', { name: /^Confirm$|^确认$/ }).click()
    await expect(secretInput).toBeVisible({ timeout: 20_000 })
    const rotatedSecret = await secretInput.inputValue()
    expect(rotatedSecret).toMatch(/^ah-/)
    expect(rotatedSecret).not.toBe(firstSecret)
    await page.getByRole('button', { name: /^Done$|^完成$/ }).click()

    await restrictedCard.getByRole('button', { name: /^Archive$|^归档$/ }).click()
    await page.getByRole('button', { name: /^Confirm$|^确认$/ }).click()
    await expect(page.getByText('Restricted Browser Key')).toHaveCount(0)

    await page.setViewportSize({ width: 390, height: 844 })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('personal-api-keys-page')).toBeVisible({ timeout: 20_000 })
    const layout = await page.evaluate(() => ({ documentWidth: document.documentElement.scrollWidth, viewportWidth: innerWidth }))
    expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewportWidth)
    expect(pageErrors).toEqual([])
    expect(consoleErrors).toEqual([])
  })
})

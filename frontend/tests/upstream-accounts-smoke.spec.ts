import { expect, test } from '@playwright/test'
import {
  graphqlRequest,
  injectAuthSession,
  signInViaApi,
  uniqueCommercialSlug,
} from './commercial-smoke.utils'

test.describe('upstream account pool browser smoke', () => {
  test('covers channel account-pool management and monitoring pages', async ({ page, request }) => {
    test.setTimeout(90_000)

    const slug = uniqueCommercialSlug('account-pool-smoke')
    const channelName = `Smoke upstream channel ${slug}`
    const poolName = `Smoke pool ${slug}`
    const accountName = `Smoke account ${slug}`

    const adminSession = await signInViaApi(request)
    await graphqlRequest<{
      createChannel: { id: string; name: string }
    }>(
      request,
      adminSession.token,
      `
        mutation CreateChannel($input: CreateChannelInput!) {
          createChannel(input: $input) {
            id
            name
          }
        }
      `,
      {
        input: {
          type: 'openai',
          name: channelName,
          baseURL: `https://api.${slug}.example.com/v1`,
          credentials: {
            apiKey: `sk-channel-${slug}`,
            apiKeys: [`sk-channel-${slug}`],
          },
          supportedModels: ['gpt-4o'],
          manualModels: ['gpt-4o'],
          autoSyncSupportedModels: false,
          tags: ['playwright', 'account-pool'],
          defaultTestModel: 'gpt-4o',
          orderingWeight: 0,
          remark: 'Playwright account-pool smoke channel',
        },
      }
    )

    await injectAuthSession(page, adminSession)
    await page.goto('/channels', { waitUntil: 'domcontentloaded' })
    await expect(page.locator('[data-testid="channels-table"]')).toBeVisible({ timeout: 20000 })

    await page.getByTestId('channels-name-filter').fill(channelName)
    await expect(page.locator('[data-testid="channels-table"] tbody tr').filter({ hasText: channelName })).toBeVisible({ timeout: 20000 })

    const channelRow = page.locator('[data-testid="channels-table"] tbody tr').filter({ hasText: channelName }).first()
    await channelRow.getByTestId('upstream-accounts-button').click()

    const dialog = page.getByRole('dialog', { name: /Upstream account pools/i })
    await expect(dialog).toBeVisible({ timeout: 20000 })

    await dialog.getByRole('tab', { name: /^Pools$/i }).click()
    await dialog.getByTestId('upstream-pool-name-input').fill(poolName)
    await dialog.getByTestId('upstream-pool-model-patterns-input').fill('gpt-*')
    const savePoolButton = dialog.getByTestId('upstream-pool-save-button')
    await savePoolButton.scrollIntoViewIfNeeded()
    await expect(savePoolButton).toBeVisible()
    await expect(savePoolButton).toBeEnabled()
    await Promise.all([
      page.waitForResponse((response) => {
        const body = response.request().postData() || ''
        return (
          response.url().includes('/admin/graphql') &&
          (body.includes('CreateUpstreamAccountPool') || body.includes('createUpstreamAccountPool')) &&
          response.status() === 200
        )
      }),
      savePoolButton.evaluate((button) => (button as HTMLButtonElement).click()),
    ])
    await expect(dialog.getByText(poolName)).toBeVisible({ timeout: 20000 })

    await dialog.getByRole('tab', { name: /^Accounts$/i }).click()
    await dialog.getByTestId('upstream-account-name-input').fill(accountName)
    await dialog.getByTestId('upstream-account-pool-select').evaluate((button) => (button as HTMLButtonElement).click())
    await page.getByRole('option', { name: poolName }).click()
    await dialog.getByTestId('upstream-account-credential-input').fill(`sk-account-${slug}`)
    const saveAccountButton = dialog.getByTestId('upstream-account-save-button')
    await saveAccountButton.scrollIntoViewIfNeeded()
    await expect(saveAccountButton).toBeVisible()
    await expect(saveAccountButton).toBeEnabled()
    await Promise.all([
      page.waitForResponse((response) => {
        const body = response.request().postData() || ''
        return (
          response.url().includes('/admin/graphql') &&
          (body.includes('CreateUpstreamAccount') || body.includes('createUpstreamAccount')) &&
          response.status() === 200
        )
      }),
      saveAccountButton.evaluate((button) => (button as HTMLButtonElement).click()),
    ])
    await expect(dialog.getByText(accountName)).toBeVisible({ timeout: 20000 })
    await expect(dialog.getByText(/Credential fields are write-only/i)).toBeVisible()

    await page.keyboard.press('Escape')
    await expect(dialog).not.toBeVisible({ timeout: 10000 })

    await page.goto('/channels/accounts', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: /Upstream accounts/i })).toBeVisible({ timeout: 20000 })
    const monitoringRow = page.locator('tbody tr').filter({ hasText: accountName }).first()
    await expect(monitoringRow).toBeVisible({ timeout: 20000 })
    await expect(monitoringRow).toContainText(channelName)
    await expect(monitoringRow).toContainText(/active/i)

    await monitoringRow.locator('a[href*="/channels/accounts/"]').click()
    await expect(page.getByRole('heading', { name: accountName })).toBeVisible({ timeout: 20000 })
    await expect(page.getByText(/Quota and cooldown/i)).toBeVisible()
    await expect(page.getByText(/Recent executions/i)).toBeVisible()
    await expect(page.getByText('Switch history', { exact: true })).toBeVisible()
  })
})

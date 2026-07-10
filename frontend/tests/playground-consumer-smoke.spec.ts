import { expect, test } from '@playwright/test'
import {
  apiBaseURL,
  enableCommercialRegistration,
  graphqlRequest,
  injectAuthSession,
  registerCommercialUser,
  signInViaApi,
  uniqueCommercialSlug,
} from './commercial-smoke.utils'

test.describe('consumer playground', () => {
  test('shows deterministic empty state and sends through the orchestrator with a personal key', async ({ page, request }) => {
    test.setTimeout(90_000)
    const pageErrors: string[] = []
    const consoleErrors: string[] = []
    const pageGraphQLRequests: string[] = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    page.on('console', (message) => {
      if (message.type() === 'error') consoleErrors.push(message.text())
    })
    page.on('request', (outbound) => {
      if (new URL(outbound.url()).pathname === '/admin/graphql') pageGraphQLRequests.push(outbound.postData() || '')
    })

    const admin = await signInViaApi(request)
    await enableCommercialRegistration(request, admin.token)
    const slug = uniqueCommercialSlug('consumer-playground')
    const email = `${slug}@example.com`
    const password = 'SmokePass123'
    await registerCommercialUser(request, { email, password, firstName: 'Playground', lastName: 'User' })
    const user = await signInViaApi(request, { email, password })
    const workspacesResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${user.token}` },
    })
    expect(workspacesResponse.ok(), await workspacesResponse.text()).toBeTruthy()
    const workspaces = await workspacesResponse.json()
    const projectId = workspaces.workspaces[0]?.id as string
    expect(projectId).toBeTruthy()

    const emptyStateResponse = await request.get(
      `${apiBaseURL()}/admin/account/playground?projectId=${encodeURIComponent(projectId)}`,
      { headers: { Authorization: `Bearer ${user.token}` } }
    )
    expect(emptyStateResponse.ok(), await emptyStateResponse.text()).toBeTruthy()
    const emptyState = await emptyStateResponse.json()
    expect(emptyState.blockReason).toBe('no_model')
    expect(emptyState.apiKeys).toHaveLength(1)
    expect(emptyState.apiKeys[0].name).toBe('Default API Key')
    expect(JSON.stringify(emptyState).toLowerCase()).not.toContain('channel')

    await injectAuthSession(page, user)
    await page.goto('/project/playground', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('playground-block-state')).toContainText(/No models available|暂无可用模型/i, { timeout: 20_000 })
    await expect(page.getByText(/Access Denied|拒绝访问/i)).toHaveCount(0)
    await expect(page.getByText(/^Channel$|^渠道$/)).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Submit' })).toBeDisabled()
    expect(pageGraphQLRequests.some((body) => /query\s+(Channels|Models)\b/.test(body))).toBe(false)

    await graphqlRequest(
      request,
      admin.token,
      `
        mutation SavePlaygroundPrice($input: SaveBillingPriceRuleForm!) {
          saveBillingPriceRule(input: $input) { id enabled }
        }
      `,
      {
        input: {
          scopeType: 'global',
          scopeId: 0,
          modelPattern: 'gpt-playground-smoke',
          price: {
            items: [
              { itemCode: 'prompt_tokens', pricing: { mode: 'usage_per_unit', usagePerUnit: '0.01' } },
              { itemCode: 'completion_tokens', pricing: { mode: 'usage_per_unit', usagePerUnit: '0.02' } },
            ],
          },
          currency: 'CNY',
          priority: 100,
          enabled: true,
          referenceId: `playground-${slug}`,
        },
      }
    )
    const channel = await graphqlRequest<{ createChannel: { id: string } }>(
      request,
      admin.token,
      `
        mutation CreatePlaygroundChannel($input: CreateChannelInput!) {
          createChannel(input: $input) { id }
        }
      `,
      {
        input: {
          type: 'openai_fake',
          name: `Playground fake ${slug}`,
          baseURL: 'https://fake.openai.com/v1',
          credentials: {},
          supportedModels: ['gpt-playground-smoke'],
          manualModels: ['gpt-playground-smoke'],
          autoSyncSupportedModels: false,
          tags: ['playwright', 'consumer-playground'],
          defaultTestModel: 'gpt-playground-smoke',
          orderingWeight: 0,
        },
      }
    )
    await graphqlRequest(
      request,
      admin.token,
      `
        mutation EnablePlaygroundChannel($id: ID!) {
          updateChannelStatus(id: $id, status: enabled) { id status }
        }
      `,
      { id: channel.createChannel.id }
    )

    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('playground-block-state')).toHaveCount(0, { timeout: 20_000 })
    await expect(page.getByTestId('playground-key-select').locator('input')).toHaveValue('Default API Key')
    await expect(page.getByTestId('playground-model-select').locator('input')).toHaveValue('gpt-playground-smoke')
    await expect(page.getByText(/price rule global \/ gpt-playground-smoke|价格规则 global \/ gpt-playground-smoke/i)).toBeVisible()

    const chatResponse = page.waitForResponse(
      (response) => new URL(response.url()).pathname === '/admin/account/playground/chat' && response.request().method() === 'POST'
    )
    const prompt = page.getByPlaceholder(/Type a message|输入消息/i)
    await prompt.fill('Count from one to twenty')
    await page.getByRole('button', { name: 'Submit' }).click()
    expect((await chatResponse).ok()).toBeTruthy()
    await expect(page.getByText(/1 2 3 4 5/)).toBeVisible({ timeout: 20_000 })
    expect(pageGraphQLRequests.some((body) => /query\s+(Channels|Models)\b/.test(body))).toBe(false)

    await page.setViewportSize({ width: 390, height: 844 })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('playground-key-select').locator('input')).toHaveValue('Default API Key', { timeout: 20_000 })
    const layout = await page.evaluate(() => ({ documentWidth: document.documentElement.scrollWidth, viewportWidth: innerWidth }))
    expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewportWidth)
    expect(pageErrors).toEqual([])
    expect(consoleErrors).toEqual([])
  })
})

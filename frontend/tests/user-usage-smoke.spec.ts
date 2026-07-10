import { expect, test, type Browser, type Page } from '@playwright/test'
import {
  apiBaseURL,
  enableCommercialRegistration,
  graphqlRequest,
  injectAuthSession,
  registerCommercialUser,
  signInViaApi,
  uniqueCommercialSlug,
  type AuthSession,
} from './commercial-smoke.utils'

test.describe('user requests and usage projection', () => {
  test('isolates users, reconciles billing, and gates project-wide visibility', async ({ browser, page, request }) => {
    test.setTimeout(120_000)
    const pageErrors: string[] = []
    const consoleErrors: string[] = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    page.on('console', (message) => {
      if (message.type() === 'error') consoleErrors.push(message.text())
    })

    const admin = await signInViaApi(request)
    await enableCommercialRegistration(request, admin.token)
    const slug = uniqueCommercialSlug('user-usage')
    const password = 'SmokePass123'
    const ownerEmail = `${slug}-owner@example.com`
    const memberEmail = `${slug}-member@example.com`
    await registerCommercialUser(request, { email: ownerEmail, password, firstName: 'Usage', lastName: 'Owner' })
    await registerCommercialUser(request, { email: memberEmail, password, firstName: 'Usage', lastName: 'Member' })
    const owner = await signInViaApi(request, { email: ownerEmail, password })
    const member = await signInViaApi(request, { email: memberEmail, password })
    const ownerWorkspacesResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${owner.token}` },
    })
    expect(ownerWorkspacesResponse.ok(), await ownerWorkspacesResponse.text()).toBeTruthy()
    const ownerWorkspaces = await ownerWorkspacesResponse.json()
    const projectId = ownerWorkspaces.workspaces[0]?.id as string
    expect(projectId).toBeTruthy()
    const memberIdentity = await graphqlRequest<{ me: { id: string } }>(
      request,
      member.token,
      `query UsageMemberIdentity { me { id } }`
    )

    await graphqlRequest(
      request,
      admin.token,
      `
        mutation AddUsageMember($input: AddUserToProjectInput!) {
          addUserToProject(input: $input) { id userID projectID isOwner scopes }
        }
      `,
      { input: { projectId, userId: memberIdentity.me.id, isOwner: false, scopes: [] } }
    )

    const memberKeyResponse = await request.post(
      `${apiBaseURL()}/admin/account/api-keys?projectId=${encodeURIComponent(projectId)}`,
      {
        headers: { Authorization: `Bearer ${member.token}` },
        data: { name: 'Member Usage Key', ipAllowlist: [], allowedModelIds: ['gpt-usage-smoke'] },
      }
    )
    expect(memberKeyResponse.ok(), await memberKeyResponse.text()).toBeTruthy()

    await graphqlRequest(
      request,
      admin.token,
      `
        mutation SaveUsagePrice($input: SaveBillingPriceRuleForm!) {
          saveBillingPriceRule(input: $input) { id enabled }
        }
      `,
      {
        input: {
          scopeType: 'global',
          scopeId: 0,
          modelPattern: 'gpt-usage-smoke',
          price: {
            items: [
              { itemCode: 'prompt_tokens', pricing: { mode: 'usage_per_unit', usagePerUnit: '0.01' } },
              { itemCode: 'completion_tokens', pricing: { mode: 'usage_per_unit', usagePerUnit: '0.02' } },
            ],
          },
          currency: 'CNY',
          priority: 120,
          enabled: true,
          referenceId: `user-usage-${slug}`,
        },
      }
    )
    const channel = await graphqlRequest<{ createChannel: { id: string } }>(
      request,
      admin.token,
      `
        mutation CreateUsageChannel($input: CreateChannelInput!) {
          createChannel(input: $input) { id }
        }
      `,
      {
        input: {
          type: 'openai_fake',
          name: `Usage fake ${slug}`,
          baseURL: 'https://fake.openai.com/v1',
          credentials: {},
          supportedModels: ['gpt-usage-smoke'],
          manualModels: ['gpt-usage-smoke'],
          autoSyncSupportedModels: false,
          tags: ['playwright', 'user-usage'],
          defaultTestModel: 'gpt-usage-smoke',
          orderingWeight: 0,
        },
      }
    )
    await graphqlRequest(
      request,
      admin.token,
      `mutation EnableUsageChannel($id: ID!) { updateChannelStatus(id: $id, status: enabled) { id status } }`,
      { id: channel.createChannel.id }
    )

    await sendPlaygroundRequest(browser, owner, projectId, `owner-${slug}`)
    await sendPlaygroundRequest(browser, member, projectId, `member-${slug}`)

    const ownerMine = await pollJSON(request, owner.token, `/admin/account/requests?projectId=${encodeURIComponent(projectId)}`, 1)
    const memberMine = await pollJSON(request, member.token, `/admin/account/requests?projectId=${encodeURIComponent(projectId)}`, 1)
    expect(ownerMine.items).toHaveLength(1)
    expect(memberMine.items).toHaveLength(1)
    expect(ownerMine.items[0].id).not.toBe(memberMine.items[0].id)
    expect(JSON.stringify(ownerMine).toLowerCase()).not.toMatch(/channel|upstream|trace|execution|clientip|requestheaders/)

    const guessed = await request.get(
      `${apiBaseURL()}/admin/account/requests/${encodeURIComponent(memberMine.items[0].id)}?projectId=${encodeURIComponent(projectId)}`,
      { headers: { Authorization: `Bearer ${owner.token}` } }
    )
    expect(guessed.status()).toBe(403)

    const memberProject = await request.get(`${apiBaseURL()}/admin/account/project-requests?projectId=${encodeURIComponent(projectId)}`, {
      headers: { Authorization: `Bearer ${member.token}` },
    })
    expect(memberProject.status()).toBe(403)
    const ownerProject = await request.get(`${apiBaseURL()}/admin/account/project-requests?projectId=${encodeURIComponent(projectId)}`, {
      headers: { Authorization: `Bearer ${owner.token}` },
    })
    expect(ownerProject.ok(), await ownerProject.text()).toBeTruthy()
    expect((await ownerProject.json()).items).toHaveLength(2)

    const ownerDetail = await pollDetail(request, owner.token, projectId, ownerMine.items[0].id)
    expect(JSON.stringify(ownerDetail.requestBody)).toContain(`owner-${slug}`)
    expect(ownerDetail.billing.billingRecordId).toBeTruthy()
    expect(ownerDetail.billing.ledgerTransactionId).toBeTruthy()
    expect(ownerDetail.billing.priceReferenceId).toBe(`user-usage-${slug}`)
    expect(ownerDetail.billing.priceSnapshot).toBeTruthy()
    expect(ownerDetail.chargeAmountMicros).toBe(ownerDetail.billing.chargeAmountMicros)
    expect(JSON.stringify(ownerDetail).toLowerCase()).not.toMatch(/channel|upstream|trace|execution|clientip|requestheaders/)

    const exportResponse = await request.get(
      `${apiBaseURL()}/admin/account/requests/export?projectId=${encodeURIComponent(projectId)}`,
      { headers: { Authorization: `Bearer ${owner.token}` } }
    )
    expect(exportResponse.ok(), await exportResponse.text()).toBeTruthy()
    const exportText = await exportResponse.text()
    expect(exportText).toContain(ownerMine.items[0].id)
    expect(exportText).not.toContain(memberMine.items[0].id)
    expect(exportText.toLowerCase()).not.toMatch(/channel|upstream|trace|execution/)

    const now = new Date()
    const from = new Date(now.getTime() - 48 * 60 * 60 * 1000).toISOString()
    const to = new Date(now.getTime() + 60 * 60 * 1000).toISOString()
    const ownerUsage = await pollUsage(request, owner.token, projectId, from, to)
    expect(ownerUsage.requestCount).toBe(1)
    expect(ownerUsage.chargeAmountMicros).toBe(ownerDetail.billing.chargeAmountMicros)
    expect(ownerUsage.totalTokens).toBeGreaterThan(0)

    const memberContext = await browser.newContext()
    const memberPage = await memberContext.newPage()
    await injectAuthSession(memberPage, member)
    await memberPage.addInitScript((id) => localStorage.setItem('axonhub_selected_project_id', id), projectId)
    await memberPage.goto('/project/requests', { waitUntil: 'domcontentloaded' })
    await expect(memberPage.getByTestId('user-requests-page')).toBeVisible({ timeout: 20_000 })
    await expect(memberPage.getByTestId('user-requests-table').locator('tbody tr')).toHaveCount(1)
    await expect(memberPage.getByTestId('user-usage-scope-switch')).toHaveCount(0)
    await memberPage.goto('/project/usage-stats', { waitUntil: 'domcontentloaded' })
    await expect(memberPage.getByTestId('user-usage-page')).toBeVisible({ timeout: 20_000 })
    await expect(memberPage.getByTestId('user-usage-scope-switch')).toHaveCount(0)
    await memberContext.close()

    await injectAuthSession(page, owner)
    await page.addInitScript((id) => localStorage.setItem('axonhub_selected_project_id', id), projectId)
    await page.goto('/project/requests', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('user-requests-page')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('user-requests-table').locator('tbody tr')).toHaveCount(1)
    await expect(page.getByTestId('user-usage-scope-switch')).toBeVisible()
    const requestProjection = page.getByTestId('user-requests-page')
    await expect(requestProjection).not.toContainText(/Channel|渠道|Upstream|上游账户|Trace|执行记录/i)

    const scopeSwitch = page.getByTestId('user-usage-scope-switch')
    await scopeSwitch.getByRole('button', { name: /Project|项目/ }).click()
    await expect(page.getByTestId('user-requests-table').locator('tbody tr')).toHaveCount(2)
    await scopeSwitch.getByRole('button', { name: /Mine|我的/ }).click()
    await page.getByTitle(/View request|查看请求/).click()
    await expect(page.getByTestId('user-request-detail')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('user-request-detail')).toContainText(`user-usage-${slug}`)
    await expect(page.getByTestId('user-request-detail')).not.toContainText(/Channel|渠道|Upstream|上游账户|Trace|执行记录/i)

    await page.goto('/project/usage-stats', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('user-usage-page')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('user-usage-summary')).toContainText(/1/)
    await expect(page.getByTestId('user-usage-chart').locator('svg')).toBeVisible()

    await page.setViewportSize({ width: 390, height: 844 })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('user-usage-page')).toBeVisible({ timeout: 20_000 })
    const layout = await page.evaluate(() => ({ documentWidth: document.documentElement.scrollWidth, viewportWidth: innerWidth }))
    expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewportWidth)
    expect(pageErrors).toEqual([])
    expect(consoleErrors).toEqual([])
  })
})

async function sendPlaygroundRequest(browser: Browser, session: AuthSession, projectId: string, marker: string) {
  const context = await browser.newContext()
  const page = await context.newPage()
  await injectAuthSession(page, session)
  await page.addInitScript((id) => localStorage.setItem('axonhub_selected_project_id', id), projectId)
  await page.goto('/project/playground', { waitUntil: 'domcontentloaded' })
  const modelInput = page.getByTestId('playground-model-select').locator('input')
  await expect(modelInput).toBeVisible({ timeout: 20_000 })
  if ((await modelInput.inputValue()) !== 'gpt-usage-smoke') {
    await modelInput.click()
    await modelInput.fill('gpt-usage-smoke')
    await page.getByRole('option', { name: 'gpt-usage-smoke' }).click()
  }
  await expect(modelInput).toHaveValue('gpt-usage-smoke')
  const response = page.waitForResponse(
    (item) => new URL(item.url()).pathname === '/admin/account/playground/chat' && item.request().method() === 'POST'
  )
  await page.getByPlaceholder(/Type a message|输入消息/i).fill(marker)
  await page.getByRole('button', { name: 'Submit' }).click()
  expect((await response).ok()).toBeTruthy()
  await expect(page.getByText(/1 2 3 4 5/)).toBeVisible({ timeout: 20_000 })
  await context.close()
}

async function pollJSON(request: any, token: string, path: string, expectedTotal: number) {
  let payload: any
  await expect
    .poll(
      async () => {
        const response = await request.get(`${apiBaseURL()}${path}`, { headers: { Authorization: `Bearer ${token}` } })
        if (!response.ok()) return -1
        payload = await response.json()
        return payload.total
      },
      { timeout: 20_000 }
    )
    .toBe(expectedTotal)
  return payload
}

async function pollUsage(request: any, token: string, projectId: string, from: string, to: string) {
  let payload: any
  await expect
    .poll(
      async () => {
        const response = await request.get(
          `${apiBaseURL()}/admin/account/usage?projectId=${encodeURIComponent(projectId)}&from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
          { headers: { Authorization: `Bearer ${token}` } }
        )
        if (!response.ok()) return -1
        payload = await response.json()
        return payload.requestCount
      },
      { timeout: 20_000 }
    )
    .toBe(1)
  return payload
}

async function pollDetail(request: any, token: string, projectId: string, requestId: string) {
  let payload: any
  await expect
    .poll(
      async () => {
        const response = await request.get(
          `${apiBaseURL()}/admin/account/requests/${encodeURIComponent(requestId)}?projectId=${encodeURIComponent(projectId)}`,
          { headers: { Authorization: `Bearer ${token}` } }
        )
        if (!response.ok()) return ''
        payload = await response.json()
        return payload.billing?.billingRecordId || ''
      },
      { timeout: 30_000 }
    )
    .not.toBe('')
  return payload
}

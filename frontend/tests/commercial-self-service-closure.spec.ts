import { expect, test, type Page } from '@playwright/test'
import {
  apiBaseURL,
  enableCommercialRegistration,
  graphqlRequest,
  injectAuthSession,
  openBillingView,
  registerCommercialUser,
  seedCommercialAssets,
  signInViaApi,
  uniqueCommercialSlug,
  waitForBillingOverview,
  type BillingView,
} from './commercial-smoke.utils'

const billingViews: BillingView[] = [
  'wallet',
  'recharge',
  'orders',
  'subscriptions',
  'redeem',
  'affiliate',
  'notifications',
  'ledger',
  'usage',
]

test.describe.configure({ mode: 'serial' })

test.describe('commercial self-service closure', () => {
  test('keeps consumer navigation, model discovery, AI usage, and billing in one flow', async ({ page, request }) => {
    test.setTimeout(120_000)
    const pageErrors: string[] = []
    const consoleErrors: string[] = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    page.on('console', (message) => {
      if (message.type() === 'error') consoleErrors.push(message.text())
    })

    const admin = await signInViaApi(request)
    await enableCommercialRegistration(request, admin.token)
    const slug = uniqueCommercialSlug('self-service-closure')
    const seed = await seedCommercialAssets(request, admin.token, slug)
    const modelId = `gpt-self-service-${slug.slice(-8)}`
    const channel = await graphqlRequest<{ createChannel: { id: string } }>(
      request,
      admin.token,
      `
        mutation CreateSelfServiceChannel($input: CreateChannelInput!) {
          createChannel(input: $input) { id }
        }
      `,
      {
        input: {
          type: 'openai_fake',
          name: `Self-service fake ${slug}`,
          baseURL: 'https://fake.openai.com/v1',
          credentials: {},
          supportedModels: [modelId],
          manualModels: [modelId],
          autoSyncSupportedModels: false,
          tags: ['playwright', 'self-service-closure'],
          defaultTestModel: modelId,
          orderingWeight: 0,
        },
      }
    )
    await graphqlRequest(
      request,
      admin.token,
      `mutation EnableSelfServiceChannel($id: ID!) { updateChannelStatus(id: $id, status: enabled) { id status } }`,
      { id: channel.createChannel.id }
    )

    const email = `${slug}@example.com`
    const password = 'SmokePass123'
    await registerCommercialUser(request, { email, password, firstName: 'Self Service', lastName: 'User' })
    const user = await signInViaApi(request, { email, password })
    const workspacesResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${user.token}` },
    })
    expect(workspacesResponse.ok(), await workspacesResponse.text()).toBeTruthy()
    const workspaces = await workspacesResponse.json()
    const projectId = workspaces.workspaces[0]?.id as string | undefined
    expect(projectId).toBeTruthy()

    await injectAuthSession(page, user)
    await page.addInitScript((id) => localStorage.setItem('axonhub_selected_project_id', id), projectId)
    await page.goto('/home', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('user-home')).toBeVisible({ timeout: 20_000 })

    const workspaceNavigation = page.getByTestId('sidebar-group-workspace')
    await expect(workspaceNavigation).toBeVisible()
    const consumerRoutes = [
      '/home',
      '/workspaces',
      '/project/api-keys',
      '/project/playground',
      '/project/usage-stats',
      '/project/models',
      '/billing',
      '/settings/profile',
    ]
    await expect(workspaceNavigation.locator('a')).toHaveCount(consumerRoutes.length)
    for (const href of consumerRoutes) await expect(workspaceNavigation.locator(`a[href="${href}"]`)).toBeVisible()
    await expect(page.getByTestId('sidebar-group-admin')).toHaveCount(0)
    await expect(page.getByTestId('sidebar-group-project')).toBeVisible()
    for (const href of ['/channels', '/channels/accounts', '/admin/billing', '/users', '/roles', '/system']) {
      await expect(page.locator(`a[href="${href}"]`)).toHaveCount(0)
    }

    await page.goto('/project/api-keys', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('personal-api-keys-page')).toBeVisible({ timeout: 20_000 })
    await page.getByTestId('create-personal-api-key').click()
    await page.locator('#personal-key-name').fill('Self-service Browser Key')
    await page.getByRole('button', { name: /^Save$|^保存$/ }).click()
    await expect(page.getByTestId('personal-api-key-secret')).toBeVisible({ timeout: 20_000 })
    await page.getByRole('button', { name: /^Done$|^完成$/ }).click()
    await expect(page.getByText('Self-service Browser Key')).toBeVisible()

    await page.goto('/project/models', { waitUntil: 'domcontentloaded' })
    const modelCard = page.getByTestId(`consumer-model-${modelId}`)
    await expect(modelCard).toBeVisible({ timeout: 20_000 })
    await expect(modelCard).toContainText(/CNY/)
    await expect(modelCard).toContainText(/0\.01/)
    await expect(modelCard).not.toContainText(/Channel|渠道|Upstream|上游/i)

    await page.goto('/project/playground', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('playground-model-select').locator('input')).toHaveValue(modelId, { timeout: 20_000 })
    const chatResponse = page.waitForResponse(
      (response) => new URL(response.url()).pathname === '/admin/account/playground/chat' && response.request().method() === 'POST'
    )
    await page.getByPlaceholder(/Type a message|输入消息/i).fill('Count from one to twenty')
    await page.getByRole('button', { name: 'Submit' }).click()
    expect((await chatResponse).ok()).toBeTruthy()
    await expect(page.getByText(/1 2 3 4 5/)).toBeVisible({ timeout: 20_000 })

    await page.goto('/project/usage-stats', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('user-usage-page')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('user-usage-summary')).toContainText(/1/)

    const billingResponse = waitForBillingOverview(page)
    await page.goto('/billing', { waitUntil: 'domcontentloaded' })
    await billingResponse
    await expect(page.getByTestId('billing-wallet-view')).toContainText(/active/i)
    for (const view of billingViews) await openBillingView(page, view)
    await openBillingView(page, 'subscriptions')
    await expect(page.getByText(seed.planName)).toBeVisible()
    await openBillingView(page, 'usage')
    await expect(page.getByTestId('billing-usage-view')).toContainText(modelId)
    await expect(page.getByTestId('billing-usage-view')).toContainText(/charged/i)
    await openBillingView(page, 'ledger')
    await expect(page.getByTestId('billing-ledger-view').locator('tbody tr')).not.toHaveCount(0)
    await openBillingView(page, 'affiliate')
    await expect(page.getByRole('button', { name: /Transfer available rebates|转出可用返利/i })).toBeDisabled()

    await page.setViewportSize({ width: 390, height: 844 })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('billing-affiliate-view')).toBeVisible({ timeout: 20_000 })
    const mobileLayout = await page.evaluate(() => ({ documentWidth: document.documentElement.scrollWidth, viewportWidth: innerWidth }))
    expect(mobileLayout.documentWidth).toBeLessThanOrEqual(mobileLayout.viewportWidth)
    expect(pageErrors).toEqual([])
    expect(consoleErrors).toEqual([])

    const adminPage = await page.context().newPage()
    await injectAuthSession(adminPage, admin)
    for (const path of ['/admin/billing', '/channels', '/project/request-admin']) {
      await adminPage.goto(path, { waitUntil: 'domcontentloaded' })
      await expect(adminPage.getByText(/Access Denied|拒绝访问/i)).toHaveCount(0)
    }
    await adminPage.close()
  })

  test('renders billing loading, empty, disabled, and error states', async ({ browser, request }) => {
    test.setTimeout(60_000)
    const admin = await signInViaApi(request)
    await enableCommercialRegistration(request, admin.token)
    const slug = uniqueCommercialSlug('billing-states')
    const email = `${slug}@example.com`
    const password = 'SmokePass123'
    await registerCommercialUser(request, { email, password, firstName: 'Billing', lastName: 'States' })
    const user = await signInViaApi(request, { email, password })

    const loadingContext = await browser.newContext()
    const loadingPage = await loadingContext.newPage()
    await injectAuthSession(loadingPage, user)
    let releaseOverview!: () => void
    const overviewGate = new Promise<void>((resolve) => {
      releaseOverview = resolve
    })
    await loadingPage.route('**/admin/graphql', async (route) => {
      if ((route.request().postData() || '').includes('MyBillingOverview')) await overviewGate
      await route.continue()
    })
    await loadingPage.goto('/billing', { waitUntil: 'domcontentloaded' })
    await expect(loadingPage.getByTestId('billing-wallet-view')).toBeVisible({ timeout: 20_000 })
    await expect(loadingPage.getByTestId('billing-wallet-view').getByText('-', { exact: true }).first()).toBeVisible()
    const overviewResponse = waitForBillingOverview(loadingPage)
    releaseOverview()
    await overviewResponse
    await openBillingView(loadingPage, 'orders')
    await expect(loadingPage.getByTestId('billing-orders-view')).toContainText(/No data|暂无数据/i)
    await openBillingView(loadingPage, 'notifications')
    await expect(loadingPage.getByRole('button', { name: /Mark all read|全部已读/i })).toBeDisabled()
    await loadingContext.close()

    const errorContext = await browser.newContext()
    const errorPage = await errorContext.newPage()
    await injectAuthSession(errorPage, user)
    await errorPage.route('**/admin/graphql', async (route) => {
      if ((route.request().postData() || '').includes('MyBillingOverview')) {
        await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'forced billing failure' }) })
        return
      }
      await route.continue()
    })
    await errorPage.goto('/billing', { waitUntil: 'domcontentloaded' })
    await expect(errorPage.getByRole('alert')).toContainText(/Load failed|加载失败|Request failed/i, { timeout: 20_000 })
    await errorContext.close()
  })
})

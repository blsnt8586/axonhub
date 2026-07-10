import { APIRequestContext, Browser, Page, TestInfo, expect, test } from '@playwright/test'
import {
  AuthSession,
  enableCommercialRegistration,
  graphqlRequest,
  injectAuthSession,
  seedCommercialAssets,
  signInViaApi,
  uniqueCommercialSlug,
  waitForBillingOverview,
} from './commercial-smoke.utils'

type ViewportCase = {
  name: 'desktop' | 'mobile'
  width: number
  height: number
}

type ResponsiveSeed = {
  adminSession: AuthSession
  userSession: AuthSession
  accountName: string
}

const viewports: ViewportCase[] = [
  { name: 'desktop', width: 1440, height: 1000 },
  { name: 'mobile', width: 390, height: 844 },
]

test.describe.configure({ mode: 'serial' })

test.describe('commercial responsive browser smoke', () => {
  test('keeps billing and upstream monitoring usable on desktop and mobile', async ({ browser, request }, testInfo) => {
    test.setTimeout(180_000)

    const seed = await seedResponsiveData(request)

    for (const viewport of viewports) {
      await verifyUserBilling(browser, seed.userSession, viewport, testInfo)
      await verifyAdminBilling(browser, seed.adminSession, viewport, testInfo)
      const accountHref = await verifyAccountMonitoring(browser, seed, viewport, testInfo)
      await verifyAccountDetail(browser, seed, accountHref, viewport, testInfo)
    }
  })
})

async function seedResponsiveData(request: APIRequestContext): Promise<ResponsiveSeed> {
  const slug = uniqueCommercialSlug('responsive-smoke')
  const adminSession = await signInViaApi(request)
  await enableCommercialRegistration(request, adminSession.token)
  const commercial = await seedCommercialAssets(request, adminSession.token, slug)

  const userEmail = `${slug}@example.com`
  const userPassword = 'ResponsivePass123'
  const registerResponse = await request.post(`${apiBaseURL()}/admin/auth/register`, {
    data: {
      email: userEmail,
      password: userPassword,
      firstName: 'Responsive',
      lastName: 'User',
      preferLanguage: 'en',
    },
  })
  expect(registerResponse.ok(), await registerResponse.text()).toBeTruthy()

  const userSession = await signInViaApi(request, { email: userEmail, password: userPassword })
  await graphqlRequest(
    request,
    userSession.token,
    `
      mutation ResponsiveRedeemCode($input: RedeemCodeInput!) {
        redeemCode(input: $input) {
          id
          status
        }
      }
    `,
    { input: { code: commercial.redeemCode } }
  )
  await graphqlRequest(
    request,
    userSession.token,
    `
      mutation ResponsivePurchaseSubscription($input: PurchaseSubscriptionPlanInput!) {
        purchaseSubscriptionPlan(input: $input) {
          id
          status
        }
      }
    `,
    { input: { planId: commercial.planId } }
  )

  const channelName = `Responsive channel ${slug}`
  const channel = await graphqlRequest<{
    createChannel: { id: string }
  }>(
    request,
    adminSession.token,
    `
      mutation ResponsiveCreateChannel($input: CreateChannelInput!) {
        createChannel(input: $input) {
          id
        }
      }
    `,
    {
      input: {
        type: 'openai',
        name: channelName,
        baseURL: `https://api.${slug}.example.com/v1`,
        credentials: {
          apiKey: `sk-responsive-channel-${slug}`,
          apiKeys: [`sk-responsive-channel-${slug}`],
        },
        supportedModels: ['gpt-4o'],
        manualModels: ['gpt-4o'],
        autoSyncSupportedModels: false,
        tags: ['playwright', 'responsive'],
        defaultTestModel: 'gpt-4o',
        orderingWeight: 0,
        remark: 'Responsive browser smoke channel',
      },
    }
  )

  const pool = await graphqlRequest<{
    createUpstreamAccountPool: { id: string }
  }>(
    request,
    adminSession.token,
    `
      mutation ResponsiveCreatePool($input: CreateUpstreamAccountPoolInput!) {
        createUpstreamAccountPool(input: $input) {
          id
        }
      }
    `,
    {
      input: {
        channelID: channel.createChannel.id,
        name: `Responsive pool ${slug}`,
        status: 'enabled',
        priority: 10,
        modelPatterns: ['gpt-*'],
        projectIDs: [],
        remark: 'Responsive browser smoke pool',
      },
    }
  )

  const accountName = `Responsive account ${slug}`
  await graphqlRequest<{
    createUpstreamAccount: { id: string }
  }>(
    request,
    adminSession.token,
    `
      mutation ResponsiveCreateAccount($input: CreateUpstreamAccountInput!) {
        createUpstreamAccount(input: $input) {
          id
        }
      }
    `,
    {
      input: {
        channelID: channel.createChannel.id,
        poolID: pool.createUpstreamAccountPool.id,
        name: accountName,
        credentialType: 'api_key',
        credentials: { apiKey: `sk-responsive-account-${slug}` },
        status: 'active',
        schedulable: true,
        priority: 10,
        weight: 100,
        concurrencyLimit: 5,
        rateMultiplier: 1,
        quotaLimitMicros: 100000000,
        quotaUsedMicros: 25000000,
      },
    }
  )

  return {
    adminSession,
    userSession,
    accountName,
  }
}

async function verifyUserBilling(browser: Browser, session: AuthSession, viewport: ViewportCase, testInfo: TestInfo) {
  const page = await authenticatedPage(browser, session, viewport)
  await page.goto('/billing', { waitUntil: 'domcontentloaded' })
  await waitForBillingOverview(page)

  await expect(page.locator('#billing-recharge-amount')).toBeVisible()
  await expect(page.locator('#billing-redeem-code')).toBeVisible()
  await expect(page.locator('#billing-subscription-promo')).toBeVisible()
  await expect(page.getByText('active', { exact: true }).first()).toBeVisible()

  await assertResponsiveLayout(page, `user-billing-${viewport.name}`, testInfo)
  await page.context().close()
}

async function verifyAdminBilling(browser: Browser, session: AuthSession, viewport: ViewportCase, testInfo: TestInfo) {
  const page = await authenticatedPage(browser, session, viewport)
  await page.goto('/admin/billing', { waitUntil: 'domcontentloaded' })

  await expect(page.getByRole('heading', { name: /Billing Admin|计费/i })).toBeVisible({ timeout: 20000 })
  const operationsTab = page.getByRole('tab', { name: /Operations|运维|运营/i })
  await operationsTab.scrollIntoViewIfNeeded()
  await operationsTab.click()
  await expect(operationsTab).toHaveAttribute('data-state', 'active')

  await assertResponsiveLayout(page, `admin-billing-${viewport.name}`, testInfo)
  await page.context().close()
}

async function verifyAccountMonitoring(
  browser: Browser,
  seed: ResponsiveSeed,
  viewport: ViewportCase,
  testInfo: TestInfo
): Promise<string> {
  const page = await authenticatedPage(browser, seed.adminSession, viewport)
  await page.goto('/channels/accounts', { waitUntil: 'domcontentloaded' })

  await expect(page.getByRole('heading', { name: /Upstream accounts/i })).toBeVisible({ timeout: 20000 })
  const accountRow = page.locator('tbody tr').filter({ hasText: seed.accountName }).first()
  await expect(accountRow).toBeVisible({ timeout: 20000 })
  const accountHref = await accountRow.locator('a[href*="/channels/accounts/"]').getAttribute('href')
  expect(accountHref).toBeTruthy()

  await assertResponsiveLayout(page, `account-monitoring-${viewport.name}`, testInfo)
  await page.context().close()
  return accountHref!
}

async function verifyAccountDetail(
  browser: Browser,
  seed: ResponsiveSeed,
  accountHref: string,
  viewport: ViewportCase,
  testInfo: TestInfo
) {
  const page = await authenticatedPage(browser, seed.adminSession, viewport)
  await page.goto(accountHref, { waitUntil: 'domcontentloaded' })

  await expect(page.getByRole('heading', { name: seed.accountName })).toBeVisible({ timeout: 20000 })
  await expect(page.getByText(/Quota and cooldown/i)).toBeVisible()
  await expect(page.getByText(/Recent executions/i)).toBeVisible()
  await expect(page.getByText('Switch history', { exact: true })).toBeVisible()

  await assertResponsiveLayout(page, `account-detail-${viewport.name}`, testInfo)
  await page.context().close()
}

async function authenticatedPage(browser: Browser, session: AuthSession, viewport: ViewportCase): Promise<Page> {
  const context = await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    deviceScaleFactor: viewport.name === 'mobile' ? 2 : 1,
  })
  const page = await context.newPage()
  await injectAuthSession(page, session)
  return page
}

async function assertResponsiveLayout(page: Page, name: string, testInfo: TestInfo) {
  await page.evaluate(async () => {
    await document.fonts.ready
  })
  await page.waitForTimeout(300)

  const diagnostics = await page.evaluate(() => {
    const viewportWidth = window.innerWidth
    const documentWidth = Math.max(document.documentElement.scrollWidth, document.body.scrollWidth)
    const candidates = Array.from(
      document.querySelectorAll<HTMLElement>('button, input, textarea, select, [role="tab"]')
    )
      .filter((element, index, elements) => elements.indexOf(element) === index)
      .filter((element) => {
        const style = window.getComputedStyle(element)
        const rect = element.getBoundingClientRect()
        return (
          style.display !== 'none' &&
          style.visibility !== 'hidden' &&
          Number(style.opacity) !== 0 &&
          rect.width > 1 &&
          rect.height > 1 &&
          rect.right > 0 &&
          rect.left < viewportWidth
        )
      })
      .map((element) => {
        const rect = element.getBoundingClientRect()
        return {
          element,
          label: element.getAttribute('aria-label') || element.textContent?.trim().slice(0, 80) || element.tagName,
          left: rect.left,
          right: rect.right,
          top: rect.top,
          bottom: rect.bottom,
        }
      })

    const overlaps: string[] = []
    for (let leftIndex = 0; leftIndex < candidates.length; leftIndex += 1) {
      for (let rightIndex = leftIndex + 1; rightIndex < candidates.length; rightIndex += 1) {
        const left = candidates[leftIndex]
        const right = candidates[rightIndex]
        if (left.element.contains(right.element) || right.element.contains(left.element)) continue

        const overlapWidth = Math.min(left.right, right.right) - Math.max(left.left, right.left)
        const overlapHeight = Math.min(left.bottom, right.bottom) - Math.max(left.top, right.top)
        if (overlapWidth > 2 && overlapHeight > 2) {
          overlaps.push(`${left.label} <> ${right.label}`)
        }
      }
    }

    return {
      viewportWidth,
      documentWidth,
      horizontalOverflow: Math.max(0, documentWidth - viewportWidth),
      overlaps,
    }
  })

  expect(diagnostics.horizontalOverflow, `${name} document overflow: ${JSON.stringify(diagnostics)}`).toBeLessThanOrEqual(2)
  expect(diagnostics.overlaps, `${name} overlapping controls: ${JSON.stringify(diagnostics.overlaps)}`).toEqual([])

  const screenshot = await page.screenshot({ fullPage: true })
  await testInfo.attach(`${name}.png`, { body: screenshot, contentType: 'image/png' })
}

function apiBaseURL() {
  return process.env.AXONHUB_API_URL || 'http://localhost:8099'
}

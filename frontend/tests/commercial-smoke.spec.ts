import { expect, test } from '@playwright/test'
import {
  enableCommercialRegistration,
  injectAuthSession,
  seedCommercialAssets,
  signInViaApi,
  uniqueCommercialSlug,
  waitForBillingOverview,
} from './commercial-smoke.utils'

test.describe.configure({ mode: 'serial' })

test.describe('commercial browser smoke', () => {
  test('covers owner billing console and user billing lifecycle', async ({ browser, page, request }) => {
    test.setTimeout(90_000)

    const slug = uniqueCommercialSlug()
    const adminSession = await signInViaApi(request)

    await enableCommercialRegistration(request, adminSession.token)
    const seed = await seedCommercialAssets(request, adminSession.token, slug)

    await injectAuthSession(page, adminSession)
    await page.goto('/admin/billing', { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('heading', { name: /Billing Admin|计费/i })).toBeVisible({ timeout: 20000 })
    await expect(page.getByRole('tab', { name: /Wallet accounts|钱包|账户/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Redeem codes|兑换/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Subscriptions|订阅/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Sell price rules|价格|定价/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Payment providers|支付|提供商/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Operations|运维|运营/i })).toBeVisible()

    const userEmail = `${slug}@example.com`
    const userPassword = 'SmokePass123'
    const userContext = await browser.newContext()
    const userPage = await userContext.newPage()

    await userPage.goto('/sign-up', { waitUntil: 'domcontentloaded' })
    await userPage.getByRole('textbox', { name: /^Email$/i }).fill(userEmail)
    await userPage.getByRole('textbox', { name: /^Password$/i }).fill(userPassword)
    await userPage.getByRole('textbox', { name: /Confirm Password|确认密码/i }).fill(userPassword)

    await Promise.all([
      userPage.waitForURL((url) => url.toString().includes('/sign-in'), { timeout: 20000 }),
      userPage.getByRole('button', { name: /Create Account|创建账户/i }).click(),
    ])

    const userSession = await signInViaApi(request, { email: userEmail, password: userPassword })
    await injectAuthSession(userPage, userSession)
    await userPage.goto('/billing', { waitUntil: 'domcontentloaded' })
    await waitForBillingOverview(userPage)

    await expect(userPage.locator('#billing-recharge-amount')).toBeVisible()
    await expect(userPage.locator('#billing-redeem-code')).toBeVisible()
    await expect(userPage.locator('#billing-subscription-promo')).toBeVisible()
    await expect(userPage.getByText(seed.planName)).toBeVisible({ timeout: 20000 })

    await userPage.locator('#billing-redeem-code').fill(seed.redeemCode)
    await Promise.all([
      userPage.waitForResponse((response) => {
        const body = response.request().postData() || ''
        return response.url().includes('/admin/graphql') && body.includes('RedeemCode') && response.status() === 200
      }),
      userPage.getByRole('button', { name: /Redeem|兑换/i }).click(),
    ])
    await expect(userPage.getByText(seed.redeemCode)).toBeVisible({ timeout: 20000 })

    userPage.once('dialog', async (dialog) => {
      expect(dialog.message()).toContain(seed.planName)
      await dialog.accept()
    })

    const planCard = userPage.locator('div').filter({ hasText: seed.planName }).filter({ has: userPage.getByRole('button', { name: /Purchase|购买/i }) }).first()
    const purchaseResponsePromise = userPage.waitForResponse((response) => {
      const body = response.request().postData() || ''
      return response.url().includes('/admin/graphql') && body.includes('PurchaseSubscriptionPlan') && response.status() === 200
    })
    await planCard.getByRole('button', { name: /Purchase|购买/i }).click()
    const purchasePayload = await purchaseResponsePromise.then((response) => response.json())
    expect(purchasePayload.errors, JSON.stringify(purchasePayload.errors ?? [], null, 2)).toBeFalsy()
    expect(purchasePayload.data?.purchaseSubscriptionPlan?.status).toBe('active')

    await userPage.locator('#billing-recharge-amount').fill('2.00')
    const paymentReturnPromise = userPage.waitForURL((url) => url.pathname === '/billing' && url.searchParams.get('trade_status') === 'TRADE_SUCCESS', { timeout: 30000 })
    await userPage.getByRole('button', { name: /Recharge|充值|Pay|支付/i }).click()
    await paymentReturnPromise
    await userPage.goto('/billing', { waitUntil: 'domcontentloaded' })
    await waitForBillingOverview(userPage)
    await expect(userPage.getByText('paid', { exact: true }).first()).toBeVisible({ timeout: 20000 })

    await userContext.close()
  })
})

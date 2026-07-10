import { expect, test } from '@playwright/test'
import {
  enableCommercialRegistration,
  injectAuthSession,
  graphqlRequest,
  registerCommercialUser,
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

    const inviterEmail = `${slug}-inviter@example.com`
    const userPassword = 'SmokePass123'
    await registerCommercialUser(request, {
      email: inviterEmail,
      password: userPassword,
      firstName: 'Smoke',
      lastName: 'Inviter',
    })
    const inviterSession = await signInViaApi(request, { email: inviterEmail, password: userPassword })
    const inviterData = await graphqlRequest<{
      myAffiliateSummary: { profile: { inviteCode: string } }
    }>(
      request,
      inviterSession.token,
      `
        query SmokeInviterProfile {
          myAffiliateSummary {
            profile { inviteCode }
          }
        }
      `
    )

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
    const userContext = await browser.newContext()
    const userPage = await userContext.newPage()

    await userPage.goto('/sign-in', { waitUntil: 'domcontentloaded' })
    const createAccountLink = userPage.getByRole('link', { name: /Create account|创建账户/i })
    await expect(createAccountLink).toBeVisible({ timeout: 20000 })
    await createAccountLink.click()
    await expect(userPage).toHaveURL(/\/sign-up$/)
    await expect(userPage.getByRole('textbox', { name: /Confirm Password|确认密码/i })).toBeVisible()
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

    await userPage.locator('#billing-affiliate-invite').fill(inviterData.myAffiliateSummary.profile.inviteCode)
    const bindResponsePromise = userPage.waitForResponse((response) => {
      const body = response.request().postData() || ''
      return response.url().includes('/admin/graphql') && body.includes('BindAffiliateInvite') && response.status() === 200
    })
    await userPage.getByRole('button', { name: /Bind inviter|绑定邀请人/i }).click()
    const bindPayload = await bindResponsePromise.then((response) => response.json())
    expect(bindPayload.errors, JSON.stringify(bindPayload.errors ?? [], null, 2)).toBeFalsy()
    expect(bindPayload.data?.bindAffiliateInvite?.inviteCode).toBe(inviterData.myAffiliateSummary.profile.inviteCode)

    await userPage.locator('#billing-redeem-code').fill(seed.redeemCode)
    await Promise.all([
      userPage.waitForResponse((response) => {
        const body = response.request().postData() || ''
        return response.url().includes('/admin/graphql') && body.includes('RedeemCode') && response.status() === 200
      }),
      userPage.getByRole('button', { name: /Redeem|兑换/i }).click(),
    ])
    await expect(userPage.getByText(seed.redeemCode)).toBeVisible({ timeout: 20000 })

    await userPage.locator('#billing-subscription-promo').fill(seed.subscriptionPromoCode)
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
    const purchaseResponse = await purchaseResponsePromise
    const purchaseRequest = JSON.parse(purchaseResponse.request().postData() || '{}')
    expect(purchaseRequest.variables?.input?.promoCode).toBe(seed.subscriptionPromoCode)
    const purchasePayload = await purchaseResponse.json()
    expect(purchasePayload.errors, JSON.stringify(purchasePayload.errors ?? [], null, 2)).toBeFalsy()
    expect(purchasePayload.data?.purchaseSubscriptionPlan?.status).toBe('active')

    await userPage.locator('#billing-recharge-amount').fill('2.00')
    await userPage.locator('#billing-recharge-promo').fill(seed.rechargePromoCode)
    const quoteResponsePromise = userPage.waitForResponse((response) => {
      const body = response.request().postData() || ''
      return response.url().includes('/admin/graphql') && body.includes('QuoteRechargePromo') && response.status() === 200
    })
    await userPage.getByRole('button', { name: /^Apply$|应用/i }).click()
    const quotePayload = await quoteResponsePromise.then((response) => response.json())
    expect(quotePayload.errors, JSON.stringify(quotePayload.errors ?? [], null, 2)).toBeFalsy()
    expect(quotePayload.data?.quoteRechargePromo?.payableAmountMicros).toBe(1_500_000)

    const checkoutResponsePromise = userPage.waitForResponse((response) => {
      const body = response.request().postData() || ''
      return response.url().includes('/admin/graphql') && body.includes('CreateMyEPayRechargeCheckout') && response.status() === 200
    })
    const paymentReturnPromise = userPage.waitForURL((url) => url.pathname === '/billing' && url.searchParams.get('trade_status') === 'TRADE_SUCCESS', { timeout: 30000 })
    await userPage.getByRole('button', { name: /Recharge|充值|Pay|支付/i }).click()
    const checkoutResponse = await checkoutResponsePromise
    const checkoutRequest = JSON.parse(checkoutResponse.request().postData() || '{}')
    expect(checkoutRequest.variables?.input?.promoCode).toBe(seed.rechargePromoCode)
    await paymentReturnPromise
    await userPage.goto('/billing', { waitUntil: 'domcontentloaded' })
    await waitForBillingOverview(userPage)
    await expect(userPage.getByText('paid', { exact: true }).first()).toBeVisible({ timeout: 20000 })

    const inviterContext = await browser.newContext()
    const inviterPage = await inviterContext.newPage()
    await injectAuthSession(inviterPage, inviterSession)
    await inviterPage.goto('/billing', { waitUntil: 'domcontentloaded' })
    await waitForBillingOverview(inviterPage)
    const transferButton = inviterPage.getByRole('button', { name: /Transfer available rebates|转出可用返利/i })
    await expect(transferButton).toBeEnabled({ timeout: 20000 })
    const transferResponsePromise = inviterPage.waitForResponse((response) => {
      const body = response.request().postData() || ''
      return response.url().includes('/admin/graphql') && body.includes('TransferAffiliateRebates') && response.status() === 200
    })
    await transferButton.click()
    const transferPayload = await transferResponsePromise.then((response) => response.json())
    expect(transferPayload.errors, JSON.stringify(transferPayload.errors ?? [], null, 2)).toBeFalsy()
    expect(transferPayload.data?.transferAffiliateRebates?.transferredCount).toBe(2)
    expect(transferPayload.data?.transferAffiliateRebates?.transferredMicros).toBe(600_000)

    await inviterContext.close()
    await userContext.close()
  })
})

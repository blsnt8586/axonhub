import { APIRequestContext, Page, expect } from '@playwright/test'

declare const process: {
  env: Record<string, string | undefined>
}

export type AuthUser = {
  id: string
  email: string
  firstName: string
  lastName: string
  isOwner: boolean
  preferLanguage: string
  scopes: string[]
  roles: Array<{ code: string; name: string }>
  projects: Array<{ projectID: string; isOwner: boolean; scopes: string[]; roles: Array<{ code: string; name: string }> }>
}

export type AuthSession = {
  token: string
  user: AuthUser
}

export type CommercialSeed = {
  redeemCode: string
  planId: string
  planName: string
  providerName: string
  rechargePromoCode: string
  subscriptionPromoCode: string
}

const defaultAdminEmail = process.env.AXONHUB_ADMIN_EMAIL || 'my@example.com'
const defaultAdminPassword = process.env.AXONHUB_ADMIN_PASSWORD || 'pwd123456'

export function apiBaseURL() {
  return process.env.AXONHUB_API_URL || 'http://localhost:8099'
}

export function uniqueCommercialSlug(prefix = 'commercial-smoke') {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

export async function signInViaApi(
  request: APIRequestContext,
  credentials: { email?: string; password?: string } = {}
): Promise<AuthSession> {
  const response = await request.post(`${apiBaseURL()}/admin/auth/signin`, {
    data: {
      email: credentials.email || defaultAdminEmail,
      password: credentials.password || defaultAdminPassword,
    },
  })

  expect(response.ok(), await response.text()).toBeTruthy()
  return (await response.json()) as AuthSession
}

export async function enableCommercialRegistration(request: APIRequestContext, adminToken: string) {
  const response = await request.put(`${apiBaseURL()}/admin/system/registration`, {
    headers: authHeaders(adminToken),
    data: {
      enabled: true,
      requireApproval: false,
      createDefaultProject: true,
      createDefaultApiKey: true,
      signupGrantAmount: '12.00',
      defaultProjectName: 'Default Project',
      defaultApiKeyName: 'Default API Key',
      rateLimitWindowSeconds: 60,
      rateLimitMaxAttempts: 20,
    },
  })

  expect(response.ok(), await response.text()).toBeTruthy()
}

export async function registerCommercialUser(
  request: APIRequestContext,
  input: { email: string; password: string; firstName?: string; lastName?: string }
) {
  const response = await request.post(`${apiBaseURL()}/admin/auth/register`, {
    data: input,
  })
  expect(response.ok(), await response.text()).toBeTruthy()
}

export async function graphqlRequest<T>(
  request: APIRequestContext,
  token: string,
  query: string,
  variables?: Record<string, unknown>
): Promise<T> {
  const operationName = query.trim().match(/^(query|mutation)\s+(\w+)/)?.[2]
  const response = await request.post(`${apiBaseURL()}/admin/graphql`, {
    headers: authHeaders(token),
    data: {
      query,
      variables,
      operationName,
    },
  })

  expect(response.ok(), await response.text()).toBeTruthy()
  const payload = await response.json()
  expect(payload.errors, JSON.stringify(payload.errors ?? [], null, 2)).toBeFalsy()
  return payload.data as T
}

export async function seedCommercialAssets(
  request: APIRequestContext,
  adminToken: string,
  slug: string
): Promise<CommercialSeed> {
  const redeem = await graphqlRequest<{
    createRedeemCodes: Array<{ code: string }>
  }>(
    request,
    adminToken,
    `
      mutation CreateRedeemCodes($input: CreateRedeemCodesInput!) {
        createRedeemCodes(input: $input) {
          code
          status
        }
      }
    `,
    {
      input: {
        count: 1,
        type: 'balance',
        amount: '3.00',
        currency: 'CNY',
        prefix: `E2E${slug.replace(/[^a-zA-Z0-9]/g, '').slice(-8).toUpperCase()}`,
        notes: `Playwright smoke redeem ${slug}`,
      },
    }
  )

  const planName = `Smoke Plan ${slug}`
  const plan = await graphqlRequest<{
    saveSubscriptionPlan: { id: string }
  }>(
    request,
    adminToken,
    `
      mutation SaveSubscriptionPlan($input: SaveSubscriptionPlanInput!) {
        saveSubscriptionPlan(input: $input) {
          id
          name
          status
        }
      }
    `,
    {
      input: {
        name: planName,
        description: 'Playwright commercial smoke plan',
        period: 'month',
        periodDays: 30,
        price: '5.00',
        currency: 'CNY',
        includedAmount: '20.00',
        supportedModelIDs: [],
        supportedProjectIDs: [],
        supportedGroupIDs: [],
        allowWalletFallback: true,
        status: 'enabled',
        sortOrder: 10,
      },
    }
  )

  await graphqlRequest(
    request,
    adminToken,
    `
      mutation SaveBillingPriceRule($input: SaveBillingPriceRuleForm!) {
        saveBillingPriceRule(input: $input) {
          id
          enabled
        }
      }
    `,
    {
      input: {
        scopeType: 'global',
        scopeId: 0,
        modelPattern: '*',
        price: {
          items: [
            {
              itemCode: 'prompt_tokens',
              pricing: { mode: 'usage_per_unit', usagePerUnit: '0.01' },
            },
            {
              itemCode: 'completion_tokens',
              pricing: { mode: 'usage_per_unit', usagePerUnit: '0.02' },
            },
          ],
        },
        currency: 'CNY',
        priority: 100,
        enabled: true,
        referenceId: `e2e-${slug}`,
      },
    }
  )

  const promoSuffix = slug.replace(/[^a-zA-Z0-9]/g, '').slice(-10).toUpperCase()
  const rechargePromoCode = `R${promoSuffix}`
  const subscriptionPromoCode = `S${promoSuffix}`
  for (const promo of [
    {
      code: rechargePromoCode,
      discountType: 'amount',
      discountAmount: '0.50',
      discountPercentBps: 0,
      scope: 'recharge',
    },
    {
      code: subscriptionPromoCode,
      discountType: 'percent',
      discountAmount: '0',
      discountPercentBps: 1000,
      scope: 'subscription',
    },
  ]) {
    await graphqlRequest(
      request,
      adminToken,
      `
        mutation SavePromoCode($input: SavePromoCodeInput!) {
          savePromoCode(input: $input) {
            id
            code
            status
          }
        }
      `,
      {
        input: {
          ...promo,
          currency: 'CNY',
          status: 'active',
          maxUses: 20,
          perUserLimit: 1,
          notes: `Playwright commercial promo ${slug}`,
        },
      }
    )
  }

  await graphqlRequest(
    request,
    adminToken,
    `
      mutation SaveAffiliateSetting($input: SaveAffiliateSettingInput!) {
        saveAffiliateSetting(input: $input) {
          id
          enabled
          defaultRebateRateBps
          freezeDays
        }
      }
    `,
    {
      input: {
        enabled: true,
        defaultRebateRateBps: 1000,
        freezeDays: 0,
        minTransferAmount: '0',
        currency: 'CNY',
      },
    }
  )

  const providerName = `Smoke ePay ${slug}`
  await graphqlRequest(
    request,
    adminToken,
    `
      mutation UpsertEPayPaymentProvider($input: UpsertEPayPaymentProviderInput!) {
        upsertEPayPaymentProvider(input: $input) {
          id
          name
          status
        }
      }
    `,
    {
      input: {
        name: providerName,
        status: 'enabled',
        currency: 'CNY',
        gatewayUrl: `${apiBaseURL()}/payment/simulate/epay/submit`,
        pid: '1001',
        key: 'axonhub-smoke-epay-secret',
        notifyUrl: `${apiBaseURL()}/payment/notify/epay`,
        returnUrl: 'http://localhost:9527/billing',
        type: 'alipay',
        siteName: 'AxonHub Playwright Smoke',
      },
    }
  )

  return {
    redeemCode: redeem.createRedeemCodes[0].code,
    planId: plan.saveSubscriptionPlan.id,
    planName,
    providerName,
    rechargePromoCode,
    subscriptionPromoCode,
  }
}

export async function injectAuthSession(page: Page, session: AuthSession) {
  await page.addInitScript(
    ({ token, user }) => {
      window.localStorage.setItem('axonhub_access_token', token)
      window.localStorage.setItem('axonhub_user_info', JSON.stringify(user))
    },
    session
  )
}

export async function waitForBillingOverview(page: Page) {
  await page.waitForResponse(
    (response) => {
      if (!response.url().includes('/admin/graphql')) return false
      const body = response.request().postData() || ''
      return body.includes('MyBillingOverview') && response.status() === 200
    },
    { timeout: 20000 }
  )
}

export type BillingView =
  | 'wallet'
  | 'recharge'
  | 'orders'
  | 'subscriptions'
  | 'redeem'
  | 'affiliate'
  | 'notifications'
  | 'ledger'
  | 'usage'

export async function openBillingView(page: Page, view: BillingView) {
  await page.getByTestId(`billing-view-${view}-tab`).evaluate((button: HTMLButtonElement) => button.click())
  await expect(page).toHaveURL(new RegExp(`[?&]view=${view}(?:&|$)`))
  await expect(page.getByTestId(`billing-${view}-view`)).toBeVisible()
}

function authHeaders(token: string) {
  return {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
  }
}

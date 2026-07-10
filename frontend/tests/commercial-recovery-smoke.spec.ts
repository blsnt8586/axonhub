import { expect, test } from '@playwright/test'
import { graphqlRequest, injectAuthSession, signInViaApi, waitForBillingOverview } from './commercial-smoke.utils'

declare const process: {
  env: Record<string, string | undefined>
}

type RecoveryState = {
  myBillingAccount: {
    balanceMicros: number
    status: string
  }
  myPaymentOrders: {
    edges: Array<{
      node: {
        orderNo: string
        status: string
      }
    }>
  }
  myUserSubscriptions: {
    edges: Array<{
      node: {
        status: string
        plan: {
          name: string
        }
      }
    }>
  }
}

test.describe('commercial recovery browser smoke', () => {
  test('restored user can read wallet, paid orders, and active subscriptions', async ({ page, request }) => {
    const email = process.env.AXONHUB_RECOVERY_USER_EMAIL
    const password = process.env.AXONHUB_RECOVERY_USER_PASSWORD || 'SmokePass123'

    expect(email, 'AXONHUB_RECOVERY_USER_EMAIL must identify the restored smoke user').toBeTruthy()

    const session = await signInViaApi(request, { email, password })
    const state = await graphqlRequest<RecoveryState>(
      request,
      session.token,
      `
        query CommercialRecoveryState {
          myBillingAccount {
            balanceMicros
            status
          }
          myPaymentOrders(first: 20, orderBy: { field: CREATED_AT, direction: DESC }) {
            edges {
              node {
                orderNo
                status
              }
            }
          }
          myUserSubscriptions(first: 20, orderBy: { field: CREATED_AT, direction: DESC }) {
            edges {
              node {
                status
                plan {
                  name
                }
              }
            }
          }
        }
      `
    )

    const paidOrder = state.myPaymentOrders.edges.find(({ node }) => node.status === 'paid')
    const activeSubscription = state.myUserSubscriptions.edges.find(({ node }) => node.status === 'active')

    expect(state.myBillingAccount.status).toBe('active')
    expect(state.myBillingAccount.balanceMicros).toBeGreaterThan(0)
    expect(paidOrder).toBeTruthy()
    expect(activeSubscription).toBeTruthy()

    await injectAuthSession(page, session)
    await page.goto('/billing', { waitUntil: 'domcontentloaded' })
    await waitForBillingOverview(page)

    await expect(page.getByText('paid', { exact: true }).first()).toBeVisible({ timeout: 20000 })
    await expect(page.getByText(activeSubscription!.node.plan.name).first()).toBeVisible({ timeout: 20000 })
  })
})

import { expect, test } from '@playwright/test'
import {
  enableCommercialRegistration,
  graphqlRequest,
  registerCommercialUser,
  signInViaApi,
  uniqueCommercialSlug,
} from './commercial-smoke.utils'

test.describe('commercial user workspace', () => {
  test('new user lands on the self-service workspace and can open consumer routes', async ({ page, request }) => {
    test.setTimeout(60_000)
    const pageErrors: string[] = []
    const consoleErrors: string[] = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    page.on('console', (message) => {
      if (message.type() === 'error') consoleErrors.push(message.text())
    })

    const adminSession = await signInViaApi(request)
    await enableCommercialRegistration(request, adminSession.token)

    const slug = uniqueCommercialSlug('workspace-smoke')
    const email = `${slug}@example.com`
    const password = 'SmokePass123'
    await registerCommercialUser(request, {
      email,
      password,
      firstName: 'Workspace',
      lastName: 'User',
    })

    const session = await signInViaApi(request, { email, password })
    expect(session.user.isOwner).toBe(false)
    expect(session.user.scopes).toEqual([])
    const me = await graphqlRequest<{
      me: {
        isOwner: boolean
        scopes: string[]
        projects: Array<{ projectID: string; isOwner: boolean; scopes: string[] }>
      }
    }>(
      request,
      session.token,
      `
        query WorkspaceUserMe {
          me {
            isOwner
            scopes
            projects {
              projectID
              isOwner
              scopes
            }
          }
        }
      `
    )
    expect(me.me.isOwner).toBe(false)
    expect(me.me.scopes).toEqual([])
    expect(me.me.projects).toHaveLength(1)
    expect(me.me.projects[0]?.isOwner).toBe(true)
    expect(me.me.projects[0]?.scopes).toEqual([])

    await page.goto('/sign-in', { waitUntil: 'domcontentloaded' })
    await page.getByTestId('sign-in-email').fill(email)
    await page.getByTestId('sign-in-password').fill(password)
    const summaryResponsePromise = page.waitForResponse(
      (response) => new URL(response.url()).pathname === '/admin/account/workspace-summary'
    )
    await Promise.all([
      page.waitForURL((url) => url.pathname === '/home', { timeout: 20_000 }),
      page.getByTestId('sign-in-submit').click(),
    ])
    const summaryResponse = await summaryResponsePromise
    const summaryPayload = await summaryResponse.json()
    expect(summaryResponse.ok(), JSON.stringify(summaryPayload, null, 2)).toBeTruthy()
    expect(summaryPayload.onboarding, JSON.stringify(summaryPayload, null, 2)).toBeDefined()

    await page.waitForTimeout(1_000)
    expect(pageErrors).toEqual([])
    expect(consoleErrors).toEqual([])
    expect(page.url()).toContain('/home')
    await expect(page.getByTestId('user-home')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByText(/Access Denied|拒绝访问/i)).toHaveCount(0)
    await expect(page.getByTestId('workspace-api-key-count')).toContainText(/1/)
    await expect(page.getByTestId('workspace-model-count')).toContainText('0')
    await expect(page.getByTestId('workspace-state-model_unavailable')).toBeVisible()
    await expect(page.getByRole('button', { name: /Playground/i })).toBeDisabled()

    for (const href of ['/channels', '/channels/accounts', '/admin/billing', '/users', '/roles', '/system']) {
      await expect(page.locator(`a[href="${href}"]`)).toHaveCount(0)
    }

    await page.goto('/project/api-keys', { waitUntil: 'domcontentloaded' })
    await expect(page.getByText(/Access Denied|拒绝访问/i)).toHaveCount(0)
    await expect(page.getByRole('button', { name: /Create API Key|创建 API Key|新建/i })).toBeVisible({ timeout: 20_000 })

    await page.goto('/project/playground', { waitUntil: 'domcontentloaded' })
    await expect(page.getByText(/Access Denied|拒绝访问/i)).toHaveCount(0)

    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/home', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('user-home')).toBeVisible({ timeout: 20_000 })
    const mobileLayout = await page.evaluate(() => ({
      documentWidth: document.documentElement.scrollWidth,
      viewportWidth: window.innerWidth,
    }))
    expect(mobileLayout.documentWidth).toBeLessThanOrEqual(mobileLayout.viewportWidth)
  })
})

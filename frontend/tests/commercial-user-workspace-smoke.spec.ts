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

test.describe('commercial user workspace', () => {
  test('new user lands on the self-service workspace and can open consumer routes', async ({ browser, page, request }) => {
    test.setTimeout(60_000)
    const pageErrors: string[] = []
    const consoleErrors: string[] = []
    page.on('pageerror', (error) => pageErrors.push(error.message))
    page.on('console', (message) => {
      if (message.type() === 'error') consoleErrors.push(message.text())
    })

    const adminSession = await signInViaApi(request)
    await enableCommercialRegistration(request, adminSession.token)
    const adminContext = await browser.newContext()
    const adminPage = await adminContext.newPage()
    await injectAuthSession(adminPage, adminSession)
    await adminPage.goto('/system', { waitUntil: 'domcontentloaded' })
    await adminPage.getByRole('tab', { name: /Registration|注册/i }).click()
    await expect(adminPage.getByTestId('workspace-policy-settings')).toBeVisible({ timeout: 20_000 })
    await adminContext.close()

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

    const workspaceListResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${session.token}` },
    })
    expect(workspaceListResponse.ok(), await workspaceListResponse.text()).toBeTruthy()
    const workspaceList = await workspaceListResponse.json()
    expect(workspaceList.workspaces).toHaveLength(1)
    expect(workspaceList.workspaces[0].id).toBe(me.me.projects[0]?.projectID)
    expect(workspaceList.workspaces[0].capabilities.consumeAI).toBe(true)
    expect(workspaceList.workspaces[0].capabilities.manageOwnAPIKeys).toBe(true)
    expect(workspaceList.workspaces[0].capabilities.viewOwnUsage).toBe(true)
    expect(workspaceList.workspaces[0].capabilities.manageMembers).toBe(true)

    const blockedCreateResponse = await request.post(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${session.token}` },
      data: { name: 'Blocked Workspace' },
    })
    expect(blockedCreateResponse.status()).toBe(403)

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

    await page.goto('/workspaces', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('workspaces-page')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId(`workspace-${workspaceList.workspaces[0].id}`).getByText('Default Project')).toBeVisible()
    await expect(page.getByTestId('create-workspace')).toBeDisabled()
    await page.evaluate(() => localStorage.setItem('axonhub_selected_project_id', 'gid://axonhub/Project/999999'))
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect.poll(() => page.evaluate(() => localStorage.getItem('axonhub_selected_project_id'))).toBe(me.me.projects[0]?.projectID)

    const enableWorkspaceCreationResponse = await request.put(`${apiBaseURL()}/admin/system/workspaces`, {
      headers: { Authorization: `Bearer ${adminSession.token}` },
      data: { allowSelfServiceCreation: true, maxWorkspacesPerUser: 2 },
    })
    expect(enableWorkspaceCreationResponse.ok(), await enableWorkspaceCreationResponse.text()).toBeTruthy()
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('create-workspace')).toBeEnabled()
    await page.getByTestId('create-workspace').click()
    await page.locator('#workspace-name').fill('Personal Lab')
    await page.locator('#workspace-description').fill('Consumer workspace created by Playwright')
    await page.getByRole('button', { name: /^创建$|^Create$/ }).click()
    await expect(page.locator('[data-slot="card-title"]', { hasText: 'Personal Lab' })).toBeVisible({ timeout: 20_000 })
    await expect.poll(() => page.evaluate(() => localStorage.getItem('axonhub_selected_project_id'))).not.toBe(me.me.projects[0]?.projectID)

    for (const href of ['/channels', '/channels/accounts', '/admin/billing', '/users', '/roles', '/system']) {
      await expect(page.locator(`a[href="${href}"]`)).toHaveCount(0)
    }

    await page.goto('/project/api-keys', { waitUntil: 'domcontentloaded' })
    await expect(page.getByText(/Access Denied|拒绝访问/i)).toHaveCount(0)
    await expect(page.getByRole('button', { name: /Create API Key|创建 API Key|新建/i })).toBeVisible({ timeout: 20_000 })

    await page.goto('/project/playground', { waitUntil: 'domcontentloaded' })
    await expect(page.getByText(/Access Denied|拒绝访问/i)).toHaveCount(0)

    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/workspaces', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('workspaces-page')).toBeVisible({ timeout: 20_000 })
    const mobileLayout = await page.evaluate(() => ({
      documentWidth: document.documentElement.scrollWidth,
      viewportWidth: window.innerWidth,
    }))
    expect(mobileLayout.documentWidth).toBeLessThanOrEqual(mobileLayout.viewportWidth)
  })
})

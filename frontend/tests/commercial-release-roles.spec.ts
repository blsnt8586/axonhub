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

test.describe.configure({ mode: 'serial' })

test.describe('commercial release role matrix', () => {
  test('covers owner, existing member, new user, suspension, no-channel, and route compatibility', async ({ browser, page, request }) => {
    test.setTimeout(120_000)
    const admin = await signInViaApi(request)
    await enableCommercialRegistration(request, admin.token)
    const ownerWorkspacesResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${admin.token}` },
    })
    expect(ownerWorkspacesResponse.ok(), await ownerWorkspacesResponse.text()).toBeTruthy()
    const ownerWorkspaces = await ownerWorkspacesResponse.json()
    const sharedProjectId = ownerWorkspaces.workspaces[0]?.id as string
    expect(sharedProjectId).toBeTruthy()

    await injectAuthSession(page, admin)
    await page.goto('/home', { waitUntil: 'domcontentloaded' })
    await expect(page.getByTestId('sidebar-group-admin')).toBeVisible({ timeout: 20_000 })
    await expect(page.locator('a[href="/admin/billing"]')).toBeVisible()
    await expect(page.locator('a[href="/channels"]')).toBeVisible()

    const slug = uniqueCommercialSlug('release-roles')
    const password = 'SmokePass123'
    const existingEmail = `${slug}-existing@example.com`
    await registerCommercialUser(request, { email: existingEmail, password, firstName: 'Existing', lastName: 'Member' })
    const existing = await signInViaApi(request, { email: existingEmail, password })
    const existingIdentity = await graphqlRequest<{ me: { id: string } }>(request, existing.token, `query ExistingReleaseIdentity { me { id } }`)
    await graphqlRequest(
      request,
      admin.token,
      `
        mutation AddExistingReleaseMember($input: AddUserToProjectInput!) {
          addUserToProject(input: $input) { id projectID userID isOwner scopes }
        }
      `,
      { input: { projectId: sharedProjectId, userId: existingIdentity.me.id, isOwner: false, scopes: [] } }
    )

    const existingWorkspacesResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${existing.token}` },
    })
    expect(existingWorkspacesResponse.ok(), await existingWorkspacesResponse.text()).toBeTruthy()
    const existingWorkspaces = await existingWorkspacesResponse.json()
    const sharedWorkspace = existingWorkspaces.workspaces.find((workspace: { id: string }) => workspace.id === sharedProjectId)
    expect(sharedWorkspace?.isOwner).toBe(false)
    expect(sharedWorkspace?.capabilities.consumeAI).toBe(true)
    expect(sharedWorkspace?.capabilities.manageOwnAPIKeys).toBe(true)
    expect(sharedWorkspace?.capabilities.viewOwnUsage).toBe(true)
    expect(sharedWorkspace?.capabilities.manageMembers).toBe(false)
    expect(existing.user.scopes).toEqual([])

    const memberContext = await browser.newContext()
    const memberPage = await memberContext.newPage()
    await injectAuthSession(memberPage, existing)
    await memberPage.addInitScript((projectId) => localStorage.setItem('axonhub_selected_project_id', projectId), sharedProjectId)
    await memberPage.goto('/home', { waitUntil: 'domcontentloaded' })
    await expect(memberPage.getByTestId('user-home')).toBeVisible({ timeout: 20_000 })
    await expect(memberPage.getByTestId('sidebar-group-workspace')).toBeVisible()
    await expect(memberPage.getByTestId('sidebar-group-project')).toHaveCount(0)
    await expect(memberPage.getByTestId('sidebar-group-admin')).toHaveCount(0)

    await memberPage.goto('/project/requests', { waitUntil: 'domcontentloaded' })
    await expect(memberPage.getByTestId('user-requests-page')).toBeVisible({ timeout: 20_000 })
    await expect(memberPage.getByTestId('user-usage-scope-switch')).toHaveCount(0)
    await memberPage.goto('/billing', { waitUntil: 'domcontentloaded' })
    await expect(memberPage.getByTestId('billing-wallet-view')).toBeVisible({ timeout: 20_000 })
    await memberContext.close()

    const newEmail = `${slug}-new@example.com`
    await registerCommercialUser(request, { email: newEmail, password, firstName: 'New', lastName: 'User' })
    const newUser = await signInViaApi(request, { email: newEmail, password })
    const newContext = await browser.newContext()
    const newPage = await newContext.newPage()
    await injectAuthSession(newPage, newUser)
    await newPage.goto('/home', { waitUntil: 'domcontentloaded' })
    await expect(newPage.getByTestId('workspace-state-model_unavailable')).toBeVisible({ timeout: 20_000 })
    await expect(newPage.getByRole('button', { name: /Playground/i })).toBeDisabled()
    await newPage.goto('/project/playground', { waitUntil: 'domcontentloaded' })
    await expect(newPage.getByTestId('playground-block-state')).toContainText(/No models available|暂无可用模型/i, { timeout: 20_000 })
    await newContext.close()

    const suspendedEmail = `${slug}-suspended@example.com`
    await registerCommercialUser(request, { email: suspendedEmail, password, firstName: 'Suspended', lastName: 'User' })
    const suspended = await signInViaApi(request, { email: suspendedEmail, password })
    const suspendedIdentity = await graphqlRequest<{ me: { id: string } }>(request, suspended.token, `query SuspendedReleaseIdentity { me { id } }`)
    const suspendedWorkspacesResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${suspended.token}` },
    })
    expect(suspendedWorkspacesResponse.ok(), await suspendedWorkspacesResponse.text()).toBeTruthy()
    const suspendedWorkspaces = await suspendedWorkspacesResponse.json()
    const suspendedProjectId = suspendedWorkspaces.workspaces[0]?.id as string
    const keyResponse = await request.post(
      `${apiBaseURL()}/admin/account/api-keys?projectId=${encodeURIComponent(suspendedProjectId)}`,
      {
        headers: { Authorization: `Bearer ${suspended.token}` },
        data: { name: 'Suspension Gate Key', ipAllowlist: [], allowedModelIds: [] },
      }
    )
    expect(keyResponse.ok(), await keyResponse.text()).toBeTruthy()
    const keyPayload = await keyResponse.json()
    expect(keyPayload.secret).toMatch(/^ah-/)

    await graphqlRequest(
      request,
      admin.token,
      `mutation SuspendReleaseUser($id: ID!, $status: UserStatus!) { updateUserStatus(id: $id, status: $status) { id status } }`,
      { id: suspendedIdentity.me.id, status: 'deactivated' }
    )

    const staleJWTResponse = await request.get(`${apiBaseURL()}/admin/account/workspaces`, {
      headers: { Authorization: `Bearer ${suspended.token}` },
    })
    expect(staleJWTResponse.status()).toBe(401)
    const suspendedSignIn = await request.post(`${apiBaseURL()}/admin/auth/signin`, {
      data: { email: suspendedEmail, password },
    })
    expect(suspendedSignIn.status()).toBe(401)
    const suspendedKeyCall = await request.post(`${apiBaseURL()}/v1/chat/completions`, {
      headers: { Authorization: `Bearer ${keyPayload.secret}` },
      data: { model: 'gpt-no-channel', messages: [{ role: 'user', content: 'must be rejected before routing' }] },
    })
    expect(suspendedKeyCall.status()).toBe(401)

    const suspendedContext = await browser.newContext()
    const suspendedPage = await suspendedContext.newPage()
    await injectAuthSession(suspendedPage, suspended)
    await suspendedPage.goto('/home', { waitUntil: 'domcontentloaded' })
    await expect(suspendedPage).toHaveURL(/\/sign-in(?:\?|$)/, { timeout: 20_000 })
    await suspendedContext.close()
  })
})

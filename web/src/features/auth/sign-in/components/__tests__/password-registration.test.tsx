/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { SignUp } from '../../../sign-up'
import { UserAuthForm } from '../user-auth-form'

const state = vi.hoisted(() => ({
  status: {
    password_login_enabled: true,
    discord_oauth: true,
    github_oauth: true,
    register_enabled: true,
    discord_required_for_registration: true,
  },
  login: vi.fn(),
  completeLogin: vi.fn(),
  discord: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('@tanstack/react-router', () => ({
  Link: (props: { children?: ReactNode; to?: string }) => (
    <a href={props.to}>{props.children}</a>
  ),
}))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: state.status }),
}))
vi.mock('@/features/auth/api', () => ({
  login: state.login,
  wechatLoginByCode: vi.fn(),
}))
vi.mock('@/features/auth/hooks/use-auth-redirect', () => ({
  useAuthRedirect: () => ({ handleLoginResult: state.completeLogin }),
}))
vi.mock('@/features/auth/hooks/use-oauth-login', () => ({
  useOAuthLogin: () => ({
    isLoading: false,
    githubButtonDisabled: false,
    handleDiscordLogin: state.discord,
    handleGitHubLogin: vi.fn(),
  }),
}))
vi.mock('@/features/auth/components/recover-by-key-dialog', () => ({
  RecoverByKeyDialog: () => null,
}))
vi.mock('@/features/auth/auth-layout', () => ({
  AuthLayout: (props: { children?: ReactNode }) => props.children,
}))
vi.mock('@/features/auth/components/terms-footer', () => ({
  TermsFooter: () => null,
}))
vi.mock('@/features/auth/components/legal-consent', () => ({
  LegalConsent: () => null,
}))

afterEach(cleanup)
beforeEach(() => {
  vi.clearAllMocks()
  state.status.password_login_enabled = true
  state.completeLogin.mockResolvedValue(false)
})

test('existing customers can submit a password and complete the existing login verification flow', async () => {
  const challenge = { require_verification: true, flow_token: 'verify-next' }
  state.login.mockResolvedValue({ success: true, data: challenge })
  render(<UserAuthForm redirectTo='/profile' />)
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('Username'), 'existing-user')
  await user.type(screen.getByLabelText('Password'), 'existing-password')
  await user.click(
    screen.getByRole('button', { name: 'Sign in with password' })
  )
  await waitFor(() =>
    expect(state.login).toHaveBeenCalledWith({
      username: 'existing-user',
      password: 'existing-password',
      turnstile: '',
      passwordEncryptionEnabled: false,
    })
  )
  await waitFor(() =>
    expect(state.completeLogin).toHaveBeenCalledWith(challenge, '/profile')
  )
  expect(screen.getByLabelText('Password')).toHaveValue('')
})

test('password fields follow the server login setting', () => {
  state.status.password_login_enabled = false
  render(<UserAuthForm />)
  expect(screen.queryByLabelText('Password')).not.toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: /Continue with Discord$/ })
  ).toBeInTheDocument()
})

test('mandatory Discord signup hides other registration providers and explains password setup', async () => {
  render(<SignUp />)
  expect(
    screen.queryByRole('button', { name: /Continue with GitHub$/ })
  ).not.toBeInTheDocument()
  expect(
    screen.getByText(
      'Create your account with Discord, then set a password in Security & Access.'
    )
  ).toBeInTheDocument()
  await userEvent.click(
    screen.getByRole('button', { name: /Sign up with Discord$/ })
  )
  expect(state.discord).toHaveBeenCalledOnce()
})

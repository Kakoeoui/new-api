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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { DirectionProvider } from '@/context/direction-provider'
import { FontProvider } from '@/context/font-provider'
import { api } from '@/lib/api'

import { AuthenticatedLayout } from '../components/authenticated-layout'

let client: QueryClient

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  vi.spyOn(api, 'get').mockImplementation(
    vi.fn(async (url: string) => ({
      data: { success: true, data: url === '/api/notice' ? '' : {} },
    }))
  )
})

afterEach(() => {
  client.clear()
  vi.restoreAllMocks()
  document.cookie = 'sidebar_state=; path=/; max-age=0'
})

async function renderLayout() {
  const route = createRootRoute({
    component: () => <AuthenticatedLayout>Layout content</AuthenticatedLayout>,
  })
  const router = createRouter({
    routeTree: route,
    history: createMemoryHistory({ initialEntries: ['/'] }),
    defaultPendingMs: 0,
    defaultPendingMinMs: 0,
  })
  const view = render(
    <QueryClientProvider client={client}>
      <FontProvider>
        <DirectionProvider>
          <RouterProvider router={router} />
        </DirectionProvider>
      </FontProvider>
    </QueryClientProvider>
  )
  await screen.findByText('Layout content')
  return view.container.querySelector('[data-slot="sidebar"]')
}

test('starts expanded and lets the user collapse it', async () => {
  const sidebar = await renderLayout()
  expect(sidebar).toHaveAttribute('data-state', 'expanded')
  await userEvent.click(
    within(screen.getByRole('banner')).getByRole('button', {
      name: 'Toggle Sidebar',
    })
  )
  expect(sidebar).toHaveAttribute('data-state', 'collapsed')
  expect(document.cookie).toContain('sidebar_state=false')
})

test('honors a saved collapsed sidebar', async () => {
  document.cookie = 'sidebar_state=false; path=/'
  expect(await renderLayout()).toHaveAttribute('data-state', 'collapsed')
})

test('offers the model market in the compact header menu', async () => {
  await renderLayout()
  await userEvent.click(
    within(screen.getByRole('banner')).getByRole('button', {
      name: 'Open menu',
    })
  )
  expect(
    await screen.findByRole('menuitem', { name: 'Model Square' })
  ).toHaveAttribute('href', '/pricing')
})

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
import { act, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { DirectionProvider } from '@/context/direction-provider'
import { FontProvider } from '@/context/font-provider'
import th from '@/i18n/locales/th.json'
import { api } from '@/lib/api'

import { Home } from '..'

let client: QueryClient

beforeEach(() => {
  localStorage.clear()
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(async () => {
  client.clear()
  vi.restoreAllMocks()
  localStorage.clear()
  await act(() => i18next.changeLanguage('en'))
  i18next.removeResourceBundle('th', 'translation')
})

async function renderHome(content = '') {
  vi.spyOn(api, 'get').mockImplementation(
    vi.fn(async (url: string) => {
      let data: unknown = {}
      if (url === '/api/home_page_content') data = content
      if (url === '/api/notice') data = ''
      if (url === '/api/status') data = { system_name: 'New API' }
      return { data: { success: true, data } }
    })
  )
  const router = createRouter({
    routeTree: createRootRoute({ component: Home }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
    defaultPendingMs: 0,
    defaultPendingMinMs: 0,
  })
  render(
    <QueryClientProvider client={client}>
      <FontProvider>
        <DirectionProvider>
          <RouterProvider router={router} />
        </DirectionProvider>
      </FontProvider>
    </QueryClientProvider>
  )
}

test('the default homepage uses the supplied RVL content and working destinations', async () => {
  await renderHome()
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent(
    'Your API model shop'
  )
  expect(screen.getByRole('link', { name: 'Buy a key' })).toHaveAttribute(
    'href',
    'https://rvlconnect.com/'
  )
  expect(
    screen.getByRole('link', { name: 'Browse available models' })
  ).toHaveAttribute('href', '/pricing')
  expect(screen.getByText('https://shop.rvlconnect.com/v1')).toBeVisible()
  expect(screen.getAllByText('RVL Connect').length).toBeGreaterThan(0)
  expect(document.querySelector('.rvl-home-page')).not.toHaveClass('dark')
  expect(document.querySelector('.crest img')).toBeNull()
  expect(
    screen.getByRole('note', { name: 'About Gemini 4 Argon' })
  ).toBeVisible()
  expect(
    screen.getByRole('link', { name: /Open status page/ })
  ).toHaveAttribute('rel', 'noopener noreferrer')
})

test('the landing page displays the reference Thai copy when Thai is selected', async () => {
  i18next.addResourceBundle('th', 'translation', th.translation)
  await act(() => i18next.changeLanguage('th'))
  await renderHome()
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent(
    'ร้านรวมโมเดล API'
  )
  expect(screen.getByRole('link', { name: 'ซื้อคีย์' })).toBeVisible()
  expect(
    screen.getByRole('heading', { name: 'วิธีซื้อและใช้งาน (วิดีโอ)' })
  ).toBeVisible()
  expect(
    screen.getByRole('heading', { name: 'เริ่มใช้งานใน 3 ขั้นตอน' })
  ).toBeVisible()
})

test('legacy HTML from a restored database cannot replace the shared RVL homepage', async () => {
  await renderHome('<h1>Legacy customer homepage</h1>')
  expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent(
    'Your API model shop'
  )
  expect(screen.getByRole('link', { name: 'Buy a key' })).toBeVisible()
  expect(screen.queryByText('Legacy customer homepage')).not.toBeInTheDocument()
})

test('merges the customer usage video into the shared homepage with playback controls and alternatives', async () => {
  await renderHome()
  const videoUrl = 'https://rvlconnect.com/assets/Clip_man_rvl_shop.mp4'
  const video = await screen.findByTitle('RVL Connect usage video')
  expect(video).toHaveAttribute('controls')
  expect(video).toHaveAttribute('playsinline')
  expect(video).toHaveAttribute('preload', 'metadata')
  expect(video).not.toHaveAttribute('autoplay')
  expect(video.querySelector('source')).toHaveAttribute('src', videoUrl)
  expect(
    screen.getByRole('link', { name: /Open video in full screen/ })
  ).toHaveAttribute('href', videoUrl)
  expect(
    screen.getByRole('link', { name: 'Read the written guide' })
  ).toHaveAttribute('href', 'https://rvlconnect.com/blog/manual-guide-shop-api')
  const videoHeading = screen.getByRole('heading', {
    name: 'How to buy and use (video)',
  })
  const stepsHeading = screen.getByRole('heading', {
    name: 'Get started in 3 steps',
  })
  expect(
    videoHeading.compareDocumentPosition(stepsHeading) &
      Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
})

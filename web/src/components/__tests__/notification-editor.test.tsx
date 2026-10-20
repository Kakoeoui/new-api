import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  expect,
  test,
  vi,
} from 'vitest'

import { NotificationEditor } from '@/components/notification-editor'
import { NotificationPopover } from '@/components/notification-popover'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

let client: QueryClient
const animationsDescriptor = Object.getOwnPropertyDescriptor(
  HTMLElement.prototype,
  'getAnimations'
)
beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, 'getAnimations', {
    configurable: true,
    value: () => [],
  })
})
afterAll(() => {
  if (animationsDescriptor) {
    Object.defineProperty(
      HTMLElement.prototype,
      'getAnimations',
      animationsDescriptor
    )
  } else {
    Reflect.deleteProperty(HTMLElement.prototype, 'getAnimations')
  }
})
const content = {
  notice: 'Existing notice',
  announcements: [
    {
      id: 42,
      content: 'Existing timeline',
      publishDate: '2026-10-20T12:00:00Z',
      type: 'warning',
      extra: 'Keep note',
    },
  ],
  announcements_enabled: true,
}
beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: content },
  })
})
afterEach(() => {
  client.clear()
  useAuthStore.getState().auth.setUser(null)
  vi.restoreAllMocks()
})

test.each([1, 10, 100])(
  'bell editor visibility follows role %i',
  async (role) => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'test', role })
    render(
      <QueryClientProvider client={client}>
        <NotificationPopover
          open
          onOpenChange={vi.fn()}
          unreadCount={0}
          activeTab='notice'
          onTabChange={vi.fn()}
          notice='Welcome'
          announcements={[]}
          loading={false}
        />
      </QueryClientProvider>
    )
    const button = screen.queryByRole('button', {
      name: 'Edit announcements and timeline',
    })
    if (role < 10) {
      expect(button).not.toBeInTheDocument()
      return
    }
    await waitFor(() => expect(button).toBeVisible())
    if (!button) throw new Error('Missing admin edit button')
    fireEvent.click(button)
    expect(
      await screen.findByRole('dialog', {
        name: 'Edit announcements and timeline',
      })
    ).toBeVisible()
    expect(await screen.findByDisplayValue('Existing notice')).toBeVisible()
  }
)

test('saving updates the existing notice and timeline and refreshes the bell readers', async () => {
  const changed = vi.fn()
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const invalidate = vi.spyOn(client, 'invalidateQueries')
  render(
    <QueryClientProvider client={client}>
      <NotificationEditor open onOpenChange={changed} />
    </QueryClientProvider>
  )
  const notice = await screen.findByDisplayValue('Existing notice')
  fireEvent.change(notice, { target: { value: 'Updated notice' } })
  fireEvent.click(screen.getByRole('button', { name: 'Add timeline entry' }))
  fireEvent.change(screen.getAllByLabelText('Content')[1], {
    target: { value: 'New timeline' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(put).toHaveBeenCalledOnce())
  expect(put.mock.calls[0][0]).toBe('/api/notification-content')
  expect(put.mock.calls[0][1]).toMatchObject({
    notice: 'Updated notice',
    announcements: [content.announcements[0], { content: 'New timeline' }],
  })
  expect(JSON.stringify(put.mock.calls[0][1])).not.toContain('editId')
  await waitFor(() => expect(changed).toHaveBeenCalledWith(false))
  expect(invalidate).toHaveBeenCalledWith({ queryKey: ['notice'] })
  expect(invalidate).toHaveBeenCalledWith({ queryKey: ['status'] })
})

test('failed save retains the editable draft', async () => {
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: false } })
  render(
    <QueryClientProvider client={client}>
      <NotificationEditor open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
  await screen.findByDisplayValue('Existing notice')
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Unable to save notifications'
  )
  expect(screen.getByDisplayValue('Existing notice')).toBeVisible()
})

test('adds a timeline entry when crypto.randomUUID is unavailable', async () => {
  Object.defineProperty(globalThis.crypto, 'randomUUID', {
    configurable: true,
    value: undefined,
  })
  render(
    <QueryClientProvider client={client}>
      <NotificationEditor open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
  await screen.findByDisplayValue('Existing notice')
  fireEvent.click(screen.getByRole('button', { name: 'Add timeline entry' }))
  expect(screen.getAllByLabelText('Content')).toHaveLength(2)
})

test('omits an empty new timeline entry from the save payload', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  render(
    <QueryClientProvider client={client}>
      <NotificationEditor open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
  await screen.findByDisplayValue('Existing notice')
  fireEvent.click(screen.getByRole('button', { name: 'Add timeline entry' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(put).toHaveBeenCalledOnce())
  expect(put.mock.calls[0][1]).toMatchObject({
    announcements: [content.announcements[0]],
  })
})

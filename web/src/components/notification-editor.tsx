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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { NativeSelect } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import dayjs from '@/lib/dayjs'
import { requireServerSuccess } from '@/lib/server-error-message'
import { STATUS_QUERY_KEY } from '@/lib/status-query'

type Announcement = {
  id?: number | string
  content: string
  extra?: string
  publishDate: string
  type?: string
}
type Content = {
  notice: string
  announcements: Announcement[]
  announcements_enabled: boolean
}
type Draft = Omit<Content, 'announcements'> & {
  announcements: (Announcement & { editId: string })[]
}

const MAX_ANNOUNCEMENTS = 100

function createDraftId(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') {
    return globalThis.crypto.randomUUID()
  }
  if (typeof globalThis.crypto?.getRandomValues === 'function') {
    const bytes = new Uint8Array(16)
    globalThis.crypto.getRandomValues(bytes)
    bytes[6] = (bytes[6] & 0x0f) | 0x40
    bytes[8] = (bytes[8] & 0x3f) | 0x80
    const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  }
  return `draft-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

export function NotificationEditor({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [draft, setDraft] = useState<Draft | null>(null)
  const query = useQuery({
    queryKey: ['notification-content'],
    enabled: open,
    queryFn: async () =>
      requireServerSuccess(
        (
          await api.get<{ success: boolean; data: Content }>(
            '/api/notification-content'
          )
        ).data
      ).data,
  })
  const save = useMutation({
    mutationFn: async (value: Draft) => {
      const content: Content = {
        ...value,
        announcements: value.announcements
          .map(({ editId: _, ...entry }) => ({
            ...entry,
            content: entry.content.trim(),
            extra: entry.extra?.trim() || undefined,
          }))
          .filter((entry) => entry.content.length > 0)
          .slice(0, MAX_ANNOUNCEMENTS),
      }
      return requireServerSuccess(
        (await api.put('/api/notification-content', content)).data
      )
    },
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ['notice'] }),
        client.invalidateQueries({ queryKey: STATUS_QUERY_KEY }),
      ])
      onOpenChange(false)
    },
  })
  const { reset } = save
  useEffect(() => {
    if (!open) {
      setDraft(null)
      reset()
      return
    }
    if (!draft && query.data && !query.isFetching) {
      setDraft({
        ...query.data,
        announcements: query.data.announcements.map((entry) => ({
          ...entry,
          editId: createDraftId(),
        })),
      })
    }
  }, [open, query.data, query.isFetching, draft, reset])

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!save.isPending) onOpenChange(next)
      }}
    >
      <DialogContent className='max-h-[85svh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Edit announcements and timeline')}</DialogTitle>
        </DialogHeader>
        {query.isError && (
          <p role='alert'>{t('Unable to load notifications')}</p>
        )}
        {!draft && !query.isError && <p>{t('Loading...')}</p>}
        {draft && (
          <form
            className='space-y-6'
            onSubmit={(event) => {
              event.preventDefault()
              save.mutate(draft)
            }}
          >
            <fieldset disabled={save.isPending} className='space-y-6'>
              <label className='block space-y-2 text-sm'>
                <span className='font-semibold'>{t('Notice')}</span>
                <Textarea
                  value={draft.notice}
                  maxLength={50000}
                  onChange={(event) =>
                    setDraft({ ...draft, notice: event.target.value })
                  }
                />
              </label>
              <section className='space-y-4'>
                <h3 className='font-semibold'>{t('Timeline')}</h3>
                <label className='flex items-center gap-2 text-sm'>
                  <Checkbox
                    checked={draft.announcements_enabled}
                    onCheckedChange={(checked) =>
                      setDraft({ ...draft, announcements_enabled: checked })
                    }
                  />
                  {t('Show timeline')}
                </label>
                {draft.announcements.map((entry, index) => {
                  const update = (patch: Partial<Announcement>) =>
                    setDraft({
                      ...draft,
                      announcements: draft.announcements.map((item, i) =>
                        i === index ? { ...item, ...patch } : item
                      ),
                    })
                  return (
                    <div
                      key={entry.editId}
                      className='space-y-3 rounded-xl border p-4'
                    >
                      <label className='block space-y-1 text-sm'>
                        <span>{t('Content')}</span>
                        <Textarea
                          required
                          maxLength={500}
                          value={entry.content}
                          onChange={(event) =>
                            update({ content: event.target.value })
                          }
                        />
                      </label>
                      <label className='block space-y-1 text-sm'>
                        <span>{t('Date')}</span>
                        <Input
                          type='datetime-local'
                          required
                          value={dayjs(entry.publishDate).format(
                            'YYYY-MM-DDTHH:mm'
                          )}
                          onChange={(event) => {
                            if (event.target.value) {
                              update({
                                publishDate: new Date(
                                  event.target.value
                                ).toISOString(),
                              })
                            }
                          }}
                        />
                      </label>
                      <label className='block space-y-1 text-sm'>
                        <span>{t('Type')}</span>
                        <NativeSelect
                          value={entry.type ?? 'default'}
                          onChange={(event) =>
                            update({ type: event.target.value })
                          }
                        >
                          <option value='default'>{t('Default')}</option>
                          <option value='ongoing'>{t('Ongoing')}</option>
                          <option value='success'>{t('Success')}</option>
                          <option value='warning'>{t('Warning')}</option>
                          <option value='error'>{t('Error')}</option>
                        </NativeSelect>
                      </label>
                      <label className='block space-y-1 text-sm'>
                        <span>{t('Note')}</span>
                        <Input
                          maxLength={100}
                          value={entry.extra ?? ''}
                          onChange={(event) =>
                            update({ extra: event.target.value })
                          }
                        />
                      </label>
                      <Button
                        type='button'
                        variant='outline'
                        onClick={() =>
                          setDraft({
                            ...draft,
                            announcements: draft.announcements.filter(
                              (item) => item.editId !== entry.editId
                            ),
                          })
                        }
                      >
                        {t('Remove')}
                      </Button>
                    </div>
                  )
                })}
                <Button
                  type='button'
                  variant='outline'
                  disabled={draft.announcements.length >= MAX_ANNOUNCEMENTS}
                  onClick={() =>
                    setDraft({
                      ...draft,
                      announcements: [
                        ...draft.announcements,
                        {
                          id: Date.now(),
                          editId: createDraftId(),
                          content: '',
                          publishDate: new Date().toISOString(),
                          type: 'default',
                        },
                      ],
                    })
                  }
                >
                  {t('Add timeline entry')}
                </Button>
              </section>
            </fieldset>
            {save.isError && (
              <p role='alert' className='text-destructive'>
                {t('Unable to save notifications')}
              </p>
            )}
            <div className='flex justify-end gap-2'>
              <Button
                type='button'
                variant='outline'
                disabled={save.isPending}
                onClick={() => onOpenChange(false)}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={save.isPending}>
                {t('Save')}
              </Button>
            </div>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}

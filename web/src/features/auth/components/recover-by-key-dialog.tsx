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
import { KeyRound, Loader2 } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import type { LoginResult } from '@/features/auth/types'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { createServerError } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

interface RecoverByKeyResponse {
  success: boolean
  message?: string
  data?: LoginResult
}

export interface RecoverByKeyDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  redirectTo?: string
  onRecovered?: () => void
}

export function RecoverByKeyDialog({
  open,
  onOpenChange,
  redirectTo,
  onRecovered,
}: RecoverByKeyDialogProps) {
  const { t } = useTranslation()
  const [username, setUsername] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const { handleLoginResult } = useAuthRedirect()
  const authUser = useAuthStore((s) => s.auth.user) as
    | { id?: number; discord_id?: string }
    | null
    | undefined

  const hasActiveDiscordSession = Boolean(authUser?.id && authUser?.discord_id)

  const handleDialogChange = (nextOpen: boolean) => {
    if (isSubmitting && !nextOpen) return
    onOpenChange(nextOpen)
    if (!nextOpen) {
      setUsername('')
      setApiKey('')
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const trimmedUsername = username.trim()
    const trimmedApiKey = apiKey.trim()

    if (!trimmedUsername || !trimmedApiKey) {
      toast.error(t('Please enter both your username and API key'))
      return
    }

    setIsSubmitting(true)
    try {
      const res = await api.post<RecoverByKeyResponse>(
        '/api/user/recover-by-key',
        {
          username: trimmedUsername,
          api_key: trimmedApiKey,
        }
      )

      if (res.data?.success && res.data?.data) {
        const completed = await handleLoginResult(res.data.data, redirectTo)
        setUsername('')
        setApiKey('')
        onOpenChange(false)
        if (completed) {
          toast.success(t('Account recovered and linked successfully!'))
        }
        onRecovered?.()
      } else {
        handleServerError(
          createServerError(
            res.data ?? { success: false },
            t('Account recovery failed')
          )
        )
      }
    } catch (error: unknown) {
      handleServerError(error, t('Account recovery failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleDialogChange}
      title={t('Recover / Migrate Account with API Key')}
      description={
        hasActiveDiscordSession
          ? t(
              'Enter your previous username and a valid API key to transfer all quota and keys to your current Discord account immediately.'
            )
          : t(
              'Enter your previous username and a valid API key to sign in to your existing account (we recommend signing in with Discord first to link your account permanently).'
            )
      }
      contentClassName='sm:max-w-md'
      headerClassName='text-left'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleDialogChange(false)}
            disabled={isSubmitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='recover-by-key-form'
            disabled={isSubmitting || !username.trim() || !apiKey.trim()}
            className='gap-2'
          >
            {isSubmitting ? (
              <Loader2 className='h-4 w-4 animate-spin' />
            ) : (
              <KeyRound className='h-4 w-4' />
            )}
            {t('Confirm Account Recovery')}
          </Button>
        </>
      }
    >
      <form
        id='recover-by-key-form'
        onSubmit={handleSubmit}
        className='space-y-4'
      >
        <div className='grid gap-2'>
          <Label htmlFor='recover-username'>{t('Previous Username')}</Label>
          <Input
            id='recover-username'
            placeholder={t('e.g. my_old_username')}
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            autoComplete='username'
            disabled={isSubmitting}
          />
        </div>

        <div className='grid gap-2'>
          <Label htmlFor='recover-api-key'>
            {t('Previous API Key (sk-...)')}
          </Label>
          <PasswordInput
            id='recover-api-key'
            placeholder='sk-...'
            value={apiKey}
            onChange={(event) => setApiKey(event.target.value)}
            autoComplete='off'
            disabled={isSubmitting}
          />
          <p className='text-muted-foreground text-xs'>
            {t('You can enter the key with or without the sk- prefix.')}
          </p>
        </div>
      </form>
    </Dialog>
  )
}

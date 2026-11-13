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
import { Link } from '@tanstack/react-router'
import { KeyRound } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { LegalConsent } from '@/features/auth/components/legal-consent'
import { OAuthProviders } from '@/features/auth/components/oauth-providers'
import { RecoverByKeyDialog } from '@/features/auth/components/recover-by-key-dialog'
import { useStatus } from '@/hooks/use-status'

import { AuthLayout } from '../auth-layout'
import { TermsFooter } from '../components/terms-footer'

export function SignUp() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [agreedToLegal, setAgreedToLegal] = useState(false)
  const [isRecoverDialogOpen, setIsRecoverDialogOpen] = useState(false)

  const hasUserAgreement = Boolean(status?.user_agreement_enabled)
  const hasPrivacyPolicy = Boolean(status?.privacy_policy_enabled)
  const requiresLegalConsent = hasUserAgreement || hasPrivacyPolicy

  useEffect(() => {
    if (requiresLegalConsent) {
      setAgreedToLegal(false)
    } else {
      setAgreedToLegal(true)
    }
  }, [requiresLegalConsent])

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-2'>
          <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
            {t('Create an account')}
          </h2>
          <p className='text-muted-foreground text-left text-sm sm:text-base'>
            {t('Already have an account?')}{' '}
            <Link
              to='/sign-in'
              className='hover:text-primary font-medium underline underline-offset-4'
            >
              {t('Sign in')}
            </Link>
            .
          </p>
        </div>

        <div className='grid gap-4'>
          {status?.discord_required_for_registration && (
            <p className='text-muted-foreground text-sm'>
              {t(
                'Create your account with Discord, then set a password in Security & Access.'
              )}
            </p>
          )}
          <OAuthProviders
            status={status}
            discordOnly={status?.discord_required_for_registration === true}
            redirectTo={
              status?.discord_required_for_registration
                ? '/security'
                : undefined
            }
            discordLabel={t('Sign up with Discord')}
            disabled={
              status?.register_enabled === false ||
              (requiresLegalConsent && !agreedToLegal)
            }
          />

          <div className='relative my-1'>
            <div className='absolute inset-0 flex items-center'>
              <span className='w-full border-t' />
            </div>
            <div className='relative flex justify-center text-xs uppercase'>
              <span className='bg-background text-muted-foreground px-2'>
                {t('Or if you already have an account')}
              </span>
            </div>
          </div>

          <Button
            type='button'
            variant='outline'
            onClick={() => setIsRecoverDialogOpen(true)}
            className='h-11 w-full justify-center gap-2 rounded-lg'
          >
            <KeyRound className='h-4 w-4' />
            {t('Recover / Migrate Account with API Key')}
          </Button>

          <LegalConsent
            status={status}
            checked={agreedToLegal}
            onCheckedChange={setAgreedToLegal}
            className='mt-1'
          />
        </div>

        <RecoverByKeyDialog
          open={isRecoverDialogOpen}
          onOpenChange={setIsRecoverDialogOpen}
        />

        <TermsFooter
          variant='sign-up'
          status={status}
          className='text-center'
        />
      </div>
    </AuthLayout>
  )
}

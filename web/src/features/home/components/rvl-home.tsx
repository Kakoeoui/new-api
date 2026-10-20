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
import { PawPrint } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'

import '../rvl-home.css'

export function RvlHome() {
  const { t, i18n } = useTranslation()
  const baseUrl = 'https://shop.rvlconnect.com/v1'
  const guideVideoUrl = 'https://rvlconnect.com/assets/Clip_man_rvl_shop.mp4'

  return (
    <div className='rvl-home-page'>
      <PublicLayout showMainContainer={false}>
        <main
          className='rvl'
          lang={toIntlLocale(i18n.resolvedLanguage || i18n.language)}
        >
          <div className='salon-top' aria-hidden='true'>
            <PawPrint size={26} />
          </div>
          <div className='hero'>
            <div className='hero-copy'>
              <div className='brand'>
                RVL<span>Connect</span>
              </div>
              <h1>
                {t('Your API model shop')}
                <br />
                {t('Ready to get started')}
              </h1>
              <p>
                {t('Buy a key, set up your app, and choose a model.')}
                <br />
                {t('Our support team is here on Discord when you need help.')}
              </p>
              <div className='actions'>
                <Button
                  role='link'
                  className='btn'
                  render={<a href='https://rvlconnect.com/' />}
                >
                  {t('Buy a key')}
                </Button>
                <Button
                  role='link'
                  className='btn ghost'
                  render={<a href='https://rvlconnect.com/check.html' />}
                >
                  {t('Check your key')}
                </Button>
                <a className='text-link' href='https://discord.gg/rvlconnect'>
                  {t('Join Discord')}
                </a>
              </div>
            </div>
            <div className='crest' aria-hidden='true'>
              <span className='crest-caption'>The house of RVL</span>
            </div>
          </div>
          <div className='ribbon' aria-hidden='true'>
            <span>API, with character.</span>
            <PawPrint size={22} aria-hidden='true' />
          </div>

          <section className='section' aria-labelledby='rvl-video-title'>
            <div className='section-head'>
              <h2 id='rvl-video-title'>{t('How to buy and use (video)')}</h2>
              <a
                className='text-link'
                href={guideVideoUrl}
                target='_blank'
                rel='noopener noreferrer'
              >
                {t('Open video in full screen')} ↗
              </a>
            </div>
            <video
              className='guide-video'
              controls
              playsInline
              preload='metadata'
              title={t('RVL Connect usage video')}
              aria-describedby='rvl-video-alternative'
            >
              <source src={guideVideoUrl} type='video/mp4' />
              <a href={guideVideoUrl}>{t('Download video')}</a>
            </video>
            <p id='rvl-video-alternative' className='video-alternative'>
              <a href={guideVideoUrl} download>
                {t('Download video')}
              </a>
              <a href='https://rvlconnect.com/blog/manual-guide-shop-api'>
                {t('Read the written guide')}
              </a>
            </p>
          </section>

          <section className='section' aria-labelledby='rvl-start'>
            <div className='section-head'>
              <h2 id='rvl-start'>{t('Get started in 3 steps')}</h2>
              <Link className='text-link' to='/pricing'>
                {t('Browse available models')}
              </Link>
            </div>
            <ol className='steps' role='list'>
              <li className='step'>
                <span className='step-number' aria-hidden='true'>
                  01
                  <PawPrint size={22} />
                </span>
                <h3>{t('Buy a key from the shop')}</h3>
                <p>
                  {t(
                    'Choose a key, transfer payment, and submit your receipt. Receive your key in about a minute.'
                  )}
                </p>
              </li>
              <li className='step'>
                <span className='step-number' aria-hidden='true'>
                  02
                  <PawPrint size={22} />
                </span>
                <h3>{t('Set up your app')}</h3>
                <p>{t('Enter your key and the Base URL below.')}</p>
                <code>{baseUrl}</code>
              </li>
              <li className='step'>
                <span className='step-number' aria-hidden='true'>
                  03
                  <PawPrint size={22} />
                </span>
                <h3>{t('Choose a model and start')}</h3>
                <p>
                  {t(
                    'Browse the available models, then select one in your app.'
                  )}
                </p>
              </li>
            </ol>
          </section>

          <section className='section' aria-labelledby='rvl-guidelines'>
            <div className='section-head'>
              <h2 id='rvl-guidelines'>
                {t('A few things to know before you start')}
              </h2>
            </div>
            <div className='guidelines'>
              <div className='guideline'>
                <h3>{t('Your key is shown only once')}</h3>
                <p>
                  {t(
                    'Copy and save it immediately. You cannot view it again after closing the page.'
                  )}
                </p>
              </div>
              <div className='guideline'>
                <h3>{t('Keep your key private')}</h3>
                <p>
                  {t(
                    'Anyone with your key can spend your balance. Contact an admin immediately if it leaks.'
                  )}
                </p>
              </div>
              <div className='guideline'>
                <h3>{t('Use a receipt less than 3 hours old')}</h3>
                <p>
                  {t(
                    'Transfer at least the required amount for the system to approve your payment.'
                  )}
                </p>
              </div>
              <div className='guideline'>
                <h3>{t('Transferring with Bangkok Bank')}</h3>
                <p>
                  {t(
                    'Wait 5 minutes after transferring before submitting your receipt.'
                  )}
                </p>
              </div>
              <div className='guideline'>
                <h3>{t('Check your balance anytime')}</h3>
                <p>
                  {t('View your balance and expiry date on the key checker.')}{' '}
                  <a href='https://rvlconnect.com/check.html'>
                    {t('Check your key')}
                  </a>
                </p>
              </div>
              <div className='guideline'>
                <h3>{t('Need help? Talk to us')}</h3>
                <p>
                  {t('Ask questions or report issues through')}{' '}
                  <a href='https://discord.gg/rvlconnect'>
                    {t('RVL Connect Discord')}
                  </a>
                </p>
              </div>
            </div>
          </section>

          <section
            className='section status-strip'
            aria-labelledby='rvl-status'
          >
            <div className='section-head'>
              <h2 id='rvl-status'>{t('API channel status')}</h2>
              <a
                className='text-link'
                href='https://rvlconnect.com/status.html'
                target='_blank'
                rel='noopener noreferrer'
              >
                {t('Open status page')} ↗
              </a>
            </div>
          </section>
          <section className='section manual' aria-labelledby='rvl-manual'>
            <div>
              <h2 id='rvl-manual'>{t('Use your key with SillyTavern')}</h2>
              <p>{t('Read the setup and usage instructions in our guide.')}</p>
            </div>
            <Button
              role='link'
              className='btn ghost'
              render={
                <a href='https://rvlconnect.com/blog/manual-guide-shop-api' />
              }
            >
              {t('Read the SillyTavern guide')}
            </Button>
          </section>
          <div className='footer'>
            <span className='footer-brand'>RVL Connect</span>
            <a className='text-link' href='https://discord.gg/rvlconnect'>
              {t('Contact support')}
            </a>
          </div>
        </main>
        <Footer />
      </PublicLayout>
    </div>
  )
}

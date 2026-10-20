import { createInstance } from 'i18next'
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
import { describe, expect, test } from 'vitest'

import {
  convertDetectedLanguage,
  normalizeInterfaceLanguage,
  toIntlLocale,
} from './languages'
import en from './locales/en.json'
import th from './locales/th.json'

describe('convertDetectedLanguage', () => {
  test('maps browser BCP-47 Chinese tags onto interface codes', () => {
    expect(convertDetectedLanguage('zh-TW')).toBe('zhTW')
    expect(convertDetectedLanguage('zh-HK')).toBe('zhTW')
    expect(convertDetectedLanguage('zh-MO')).toBe('zhTW')
    expect(convertDetectedLanguage('zh-Hant-TW')).toBe('zhTW')
    expect(convertDetectedLanguage('zh')).toBe('zhCN')
    expect(convertDetectedLanguage('zh-CN')).toBe('zhCN')
    expect(convertDetectedLanguage('zh-Hans')).toBe('zhCN')
  })

  test('keeps already-normalized interface codes stable (localStorage round-trip)', () => {
    // i18next caches `zhTW`/`zhCN` (the supportedLngs codes) to localStorage,
    // and the detector runs this converter on the cached value at every page
    // load — if `zhTW` does not survive the round-trip, a user who picked
    // Traditional Chinese is flipped to Simplified on the next load and the
    // cache is overwritten, making the flip permanent.
    expect(convertDetectedLanguage('zhTW')).toBe('zhTW')
    expect(convertDetectedLanguage('zhCN')).toBe('zhCN')
  })

  test('passes non-Chinese values through unchanged', () => {
    expect(convertDetectedLanguage('en')).toBe('en')
    expect(convertDetectedLanguage('fr-FR')).toBe('fr-FR')
    expect(convertDetectedLanguage('ja')).toBe('ja')
  })
})

describe('Thai interface language', () => {
  test.each(['th', 'th-TH', 'th_TH'])('normalizes %s', (language) => {
    expect(normalizeInterfaceLanguage(language)).toBe('th')
    expect(convertDetectedLanguage(language)).toBe('th')
  })

  test.each([
    ['zhCN', 'zh-CN'],
    ['zhTW', 'zh-TW'],
    ['en', 'en'],
    ['fr', 'fr'],
    ['ru', 'ru'],
    ['ja', 'ja'],
    ['vi', 'vi'],
    ['th', 'th'],
    ['not_a_locale', undefined],
  ])('formats %s with a valid Intl locale', (language, expected) => {
    expect(toIntlLocale(language)).toBe(expected)
  })

  test('uses Thai translations and preserves interpolation variables', () => {
    expect(th.translation.Home).toBe('หน้าแรก')
    for (const [key, value] of Object.entries(th.translation)) {
      expect(key in en.translation, key).toBe(true)
      const placeholders = key.match(/\{\{[^}]+\}\}/g) ?? []
      for (const placeholder of placeholders) {
        expect(value, key).toContain(placeholder)
      }
    }
  })

  test('switches between Thai and English with fallback for untranslated keys', async () => {
    const instance = createInstance()
    await instance.init({
      resources: { en, th },
      lng: convertDetectedLanguage('th-TH'),
      fallbackLng: 'en',
    })
    expect(instance.t('Home')).toBe('หน้าแรก')
    expect(instance.t('Channel priorities saved')).toBe(
      'Channel priorities saved'
    )
    await instance.changeLanguage('en')
    expect(instance.t('Home')).toBe('Home')
    await instance.changeLanguage('th')
    expect(instance.t('Home')).toBe('หน้าแรก')
  })
})

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
import { expect, test } from 'vitest'

import { parseDiscordCheckinAuthorizationUrl } from '../discord-checkin'

test('only follows Discord authorization URLs for check-in consent retries', () => {
  const url =
    'https://discord.com/oauth2/authorize?state=one-time&prompt=consent'
  expect(parseDiscordCheckinAuthorizationUrl(url)).toBe(url)
  for (const value of [
    'https://example.com/oauth2/authorize',
    'http://discord.com/oauth2/authorize',
    'https://discord.com/other',
    'https://discord.com.evil.test/oauth2/authorize',
    'https://user:secret@discord.com/oauth2/authorize',
    'javascript:alert(1)',
    '/oauth2/authorize',
  ]) {
    expect(() => parseDiscordCheckinAuthorizationUrl(value)).toThrow()
  }
})

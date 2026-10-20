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
import { DEFAULT_LOGO, DEFAULT_SYSTEM_NAME } from './constants'

export const SITE_NAME = 'RVL Connect'
export const SITE_LOGO = '/rvl-logo.png'

// The backend and older browser caches may still contain the upstream defaults.
// Explicit custom branding from the admin continues to take precedence.
export function resolveSiteBrand(name?: unknown, logo?: unknown) {
  const systemName = typeof name === 'string' ? name.trim() : ''
  const logoUrl = typeof logo === 'string' ? logo.trim() : ''
  const stockLogo =
    !logoUrl ||
    logoUrl === DEFAULT_LOGO ||
    (typeof window !== 'undefined' &&
      logoUrl === `${window.location.origin}${DEFAULT_LOGO}`)

  return {
    systemName:
      !systemName ||
      systemName.replaceAll(/[\s-]/g, '').toLowerCase() ===
        DEFAULT_SYSTEM_NAME.replaceAll(/\s/g, '').toLowerCase()
        ? SITE_NAME
        : systemName,
    logo: stockLogo ? SITE_LOGO : logoUrl,
  }
}

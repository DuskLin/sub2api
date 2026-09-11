import { describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    kimi: {
      startDeviceAuthorization: vi.fn(),
      pollDeviceAuthorization: vi.fn(),
      refreshKimiToken: vi.fn()
    }
  }
}))

import { useKimiOAuth } from '@/composables/useKimiOAuth'
import type { KimiTokenInfo } from '@/api/admin/kimi'

describe('useKimiOAuth.buildCredentials', () => {
  it('stores coding-plan adaptive endpoints for mainland and global regions', () => {
    const oauth = useKimiOAuth()
    const mainland = oauth.buildCredentials({
      access_token: 'at',
      refresh_token: 'rt',
      token_type: 'Bearer',
      expires_at: 1700000000,
      region: 'mainland-cn',
      email: 'user@example.com'
    } as KimiTokenInfo)
    expect(mainland.account_mode).toBe('coding')
    expect(mainland.api_protocol).toBe('adaptive')
    expect(mainland.base_url).toBe('https://api.kimi.com/coding/v1')
    expect(mainland.email).toBe('user@example.com')

    const global = oauth.buildCredentials({
      access_token: 'at',
      refresh_token: 'rt',
      region: 'global'
    } as KimiTokenInfo)
    expect(global.base_url).toBe('https://api.kimi.ai/coding/v1')
    expect(global.region).toBe('global')
  })
})

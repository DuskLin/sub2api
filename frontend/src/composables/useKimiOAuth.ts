import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { KimiOAuthRegion, KimiTokenInfo } from '@/api/admin/kimi'
import { defaultKimiOAuthAdaptiveBaseUrls } from '@/components/account/credentialsBuilder'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'

export function useKimiOAuth() {
  const appStore = useAppStore()
  const { t } = useI18n()

  const sessionId = ref('')
  const userCode = ref('')
  const verificationUri = ref('')
  const verificationUriComplete = ref('')
  const interval = ref(5)
  const expiresAt = ref(0)
  const region = ref<KimiOAuthRegion>('mainland-cn')
  const loading = ref(false)
  const polling = ref(false)
  const error = ref('')
  let pollTimer: ReturnType<typeof setTimeout> | null = null

  const resetState = () => {
    stopPolling()
    sessionId.value = ''
    userCode.value = ''
    verificationUri.value = ''
    verificationUriComplete.value = ''
    interval.value = 5
    expiresAt.value = 0
    loading.value = false
    polling.value = false
    error.value = ''
  }

  const stopPolling = () => {
    polling.value = false
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  const startDeviceAuthorization = async (
    selectedRegion: KimiOAuthRegion,
    proxyId: number | null | undefined
  ): Promise<boolean> => {
    stopPolling()
    loading.value = true
    error.value = ''
    region.value = selectedRegion
    try {
      const payload: Record<string, unknown> = { region: selectedRegion }
      if (proxyId) payload.proxy_id = proxyId
      const response = await adminAPI.kimi.startDeviceAuthorization(payload)
      sessionId.value = response.session_id
      userCode.value = response.user_code
      verificationUri.value = response.verification_uri
      verificationUriComplete.value = response.verification_uri_complete
      interval.value = Math.max(response.interval || 5, 1)
      expiresAt.value = response.expires_at
      return true
    } catch (err: unknown) {
      error.value = extractApiErrorMessage(err, t('admin.accounts.oauth.kimi.failedToStart'))
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  const pollOnce = async (): Promise<KimiTokenInfo | null> => {
    if (!sessionId.value) return null
    try {
      const result = await adminAPI.kimi.pollDeviceAuthorization(sessionId.value)
      if (result.status === 'success' && result.token_info) {
        stopPolling()
        return result.token_info
      }
      if (result.status === 'expired' || result.status === 'denied') {
        stopPolling()
        error.value =
          result.status === 'denied'
            ? t('admin.accounts.oauth.kimi.denied')
            : t('admin.accounts.oauth.kimi.expired')
        appStore.showError(error.value)
        return null
      }
      if (result.interval && result.interval > 0) {
        interval.value = result.interval
      }
      return null
    } catch (err: unknown) {
      error.value = extractI18nErrorMessage(
        err,
        t,
        'admin.accounts.oauth.kimi.errors',
        t('admin.accounts.oauth.kimi.failedToPoll')
      )
      appStore.showError(error.value)
      stopPolling()
      return null
    }
  }

  const pollUntilComplete = async (): Promise<KimiTokenInfo | null> => {
    if (!sessionId.value) return null
    polling.value = true
    error.value = ''
    while (polling.value) {
      const token = await pollOnce()
      if (token || !polling.value) return token
      await new Promise<void>((resolve) => {
        pollTimer = setTimeout(resolve, interval.value * 1000)
      })
    }
    return null
  }

  const validateRefreshToken = async (
    refreshToken: string,
    proxyId?: number | null,
    selectedRegion?: KimiOAuthRegion
  ): Promise<KimiTokenInfo | null> => {
    if (!refreshToken.trim()) {
      error.value = t('admin.accounts.oauth.kimi.pleaseEnterRefreshToken')
      return null
    }
    loading.value = true
    error.value = ''
    try {
      return await adminAPI.kimi.refreshKimiToken(refreshToken.trim(), {
        proxyId,
        region: selectedRegion || region.value
      })
    } catch (err: unknown) {
      error.value = extractI18nErrorMessage(
        err,
        t,
        'admin.accounts.oauth.kimi.errors',
        t('admin.accounts.oauth.kimi.failedToValidateRT')
      )
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const buildCredentials = (tokenInfo: KimiTokenInfo): Record<string, unknown> => {
    const selectedRegion: KimiOAuthRegion = tokenInfo.region === 'global' ? 'global' : 'mainland-cn'
    const endpoints = defaultKimiOAuthAdaptiveBaseUrls(selectedRegion)
    const credentials: Record<string, unknown> = {
      access_token: tokenInfo.access_token,
      refresh_token: tokenInfo.refresh_token,
      token_type: tokenInfo.token_type || 'Bearer',
      expires_at: tokenInfo.expires_at,
      expires_in: tokenInfo.expires_in,
      client_id: tokenInfo.client_id,
      scope: tokenInfo.scope,
      region: selectedRegion,
      oauth_host: tokenInfo.oauth_host,
      device_id: tokenInfo.device_id,
      email: tokenInfo.email,
      nickname: tokenInfo.nickname,
      user_id: tokenInfo.user_id,
      account_mode: 'coding',
      api_protocol: 'adaptive',
      base_url: endpoints.chat_completions,
      api_base_urls: endpoints
    }
    return Object.fromEntries(
      Object.entries(credentials).filter(([, value]) => value !== undefined && value !== '')
    )
  }

  const buildExtraInfo = (tokenInfo: KimiTokenInfo): Record<string, unknown> => {
    const extra: Record<string, unknown> = {}
    if (tokenInfo.email) extra.email = tokenInfo.email
    if (tokenInfo.nickname) extra.nickname = tokenInfo.nickname
    if (tokenInfo.user_id) extra.user_id = tokenInfo.user_id
    if (tokenInfo.region) extra.kimi_region = tokenInfo.region
    return extra
  }

  return {
    sessionId,
    userCode,
    verificationUri,
    verificationUriComplete,
    interval,
    expiresAt,
    region,
    loading,
    polling,
    error,
    resetState,
    stopPolling,
    startDeviceAuthorization,
    pollUntilComplete,
    validateRefreshToken,
    buildCredentials,
    buildExtraInfo
  }
}

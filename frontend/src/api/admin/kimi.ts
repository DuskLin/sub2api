import { apiClient } from '../client'

export type KimiOAuthRegion = 'mainland-cn' | 'global'

export interface KimiDeviceAuthorizationRequest {
  region?: KimiOAuthRegion
  proxy_id?: number
}

export interface KimiDeviceAuthorizationResponse {
  session_id: string
  user_code: string
  verification_uri: string
  verification_uri_complete: string
  expires_in: number
  interval: number
  expires_at: number
  region: KimiOAuthRegion
  oauth_host: string
}

export interface KimiTokenInfo {
  access_token?: string
  refresh_token?: string
  token_type?: string
  expires_at?: number | string
  expires_in?: number
  scope?: string
  client_id?: string
  region?: KimiOAuthRegion
  oauth_host?: string
  device_id?: string
  email?: string
  nickname?: string
  user_id?: string
  [key: string]: unknown
}

export interface KimiDevicePollResponse {
  status: 'pending' | 'success' | 'expired' | 'denied'
  interval?: number
  error_code?: string
  description?: string
  token_info?: KimiTokenInfo
}

export async function startDeviceAuthorization(
  payload: KimiDeviceAuthorizationRequest
): Promise<KimiDeviceAuthorizationResponse> {
  const { data } = await apiClient.post<KimiDeviceAuthorizationResponse>(
    '/admin/kimi/oauth/device-authorization',
    payload
  )
  return data
}

export async function pollDeviceAuthorization(sessionId: string): Promise<KimiDevicePollResponse> {
  const { data } = await apiClient.post<KimiDevicePollResponse>('/admin/kimi/oauth/poll', {
    session_id: sessionId
  })
  return data
}

export async function refreshKimiToken(
  refreshToken: string,
  options?: { proxyId?: number | null; region?: KimiOAuthRegion }
): Promise<KimiTokenInfo> {
  const payload: Record<string, unknown> = { refresh_token: refreshToken }
  if (options?.proxyId) payload.proxy_id = options.proxyId
  if (options?.region) payload.region = options.region
  const { data } = await apiClient.post<KimiTokenInfo>('/admin/kimi/oauth/refresh-token', payload)
  return data
}

export default {
  startDeviceAuthorization,
  pollDeviceAuthorization,
  refreshKimiToken
}

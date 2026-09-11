import { describe, expect, it } from 'vitest'

import { applyKimiOAuthRouting, cnSupportsNativeResponses, defaultCNAdaptiveBaseUrls, defaultKimiOAuthAdaptiveBaseUrls } from '../credentialsBuilder'

describe('cnSupportsNativeResponses', () => {
  it('is true for DeepSeek, Kimi, and MiniMax', () => {
    expect(cnSupportsNativeResponses('deepseek')).toBe(true)
    expect(cnSupportsNativeResponses('kimi')).toBe(true)
    expect(cnSupportsNativeResponses('minimax')).toBe(true)
    expect(cnSupportsNativeResponses('zhipu')).toBe(false)
    expect(cnSupportsNativeResponses('openai')).toBe(false)
  })
})

describe('defaultCNAdaptiveBaseUrls', () => {
  it('resolves Kimi endpoints by account mode', () => {
    expect(defaultCNAdaptiveBaseUrls('kimi', 'payg')).toEqual({
      chat_completions: 'https://api.moonshot.cn/v1',
      anthropic: 'https://api.moonshot.cn/anthropic',
      responses: 'https://api.moonshot.cn/v1'
    })
    expect(defaultCNAdaptiveBaseUrls('kimi', 'coding')).toEqual({
      chat_completions: 'https://api.kimi.com/coding/v1',
      anthropic: 'https://api.kimi.com/coding',
      responses: 'https://api.kimi.com/coding/v1'
    })
    expect(defaultKimiOAuthAdaptiveBaseUrls('global')).toEqual({
      chat_completions: 'https://api.kimi.ai/coding/v1',
      anthropic: 'https://api.kimi.ai/coding',
      responses: 'https://api.kimi.ai/coding/v1'
    })
  })

  it('resolves GLM endpoints by account mode', () => {
    expect(defaultCNAdaptiveBaseUrls('zhipu', 'payg')).toEqual({
      chat_completions: 'https://open.bigmodel.cn/api/paas/v4',
      anthropic: 'https://open.bigmodel.cn/api/anthropic',
      responses: ''
    })
    expect(defaultCNAdaptiveBaseUrls('zhipu', 'coding')).toEqual({
      chat_completions: 'https://open.bigmodel.cn/api/coding/paas/v4',
      anthropic: 'https://open.bigmodel.cn/api/anthropic',
      responses: ''
    })
  })

  it('includes all three native DeepSeek endpoints', () => {
    expect(defaultCNAdaptiveBaseUrls('deepseek', 'payg')).toEqual({
      chat_completions: 'https://api.deepseek.com',
      anthropic: 'https://api.deepseek.com/anthropic',
      responses: 'https://api.deepseek.com'
    })
  })

  it('uses the same MiniMax CN endpoints for payg and coding', () => {
    const expected = {
      chat_completions: 'https://api.minimaxi.com/v1',
      anthropic: 'https://api.minimaxi.com/anthropic',
      responses: 'https://api.minimaxi.com/v1'
    }
    expect(defaultCNAdaptiveBaseUrls('minimax', 'payg')).toEqual(expected)
    expect(defaultCNAdaptiveBaseUrls('minimax', 'coding')).toEqual(expected)
  })
})

describe('applyKimiOAuthRouting', () => {
  it('writes adaptive endpoints and keeps coding mode', () => {
    const credentials: Record<string, unknown> = {}
    applyKimiOAuthRouting(
      credentials,
      'adaptive',
      'global',
      {
        chat_completions: 'https://relay.example.com/v1',
        anthropic: '',
        responses: 'https://relay.example.com/responses'
      },
      ''
    )
    expect(credentials).toMatchObject({
      account_mode: 'coding',
      api_protocol: 'adaptive',
      base_url: 'https://relay.example.com/v1',
      api_base_urls: {
        chat_completions: 'https://relay.example.com/v1',
        anthropic: 'https://api.kimi.ai/coding',
        responses: 'https://relay.example.com/responses'
      }
    })
  })

  it('writes a single legacy endpoint and drops api_base_urls', () => {
    const credentials: Record<string, unknown> = {
      api_base_urls: { chat_completions: 'https://api.kimi.com/coding/v1' }
    }
    applyKimiOAuthRouting(
      credentials,
      'anthropic',
      'mainland-cn',
      {
        chat_completions: '',
        anthropic: '',
        responses: ''
      },
      'https://relay.example.com/anthropic'
    )
    expect(credentials.account_mode).toBe('coding')
    expect(credentials.api_protocol).toBe('anthropic')
    expect(credentials.base_url).toBe('https://relay.example.com/anthropic')
    expect(credentials).not.toHaveProperty('api_base_urls')
  })
})

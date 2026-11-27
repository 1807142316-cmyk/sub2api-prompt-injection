import { describe, expect, it } from 'vitest'
import {
  applyCodexRequestCompat,
  DEFAULT_CODEX_REQUEST_USER_AGENT,
  isCodexRequestCompatCapable,
  readCodexRequestCompat,
  resolveCodexRequestUserAgent,
  validateCodexRequestCompat
} from '../credentialsBuilder'

describe('Codex request compatibility credentials', () => {
  it('keeps untouched legacy credentials unchanged', () => {
    const credentials = {
      user_agent: 'ExistingAgent/1',
      header_override_enabled: true,
      header_overrides: { 'User-Agent': 'ProviderAgent/2', 'x-codex-installation-id': 'existing-id' }
    }
    const before = structuredClone(credentials)
    const draft = readCodexRequestCompat(credentials)
    expect(draft).toEqual({ enabled: false, userAgent: 'ProviderAgent/2', configured: false })
    applyCodexRequestCompat(credentials, draft)
    expect(credentials).toEqual(before)
  })

  it('prefers a saved compatibility value and preserves it while disabled', () => {
    const credentials: Record<string, unknown> = {
      codex_request_compat_enabled: true,
      codex_request_compat_user_agent: 'SavedAgent/3',
      header_override_enabled: true,
      header_overrides: { 'User-Agent': 'ProviderAgent/2', 'x-codex-installation-id': 'existing-id' }
    }
    const draft = readCodexRequestCompat(credentials)
    expect(draft.userAgent).toBe('SavedAgent/3')
    applyCodexRequestCompat(credentials, { ...draft, enabled: false })
    expect(credentials).toMatchObject({
      codex_request_compat_enabled: false,
      codex_request_compat_user_agent: 'SavedAgent/3',
      header_override_enabled: true,
      header_overrides: { 'User-Agent': 'ProviderAgent/2', 'x-codex-installation-id': 'existing-id' }
    })
    expect(Object.keys(credentials)).toHaveLength(4)
    expect(readCodexRequestCompat(credentials)).toEqual({ ...draft, enabled: false })
  })

  it.each(['', 'Bad\nAgent', '{{request_id}}'])('ignores unusable custom UA %j when prefilling', (value) => {
    expect(readCodexRequestCompat({
      header_override_enabled: true,
      header_overrides: { 'USER-AGENT': value },
      user_agent: 'LegacyAgent/4'
    }).userAgent).toBe('LegacyAgent/4')
  })

  it('uses strict booleans and a default only when no usable UA exists', () => {
    expect(readCodexRequestCompat({
      codex_request_compat_enabled: 'true',
      header_override_enabled: 'true',
      header_overrides: { 'User-Agent': 'InactiveAgent/1' }
    })).toEqual({ enabled: false, userAgent: DEFAULT_CODEX_REQUEST_USER_AGENT, configured: true })
  })

  it('does not replace a saved empty UA with a suggested default', () => {
    const draft = readCodexRequestCompat({ codex_request_compat_user_agent: '' })
    expect(draft.userAgent).toBe('')
    expect(validateCodexRequestCompat(draft)).toBeNull()
    expect(validateCodexRequestCompat({ ...draft, enabled: true })).toBe('userAgentRequired')
  })

  it.each([
    [' \t ', 'userAgentRequired'],
    ['Agent\nInjected', 'userAgentInvalid'],
    ['Agent\rInjected', 'userAgentInvalid'],
    ['Agent\u0000', 'userAgentInvalid'],
    ['a'.repeat(8193), 'userAgentInvalid'],
    ['\u4e2d'.repeat(2731), 'userAgentInvalid'],
    ['Agent/{{request_id}}', 'userAgentLiteralRequired'],
    ['a'.repeat(8192), null],
    ['Agent\tVersion', null]
  ])('validates the saved header value', (userAgent, error) => {
    expect(validateCodexRequestCompat({
      configured: true, enabled: true, userAgent: userAgent as string
    })).toBe(error)
  })

  it('validates an invalid saved UA even when compatibility is off', () => {
    expect(validateCodexRequestCompat({
      configured: true, enabled: false, userAgent: 'bad\nvalue'
    })).toBe('userAgentInvalid')
  })

  it('previews case-insensitive overrides and skips empty or disabled overrides', () => {
    const draft = { configured: true, enabled: true, userAgent: 'CompatAgent/1' }
    expect(resolveCodexRequestUserAgent(draft, true, [{ name: 'USER-Agent', value: 'Custom/2' }]))
      .toEqual({ source: 'override', value: 'Custom/2' })
    expect(resolveCodexRequestUserAgent(draft, true, [{ name: 'user-agent', value: '  ' }]))
      .toEqual({ source: 'compat', value: 'CompatAgent/1' })
    expect(resolveCodexRequestUserAgent(draft, false, [{ name: 'user-agent', value: 'Custom/2' }]))
      .toEqual({ source: 'compat', value: 'CompatAgent/1' })
    expect(resolveCodexRequestUserAgent({ ...draft, enabled: false }, true, [{ name: 'user-agent', value: '' }]))
      .toEqual({ source: 'original', value: '' })
  })

  it.each([
    ['openai', 'apikey', true],
    ['openai', 'oauth', false],
    ['anthropic', 'apikey', false],
    ['grok', 'apikey', false]
  ])('is restricted to OpenAI API key accounts', (platform, type, expected) => {
    expect(isCodexRequestCompatCapable(platform as string, type as string)).toBe(expected)
  })
})

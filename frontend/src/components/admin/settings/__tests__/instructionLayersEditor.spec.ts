import { describe, expect, it } from 'vitest'
import type { InstructionLayerProfile, InstructionLayersConfig } from '@/api/admin/instruction-layers'
import { canonicalScopeId, MAX_TEXT_BYTES, profileFromConfig, resolveLayers, validateDrafts } from '../instructionLayersEditor'

const profile = (changes: Partial<InstructionLayerProfile> = {}): InstructionLayerProfile => ({
  enabled: true, prefix: 'default before', suffix: 'default after', model_suffixes: {}, ...changes,
})
const config = (): InstructionLayersConfig => ({
  enabled: true,
  default: profile({ prefix: '  默认\n前置 \n', suffix: '\t默认后置\n', model_suffixes: { '*': 'other', exact: '' } }),
  users: { '7': profile({ prefix: '', suffix: 'user after' }) },
  api_keys: { '9': profile({ enabled: false, prefix: 'key before' }) },
  groups: {},
})
function drafts(value: InstructionLayersConfig) {
  let sequence = 0
  const key = (prefix: string) => `${prefix}-${++sequence}`
  return [
    profileFromConfig('default', '', value.default, key),
    ...Object.entries(value.users).map(([id, item]) => profileFromConfig('user', id, item, key)),
    ...Object.entries(value.api_keys).map(([id, item]) => profileFromConfig('api_key', id, item, key)),
    ...Object.entries(value.groups || {}).map(([id, item]) => profileFromConfig('group', id, item, key)),
  ]
}

describe('instruction editor configuration compatibility', () => {
  it('round-trips all text, model blank overrides and scope bindings without normalization', () => {
    const source = config()
    source.default.model_suffixes[' legacy-model '] = '\n 模型内容 \t'
    const result = validateDrafts(source.enabled, drafts(source))
    expect(result.errors).toEqual([])
    expect(result.config).toEqual(source)
    expect(drafts(source)[0].modelSuffixes.find((rule) => rule.model === 'exact')?.mode).toBe('none')
  })

  it('applies complete key/user/default precedence without merging blank fields', () => {
    const source = config()
    expect(resolveLayers(source, 'model', '7').prefix).toBe('')
    expect(resolveLayers(source, 'model', '7').suffix).toBe('user after')
    expect(resolveLayers(source, 'model', '7', '9')).toMatchObject({
      scope: 'api_key', reason: 'profile_disabled', prefix: '', suffix: '',
    })
    expect(resolveLayers(source, 'model', '7', 'missing')).toMatchObject({ scope: 'user', prefix: '' })
    expect(resolveLayers(source, 'model', 'missing', 'missing')).toMatchObject({ scope: 'default' })
  })

  it('keeps an exact blank suffix ahead of wildcard and common text', () => {
    const source = config()
    expect(resolveLayers(source, 'exact')).toMatchObject({ suffix: '', suffixSource: 'exact', prefix: source.default.prefix })
    expect(resolveLayers(source, 'other')).toMatchObject({ suffix: 'other', suffixSource: 'fallback' })
    source.default.model_suffixes = {}
    expect(resolveLayers(source, 'other')).toMatchObject({ suffix: '\t默认后置\n', suffixSource: 'common' })
  })

  it('disables all page layers at the master switch without changing saved profiles', () => {
    const source = config()
    source.enabled = false
    expect(resolveLayers(source, 'exact', '7')).toMatchObject({ scope: 'user', enabled: false, reason: 'master_disabled', prefix: '', suffix: '' })
    expect(source.users['7'].suffix).toBe('user after')
  })

  it('serializes explicit no-append as empty but retains unsaved custom text for toggling back', () => {
    const source = config()
    const items = drafts(source)
    items[0].modelSuffixes[0].mode = 'none'
    expect(validateDrafts(true, items).config?.default.model_suffixes['*']).toBe('')
    expect(items[0].modelSuffixes[0].text).toBe('other')
    items[0].modelSuffixes[0].mode = 'custom'
    expect(validateDrafts(true, items).config?.default.model_suffixes['*']).toBe('other')
  })

  it('rejects duplicate models, duplicate objects and missing custom content', () => {
    const items = drafts(config())
    items[0].modelSuffixes.push({ key: 'duplicate', model: '*', mode: 'custom', text: '' })
    items.push({ ...items[1], key: 'duplicate-user' })
    const result = validateDrafts(true, items)
    expect(result.config).toBeNull()
    expect(result.errors.join('\n')).toContain('重复规则')
    expect(result.errors.join('\n')).toContain('同一使用对象')
    expect(result.errors.join('\n')).toContain('专用内容')
  })

  it('validates canonical int64 IDs without JavaScript numeric rounding', () => {
    for (const value of ['', '0', '01', '+1', '-1', '1.0', '9223372036854775808']) expect(canonicalScopeId(value)).toBeNull()
    expect(canonicalScopeId('9223372036854775807')).toBe('9223372036854775807')
    expect(canonicalScopeId(' 12 ')).toBe('12')
  })

  it('checks UTF-8 byte limits and the total configuration limit', () => {
    const items = drafts(config())
    items[0].prefix = '中'.repeat(Math.floor(MAX_TEXT_BYTES / 3) + 1)
    expect(validateDrafts(true, items).errors.join('\n')).toContain('前置提示词超过')
    const large = profile({ prefix: 'a'.repeat(MAX_TEXT_BYTES), suffix: 'b'.repeat(MAX_TEXT_BYTES) })
    const source: InstructionLayersConfig = { enabled: true, default: large, users: {}, api_keys: {} }
    for (let id = 1; id < 9; id++) source.users[String(id)] = large
    expect(validateDrafts(true, drafts(source)).errors.join('\n')).toContain('合计超过')
  })

  it('handles model names that overlap object prototype properties as exact keys only', () => {
    const source = config()
    source.default.model_suffixes = JSON.parse('{"__proto__":"safe literal","constructor":""}')
    const roundTrip = validateDrafts(true, drafts(source)).config!
    expect(resolveLayers(roundTrip, '__proto__').suffix).toBe('safe literal')
    expect(resolveLayers(roundTrip, 'constructor').suffix).toBe('')
    expect(resolveLayers(roundTrip, 'toString').suffix).toBe(source.default.suffix)
  })

  it('round-trips group content and falls back through key, user, group, then default', () => {
    const source = config()
    source.groups = { '3': profile({ prefix: ' \n分组前置\t', suffix: '', model_suffixes: { exact: '' } }) }
    expect(validateDrafts(true, drafts(source)).config).toEqual(source)
    expect(resolveLayers(source, 'exact', '', '', '3')).toMatchObject({ scope: 'group', prefix: ' \n分组前置\t', suffix: '', suffixSource: 'exact' })
    expect(resolveLayers(source, 'model', '7', '', '3')).toMatchObject({ scope: 'user', suffix: 'user after' })
    expect(resolveLayers(source, 'model', '7', '9', '3')).toMatchObject({ scope: 'api_key', reason: 'profile_disabled' })
    delete source.groups['3']
    expect(resolveLayers(source, 'model', '', '', '3').scope).toBe('default')
  })

  it('gives group opt-out priority over user and key profiles even with the master off', () => {
    const source = config()
    source.groups = { '3': profile({ enabled: false, prefix: 'retained group draft' }) }
    source.api_keys['9'].enabled = true
    for (const enabled of [true, false]) {
      source.enabled = enabled
      expect(resolveLayers(source, 'exact', '7', '9', '3')).toMatchObject({
        scope: 'group', scopeId: '3', enabled: false, reason: 'group_opted_out', prefix: '', suffix: '',
      })
    }
    expect(source.groups['3'].prefix).toBe('retained group draft')
  })

  it('accepts legacy responses without groups without changing other scope behavior', () => {
    const source = config()
    delete source.groups
    expect(resolveLayers(source, 'model', '', '', '3').scope).toBe('default')
    expect(validateDrafts(true, drafts(source)).config).toEqual({ ...source, groups: {} })
  })

  it('counts Unicode model-name characters and all scopes against backend limits', () => {
    const source = config()
    source.default.model_suffixes = { ['𠮷'.repeat(200)]: 'allowed' }
    expect(validateDrafts(true, drafts(source)).errors).toEqual([])
    source.default.model_suffixes = { ['𠮷'.repeat(201)]: 'too long' }
    expect(validateDrafts(true, drafts(source)).errors.join('\n')).toContain('模型名称超过 200 个字符')
    source.default.model_suffixes = {}
    for (let id = 1; id <= 126; id++) source.groups![String(id)] = profile()
    expect(validateDrafts(true, drafts(source)).errors.join('\n')).toContain('最多可保存 128 份配置')
  })
})

import type { InstructionLayerProfile, InstructionLayersConfig } from '@/api/admin/instruction-layers'

export type ScopeKind = 'default' | 'group' | 'user' | 'api_key'
export interface ModelRuleDraft {
  key: string
  model: string
  mode: 'custom' | 'none'
  text: string
}
export interface ProfileDraft {
  key: string
  scope: ScopeKind
  scopeId: string
  enabled: boolean
  prefix: string
  suffix: string
  modelSuffixes: ModelRuleDraft[]
}

export const MAX_TEXT_BYTES = 128 * 1024
export const MAX_CONFIG_BYTES = 2 * 1024 * 1024
const own = (value: object, key: string): boolean => Object.prototype.hasOwnProperty.call(value, key)
const utf8Bytes = (value: string): number => new TextEncoder().encode(value).length

export function canonicalScopeId(value: string): string | null {
  const id = value.trim()
  if (!/^[1-9]\d*$/.test(id) || id.length > 19 || (id.length === 19 && id > '9223372036854775807')) return null
  return id
}

export function emptyProfile(scope: ScopeKind, scopeId: string, key: string): ProfileDraft {
  return { key, scope, scopeId, enabled: true, prefix: '', suffix: '', modelSuffixes: [] }
}

export function profileFromConfig(
  scope: ScopeKind,
  scopeId: string,
  value: InstructionLayerProfile,
  nextKey: (prefix: string) => string,
): ProfileDraft {
  return {
    ...emptyProfile(scope, scopeId, scope === 'default' ? 'default' : nextKey(scope)),
    enabled: value.enabled === true,
    prefix: value.prefix,
    suffix: value.suffix,
    modelSuffixes: Object.entries(value.model_suffixes || {}).map(([model, text]) => ({
      key: nextKey('model'), model, text, mode: text === '' ? 'none' : 'custom',
    })),
  }
}

export function basicProfileLabel(profile: ProfileDraft): string {
  if (profile.scope === 'default') return '全局默认'
  const label = profile.scope === 'group' ? '指定分组' : profile.scope === 'user' ? '指定用户' : '指定客户端'
  return `${label} · 编号 ${profile.scopeId || '未填写'}`
}

export function validateDrafts(
  enabled: boolean,
  profiles: ProfileDraft[],
): { config: InstructionLayersConfig | null; errors: string[] } {
  const errors: string[] = []
  const config: InstructionLayersConfig = {
    enabled, default: { enabled: false, prefix: '', suffix: '', model_suffixes: {} }, users: {}, api_keys: {}, groups: {},
  }
  const seenScopes = new Set<string>()
  if (profiles.length > 128) errors.push('最多可保存 128 份配置，请合并不再需要的专属配置。')
  for (const draft of profiles) {
    const label = basicProfileLabel(draft)
    const id = draft.scope === 'default' ? '' : canonicalScopeId(draft.scopeId)
    if (id === null) errors.push(`${label}：编号应为不含前导零的正整数。`)
    const scopeKey = `${draft.scope}:${id ?? draft.scopeId}`
    if (seenScopes.has(scopeKey)) errors.push(`${label}：已经存在同一使用对象的配置。`)
    seenScopes.add(scopeKey)
    if (utf8Bytes(draft.prefix) > MAX_TEXT_BYTES) errors.push(`${label}：前置提示词超过 128 KiB，请缩短正文。`)
    if (utf8Bytes(draft.suffix) > MAX_TEXT_BYTES) errors.push(`${label}：后置提示词超过 128 KiB，请缩短正文。`)
    const seenModels = new Set<string>()
    const modelEntries: [string, string][] = []
    for (const [index, rule] of draft.modelSuffixes.entries()) {
      if (!rule.model.trim()) errors.push(`${label}：第 ${index + 1} 条模型规则尚未选择模型。`)
      else if (seenModels.has(rule.model)) errors.push(`${label}：模型“${rule.model}”存在重复规则。`)
      if ([...rule.model].length > 200) errors.push(`${label}：第 ${index + 1} 条规则的模型名称超过 200 个字符。`)
      seenModels.add(rule.model)
      if (rule.mode === 'custom' && rule.text === '') {
        errors.push(`${label}：请填写“${rule.model || `第 ${index + 1} 条规则`}”的专用内容，或选择“不追加后置提示词”。`)
      }
      const text = rule.mode === 'none' ? '' : rule.text
      if (utf8Bytes(text) > MAX_TEXT_BYTES) errors.push(`${label}：模型“${rule.model}”的提示词超过 128 KiB。`)
      if (rule.model.trim()) modelEntries.push([rule.model, text])
    }
    const profile: InstructionLayerProfile = {
      enabled: draft.enabled, prefix: draft.prefix, suffix: draft.suffix, model_suffixes: Object.fromEntries(modelEntries),
    }
    if (draft.scope === 'default') config.default = profile
    else if (id) {
      if (draft.scope === 'group') config.groups![id] = profile
      else if (draft.scope === 'user') config.users[id] = profile
      else config.api_keys[id] = profile
    }
  }
  if (!seenScopes.has('default:')) errors.push('缺少全局默认配置，请重新读取后重试。')
  const encoded = JSON.stringify(config).replace(/[<>&\u2028\u2029]/g, (character) =>
    `\\u${character.charCodeAt(0).toString(16).padStart(4, '0')}`)
  if (utf8Bytes(encoded) > MAX_CONFIG_BYTES) errors.push('全部提示词合计超过 2 MiB，请缩短正文或减少专属配置。')
  return { config: errors.length ? null : config, errors }
}

export interface ResolvedLayers {
  scope: ScopeKind
  scopeId: string
  enabled: boolean
  reason: 'enabled' | 'master_disabled' | 'profile_disabled' | 'group_opted_out'
  prefix: string
  suffix: string
  suffixSource: 'exact' | 'fallback' | 'common'
}

export function resolveLayers(
  config: InstructionLayersConfig,
  model: string,
  userId = '',
  apiKeyId = '',
  groupId = '',
): ResolvedLayers {
  const group = groupId && own(config.groups || {}, groupId) ? config.groups![groupId] : undefined
  if (group && !group.enabled) {
    return {
      scope: 'group', scopeId: groupId, enabled: false, reason: 'group_opted_out',
      prefix: '', suffix: '', suffixSource: 'common',
    }
  }
  let scope: ScopeKind = 'default'
  let scopeId = ''
  let profile = config.default
  if (apiKeyId && own(config.api_keys, apiKeyId)) {
    scope = 'api_key'
    scopeId = apiKeyId
    profile = config.api_keys[apiKeyId]
  } else if (userId && own(config.users, userId)) {
    scope = 'user'
    scopeId = userId
    profile = config.users[userId]
  } else if (group) {
    scope = 'group'
    scopeId = groupId
    profile = group
  }
  let suffixSource: ResolvedLayers['suffixSource'] = 'common'
  let suffix = profile.suffix
  if (own(profile.model_suffixes, model)) {
    suffixSource = 'exact'
    suffix = profile.model_suffixes[model]
  } else if (own(profile.model_suffixes, '*')) {
    suffixSource = 'fallback'
    suffix = profile.model_suffixes['*']
  }
  const enabled = config.enabled && profile.enabled
  return {
    scope, scopeId, enabled,
    reason: !config.enabled ? 'master_disabled' : !profile.enabled ? 'profile_disabled' : 'enabled',
    prefix: enabled ? profile.prefix : '', suffix: enabled ? suffix : '', suffixSource,
  }
}

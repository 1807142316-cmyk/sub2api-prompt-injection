<template>
  <section class="card" aria-labelledby="instruction-layers-title" data-testid="instruction-layers-panel">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 id="instruction-layers-title" class="text-lg font-semibold text-gray-900 dark:text-white">提示词注入管理面板（分层指令）</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            正文作为网关工作规范，保留客户端原指令。这里只保存正文，不会读取或自动创建文本提到的外部 profile 文件。
          </p>
        </div>
        <span v-if="dirty" class="rounded-full bg-amber-50 px-3 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/20 dark:text-amber-300" role="status">有未保存更改</span>
      </div>
    </div>

    <div class="space-y-5 p-6">
      <div v-if="loading" class="flex min-h-24 items-center justify-center gap-2 text-sm text-gray-500" role="status">
        <span class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></span>
        正在读取分层指令配置…
      </div>
      <div v-else-if="loadFailed" class="rounded-lg border border-red-200 bg-red-50 p-4 dark:border-red-900 dark:bg-red-950/20" role="alert">
        <p class="text-sm text-red-700 dark:text-red-300">配置读取失败，尚未覆盖任何设置。请检查连接后重试。</p>
        <button type="button" class="btn btn-secondary btn-sm mt-3" @click="loadConfig">重新读取</button>
      </div>
      <template v-else>
        <fieldset :disabled="saving" class="min-w-0 space-y-5 disabled:opacity-70">
          <div class="flex items-center justify-between gap-4">
            <div>
              <label for="instruction-layers-master" class="font-medium text-gray-900 dark:text-white">启用分层指令</label>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">关闭后跳过全部前置和追加层，原有任务 Skills 仍正常生效。</p>
            </div>
            <Toggle id="instruction-layers-master" v-model="enabled" aria-label="启用分层指令" />
          </div>

          <div class="rounded-xl border border-primary-100 bg-primary-50/50 p-4 dark:border-primary-900/50 dark:bg-primary-950/20">
            <div class="grid gap-2 text-center text-sm sm:grid-cols-[1fr_auto_1fr_auto_1fr]" aria-label="指令拼接顺序">
              <div class="rounded-lg border border-primary-100 bg-white p-3 dark:border-primary-900/50 dark:bg-dark-800"><strong>① 前置正文</strong><p class="mt-1 text-xs text-gray-500">选中 profile 的 prefix</p></div>
              <span class="self-center text-primary-500" aria-hidden="true">→</span>
              <div class="rounded-lg border border-primary-100 bg-white p-3 dark:border-primary-900/50 dark:bg-dark-800"><strong>② 客户端原指令</strong><p class="mt-1 text-xs text-gray-500">保留客户端提供的内容</p></div>
              <span class="self-center text-primary-500" aria-hidden="true">→</span>
              <div class="rounded-lg border border-primary-100 bg-white p-3 dark:border-primary-900/50 dark:bg-dark-800"><strong>③ 追加正文</strong><p class="mt-1 text-xs text-gray-500">模型匹配的 suffix</p></div>
            </div>
            <p class="mt-3 text-center text-xs text-gray-600 dark:text-gray-300">随后追加按需求选择的任务 Skills；不同协议仍使用各自原有的指令字段。</p>
          </div>

          <div class="space-y-1 rounded-lg bg-gray-50 p-4 text-sm text-gray-600 dark:bg-dark-800 dark:text-gray-300">
            <p><strong>作用域：</strong>客户端 Key ID ＞ 用户 ID ＞ 默认。匹配较高作用域后使用整份 profile，不逐字段继承。</p>
            <p><strong>profile 关闭：</strong>该作用域阻断继承，不回落到用户或默认 profile。</p>
            <p><strong>模型追加：</strong>精确模型名 ＞ <code>*</code> ＞ 通用追加正文；前置正文不随模型切换。</p>
          </div>

          <div class="grid gap-5 lg:grid-cols-[250px_minmax(0,1fr)]">
            <aside class="min-w-0 space-y-3" aria-label="配置作用域列表">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">选择 profile</h3>
              <div class="space-y-2">
                <button
                  v-for="profile in profiles"
                  :key="profile.key"
                  type="button"
                  :aria-pressed="selectedKey === profile.key"
                  :class="[
                    'flex w-full items-center justify-between gap-2 rounded-lg border px-3 py-3 text-left text-sm transition-colors',
                    selectedKey === profile.key
                      ? 'border-primary-500 bg-primary-50 text-primary-800 dark:bg-primary-950/30 dark:text-primary-200'
                      : 'border-gray-200 text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-800',
                  ]"
                  @click="selectedKey = profile.key"
                >
                  <span class="min-w-0 break-all">{{ profileLabel(profile) }}</span>
                  <span class="shrink-0 text-xs" :class="profile.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500'">{{ profile.enabled ? '启用' : '关闭' }}</span>
                </button>
              </div>
              <div class="space-y-3 rounded-lg border border-dashed border-gray-300 p-3 dark:border-dark-600">
                <h4 class="text-sm font-medium text-gray-800 dark:text-gray-200">添加作用域</h4>
                <label for="instruction-layer-scope-kind" class="sr-only">新作用域类型</label>
                <select id="instruction-layer-scope-kind" v-model="newScope" class="input w-full">
                  <option value="user">用户 ID</option>
                  <option value="api_key">客户端 Key ID</option>
                </select>
                <Input
                  id="instruction-layer-new-id"
                  v-model="newScopeId"
                  label="数据库数值 ID"
                  placeholder="例如 12"
                  :error="addError"
                  autocomplete="off"
                  @enter="addProfile"
                />
                <button type="button" class="btn btn-secondary btn-sm w-full" @click="addProfile">添加 profile</button>
                <p class="text-xs text-gray-500 dark:text-gray-400">Key ID 是客户端 Key 的数据库数值 ID，不是密钥正文；此处无需填写密码或 secret Key。</p>
              </div>
            </aside>

            <div v-if="selectedProfile" class="min-w-0 space-y-4 rounded-xl border border-gray-200 p-4 dark:border-dark-600 sm:p-5">
              <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 pb-4 dark:border-dark-700">
                <div>
                  <h3 class="font-semibold text-gray-900 dark:text-white">{{ profileLabel(selectedProfile) }}</h3>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">独立整份 profile；留空的正文不会继承较低作用域。</p>
                </div>
                <button v-if="selectedProfile.scope !== 'default'" type="button" class="btn btn-secondary btn-sm text-red-600 dark:text-red-400" @click="removeSelectedProfile">删除此作用域</button>
              </div>

              <Input
                v-if="selectedProfile.scope !== 'default'"
                :id="`instruction-layer-scope-${selectedProfile.key}`"
                v-model="selectedProfile.scopeId"
                :label="selectedProfile.scope === 'user' ? '用户数据库 ID' : '客户端 Key 数据库 ID'"
                placeholder="正整数 ID"
                autocomplete="off"
                hint="更改数值 ID 会将整份 profile 保存到新的作用域。"
              />

              <div class="flex items-center justify-between gap-4">
                <div>
                  <label :for="`instruction-layer-enabled-${selectedProfile.key}`" class="text-sm font-medium text-gray-900 dark:text-white">启用此 profile</label>
                  <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">关闭时该作用域不注入前置或追加正文，并阻断向下继承。</p>
                </div>
                <Toggle :id="`instruction-layer-enabled-${selectedProfile.key}`" v-model="selectedProfile.enabled" :aria-label="`启用${profileLabel(selectedProfile)}`" />
              </div>
              <p v-if="!selectedProfile.enabled" class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">此 profile 当前关闭；下方正文可以预先编辑并保存，启用后再生效。</p>
              <div>
                <label :for="`instruction-layer-prefix-${selectedProfile.key}`" class="input-label">前置正文（prefix）</label>
                <textarea :id="`instruction-layer-prefix-${selectedProfile.key}`" v-model="selectedProfile.prefix" rows="7" class="input min-h-40 w-full resize-y font-mono text-sm" placeholder="放在客户端原指令前的网关工作规范"></textarea>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ selectedProfile.prefix.length }} 字符 · 所有模型共用这份前置正文。</p>
              </div>
              <div>
                <label :for="`instruction-layer-suffix-${selectedProfile.key}`" class="input-label">通用追加正文（suffix）</label>
                <textarea :id="`instruction-layer-suffix-${selectedProfile.key}`" v-model="selectedProfile.suffix" rows="7" class="input min-h-40 w-full resize-y font-mono text-sm" placeholder="未匹配模型专用项或 * 时，追加到客户端原指令后的正文"></textarea>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ selectedProfile.suffix.length }} 字符 · 空字符串表示该项不追加正文。</p>
              </div>

              <div class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h4 class="text-sm font-semibold text-gray-900 dark:text-white">模型专用追加正文</h4>
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">精确模型名优先；<code>*</code> 匹配其余模型。命中后整段替换通用追加，空正文表示不追加。</p>
                  </div>
                  <button type="button" class="btn btn-secondary btn-sm" @click="addModelSuffix">添加模型项</button>
                </div>
                <p v-if="selectedProfile.modelSuffixes.length === 0" class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-center text-sm text-gray-500 dark:border-dark-600">尚无模型项，所有模型使用通用追加正文。</p>
                <div v-for="(entry, index) in selectedProfile.modelSuffixes" :key="entry.key" class="space-y-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
                  <div class="flex items-end gap-3">
                    <Input :id="`instruction-layer-model-${entry.key}`" v-model="entry.model" :label="`模型项 ${index + 1}`" placeholder="精确模型名或 *" autocomplete="off" />
                    <button type="button" class="btn btn-secondary btn-sm mb-0.5 shrink-0 text-red-600 dark:text-red-400" :aria-label="`删除模型项 ${index + 1}`" @click="removeModelSuffix(entry.key)">删除</button>
                  </div>
                  <label :for="`instruction-layer-model-body-${entry.key}`" class="sr-only">{{ entry.model || `模型项 ${index + 1}` }} 追加正文</label>
                  <textarea :id="`instruction-layer-model-body-${entry.key}`" v-model="entry.text" rows="5" class="input min-h-28 w-full resize-y font-mono text-sm" placeholder="该模型的完整追加正文"></textarea>
                </div>
              </div>
            </div>
          </div>

          <div v-if="validation.errors.length > 0" class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950/20 dark:text-amber-200" role="alert">
            <p class="font-medium">请先修正以下项目：</p>
            <ul class="mt-2 list-inside list-disc space-y-1"><li v-for="error in validation.errors" :key="error">{{ error }}</li></ul>
          </div>
          <details class="rounded-lg border border-gray-200 dark:border-dark-600">
            <summary class="cursor-pointer px-4 py-3 text-sm font-medium text-gray-700 dark:text-gray-300">可选：查看将提交的 JSON（只读）</summary>
            <pre v-if="validation.config" class="max-h-80 overflow-auto border-t border-gray-200 bg-gray-50 p-4 text-xs text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300">{{ jsonPreview }}</pre>
            <p v-else class="px-4 pb-4 text-sm text-gray-500">修正上面的校验项目后显示完整预览。</p>
          </details>
        </fieldset>

        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-gray-400">此区独立保存；页面底部的其他设置保存按钮不会提交这里的草稿。切换设置页签会保留未保存编辑。</p>
          <div class="flex shrink-0 gap-3">
            <button type="button" class="btn btn-secondary" :disabled="saving" @click="reloadConfig">重新读取</button>
            <button type="button" class="btn btn-primary" :disabled="saving || !dirty || validation.errors.length > 0" data-testid="instruction-layers-save" @click="saveConfig">{{ saving ? '正在保存…' : '保存分层指令' }}</button>
          </div>
        </div>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import Input from '@/components/common/Input.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores'
import {
  getInstructionLayers,
  updateInstructionLayers,
  type InstructionLayerProfile,
  type InstructionLayersConfig,
} from '@/api/admin/instruction-layers'

type ScopeKind = 'default' | 'user' | 'api_key'
interface ModelSuffixDraft { key: string; model: string; text: string }
interface ProfileDraft {
  key: string
  scope: ScopeKind
  scopeId: string
  enabled: boolean
  prefix: string
  suffix: string
  modelSuffixes: ModelSuffixDraft[]
}

const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const loadFailed = ref(false)
const enabled = ref(false)
const profiles = ref<ProfileDraft[]>([])
const selectedKey = ref('default')
const newScope = ref<'user' | 'api_key'>('user')
const newScopeId = ref('')
const addError = ref('')
const savedState = ref('')
let keySequence = 0
let alive = true
const nextKey = (prefix: string): string => `${prefix}-${++keySequence}`
const selectedProfile = computed(() => profiles.value.find((profile) => profile.key === selectedKey.value))
const stateString = computed(() => JSON.stringify({ enabled: enabled.value, profiles: profiles.value }))
const dirty = computed(() => !loading.value && !loadFailed.value && stateString.value !== savedState.value)

function emptyProfile(scope: ScopeKind, scopeId = ''): ProfileDraft {
  return { key: scope === 'default' ? 'default' : nextKey(scope), scope, scopeId, enabled: true, prefix: '', suffix: '', modelSuffixes: [] }
}
function profileLabel(profile: ProfileDraft): string {
  if (profile.scope === 'default') return '默认 profile'
  return `${profile.scope === 'user' ? '用户 ID' : '客户端 Key ID'} ${profile.scopeId || '（未填写）'}`
}
function normalizedId(value: string): string | null {
  const input = value.trim()
  if (!/^\d+$/.test(input)) return null
  const id = input.replace(/^0+/, '')
  if (!id || id.length > 19 || (id.length === 19 && id > '9223372036854775807')) return null
  return id
}
function toDraft(scope: ScopeKind, scopeId: string, profile?: InstructionLayerProfile): ProfileDraft {
  return {
    ...emptyProfile(scope, scopeId),
    enabled: profile?.enabled === true,
    prefix: typeof profile?.prefix === 'string' ? profile.prefix : '',
    suffix: typeof profile?.suffix === 'string' ? profile.suffix : '',
    modelSuffixes: Object.entries(profile?.model_suffixes || {}).map(([model, text]) => ({ key: nextKey('model'), model, text: typeof text === 'string' ? text : '' })),
  }
}
function applyLoaded(config: InstructionLayersConfig): void {
  enabled.value = config.enabled === true
  profiles.value = [
    toDraft('default', '', config.default),
    ...Object.entries(config.users || {}).map(([id, profile]) => toDraft('user', id, profile)),
    ...Object.entries(config.api_keys || {}).map(([id, profile]) => toDraft('api_key', id, profile)),
  ]
  selectedKey.value = 'default'
  savedState.value = stateString.value
}

const validation = computed<{ config: InstructionLayersConfig | null; errors: string[] }>(() => {
  const errors: string[] = []
  const config: InstructionLayersConfig = { enabled: enabled.value, default: { enabled: false, prefix: '', suffix: '', model_suffixes: {} }, users: {}, api_keys: {} }
  const seenScopes = new Set<string>()
  for (const draft of profiles.value) {
    const label = profileLabel(draft)
    const id = draft.scope === 'default' ? '' : normalizedId(draft.scopeId)
    if (id === null) errors.push(`${label}：ID 应为有效的正整数数据库 ID。`)
    const scopeKey = `${draft.scope}:${id ?? draft.scopeId}`
    if (seenScopes.has(scopeKey)) errors.push(`${label}：作用域重复，请合并或删除重复项。`)
    seenScopes.add(scopeKey)
    const seenModels = new Set<string>()
    const modelEntries: [string, string][] = []
    for (const [index, entry] of draft.modelSuffixes.entries()) {
      const model = entry.model.trim()
      if (!model) errors.push(`${label}：模型项 ${index + 1} 的模型名为空。`)
      else if (seenModels.has(model)) errors.push(`${label}：模型名“${model}”重复。`)
      seenModels.add(model)
      if (model) modelEntries.push([model, entry.text])
    }
    const profile: InstructionLayerProfile = { enabled: draft.enabled, prefix: draft.prefix, suffix: draft.suffix, model_suffixes: Object.fromEntries(modelEntries) }
    if (draft.scope === 'default') config.default = profile
    else if (id) {
      if (draft.scope === 'user') config.users[id] = profile
      else config.api_keys[id] = profile
    }
  }
  return { config: errors.length === 0 ? config : null, errors }
})
const jsonPreview = computed(() => JSON.stringify(validation.value.config, null, 2))

function addProfile(): void {
  addError.value = ''
  const id = normalizedId(newScopeId.value)
  if (!id) { addError.value = '请输入正整数数据库 ID，不是 secret Key。'; return }
  if (profiles.value.some((profile) => profile.scope === newScope.value && normalizedId(profile.scopeId) === id)) {
    addError.value = '该作用域已存在，请直接选择并编辑。'
    return
  }
  const draft = emptyProfile(newScope.value, id)
  profiles.value.push(draft)
  selectedKey.value = draft.key
  newScopeId.value = ''
}
function removeSelectedProfile(): void {
  const draft = selectedProfile.value
  if (!draft || draft.scope === 'default') return
  profiles.value = profiles.value.filter((profile) => profile.key !== draft.key)
  selectedKey.value = 'default'
}
function addModelSuffix(): void {
  selectedProfile.value?.modelSuffixes.push({ key: nextKey('model'), model: '', text: '' })
}
function removeModelSuffix(key: string): void {
  const draft = selectedProfile.value
  if (draft) draft.modelSuffixes = draft.modelSuffixes.filter((entry) => entry.key !== key)
}
async function loadConfig(): Promise<void> {
  if (loading.value && savedState.value) return
  loading.value = true
  loadFailed.value = false
  try {
    const config = await getInstructionLayers()
    if (alive) applyLoaded(config)
  } catch {
    if (alive) {
      loadFailed.value = true
      appStore.showError('分层指令读取失败，请重试；当前设置保持不变。')
    }
  } finally {
    if (alive) loading.value = false
  }
}
function reloadConfig(): void {
  if (saving.value) return
  if (dirty.value && !window.confirm('有未保存的分层指令更改。重新读取会丢弃此区草稿，继续读取吗？')) return
  void loadConfig()
}
async function saveConfig(): Promise<void> {
  const config = validation.value.config
  if (saving.value || !dirty.value || !config) return
  saving.value = true
  const previousSelection = selectedProfile.value
  const previousScope = previousSelection?.scope
  const previousId = previousSelection?.scopeId ? normalizedId(previousSelection.scopeId) : ''
  try {
    const saved = await updateInstructionLayers(config)
    if (!alive) return
    applyLoaded(saved)
    const target = profiles.value.find((profile) => profile.scope === previousScope && (profile.scope === 'default' || normalizedId(profile.scopeId) === previousId))
    if (target) selectedKey.value = target.key
    appStore.showSuccess('分层指令已保存：前置→客户端原指令→追加→任务Skills。')
  } catch {
    if (alive) appStore.showError('分层指令保存失败，草稿已保留；请检查后重试。')
  } finally {
    if (alive) saving.value = false
  }
}
function beforeUnload(event: BeforeUnloadEvent): void {
  if (!dirty.value) return
  event.preventDefault()
  event.returnValue = ''
}
onBeforeRouteLeave(() => !dirty.value || window.confirm('分层指令尚未保存。离开此页面会丢弃草稿，继续离开吗？'))
onMounted(() => {
  window.addEventListener('beforeunload', beforeUnload)
  void loadConfig()
})
onBeforeUnmount(() => {
  alive = false
  window.removeEventListener('beforeunload', beforeUnload)
})
</script>

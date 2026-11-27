<template>
  <section
    class="min-w-0 space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600"
    data-testid="codex-request-compat"
    :aria-labelledby="`${fieldId}-title`"
  >
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <h3 :id="`${fieldId}-title`" class="input-label mb-0">
          {{ t('admin.accounts.codexRequestCompat.title') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.codexRequestCompat.hint') }}
        </p>
      </div>
      <Toggle
        :model-value="modelValue.enabled"
        :aria-label="t('admin.accounts.codexRequestCompat.title')"
        data-testid="codex-request-compat-toggle"
        @update:model-value="updateDraft({ enabled: $event })"
      />
    </div>

    <div v-if="modelValue.enabled || validationError" class="space-y-3">
      <div>
        <label :for="`${fieldId}-ua`" class="input-label">
          {{ t('admin.accounts.codexRequestCompat.userAgent') }}
        </label>
        <textarea
          :id="`${fieldId}-ua`"
          :value="modelValue.userAgent"
          rows="3"
          class="input w-full resize-y break-all font-mono text-xs"
          spellcheck="false"
          autocomplete="off"
          data-testid="codex-request-compat-user-agent"
          :aria-invalid="validationError ? true : undefined"
          :aria-describedby="validationError ? `${fieldId}-error` : undefined"
          @input="updateDraft({ userAgent: ($event.target as HTMLTextAreaElement).value })"
        />
        <p
          v-if="validationError"
          :id="`${fieldId}-error`"
          class="mt-1 text-xs text-red-600 dark:text-red-400"
          role="alert"
        >
          {{ t(`admin.accounts.codexRequestCompat.${validationError}`) }}
        </p>
      </div>
      <details class="text-sm">
        <summary class="cursor-pointer text-gray-600 dark:text-gray-300">
          {{ t('admin.accounts.codexRequestCompat.relatedSettings') }}
        </summary>
        <div class="mt-3 space-y-3">
          <div class="flex items-center justify-between gap-4">
            <span :id="`${fieldId}-passthrough`">
              {{ t('admin.accounts.codexRequestCompat.passthrough') }}
            </span>
            <Toggle
              :model-value="passthroughEnabled"
              :aria-labelledby="`${fieldId}-passthrough`"
              data-testid="codex-request-compat-passthrough"
              @update:model-value="emit('update:passthroughEnabled', $event)"
            />
          </div>
          <div>
            <label :for="`${fieldId}-responses`" class="input-label">
              {{ t('admin.accounts.openai.responsesMode') }}
            </label>
            <Select
              :id="`${fieldId}-responses`"
              :model-value="responsesMode"
              :options="responsesModeOptions"
              :disabled="!textGenerationEnabled"
              :aria-label="t('admin.accounts.openai.responsesMode')"
              data-testid="codex-request-compat-responses-mode"
              @update:model-value="emit('update:responsesMode', $event as OpenAIResponsesMode)"
            />
          </div>
        </div>
      </details>
    </div>

    <div
      v-if="modelValue.enabled || effectiveUserAgent.source === 'override'"
      class="space-y-1 border-l-2 border-gray-300 pl-3 text-xs dark:border-dark-500"
      aria-live="polite"
      data-testid="codex-request-compat-effective"
    >
      <p class="font-medium text-gray-700 dark:text-gray-200">
        {{ t('admin.accounts.codexRequestCompat.effectiveUserAgent') }}
      </p>
      <p
        class="text-gray-500 dark:text-gray-400"
        :class="{ 'text-amber-700 dark:text-amber-400': effectiveUserAgent.source === 'override' }"
      >
        {{ t(`admin.accounts.codexRequestCompat.source.${effectiveUserAgent.source}`) }}
      </p>
      <code v-if="effectiveUserAgent.value" class="block break-all text-gray-700 dark:text-gray-200">
        {{ effectiveUserAgent.value }}
      </code>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Select from '@/components/common/Select.vue'
import {
  buildHeaderOverridesObject,
  readCodexRequestCompat,
  resolveCodexRequestUserAgent,
  validateCodexRequestCompat,
  type CodexRequestCompatDraft,
  type HeaderOverrideRow
} from './credentialsBuilder'
import type { OpenAIResponsesMode } from '@/types'

const props = defineProps<{
  modelValue: CodexRequestCompatDraft
  headerOverrideEnabled: boolean
  headerOverrideRows: HeaderOverrideRow[]
  passthroughEnabled: boolean
  responsesMode: OpenAIResponsesMode
  textGenerationEnabled: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: CodexRequestCompatDraft]
  'update:passthroughEnabled': [value: boolean]
  'update:responsesMode': [value: OpenAIResponsesMode]
}>()

const { t } = useI18n()
const fieldId = useId()
const effectiveUserAgent = computed(() => resolveCodexRequestUserAgent(
  props.modelValue, props.headerOverrideEnabled, props.headerOverrideRows
))
const validationError = computed(() => validateCodexRequestCompat(props.modelValue))
const responsesModeOptions = computed(() => [
  { value: 'auto', label: t('admin.accounts.openai.responsesModeAuto') },
  { value: 'force_responses', label: t('admin.accounts.openai.responsesModeForceResponses') },
  { value: 'force_chat_completions', label: t('admin.accounts.openai.responsesModeForceChatCompletions') }
])

function updateDraft(patch: Partial<CodexRequestCompatDraft>) {
  const current = props.modelValue
  const userAgent = patch.enabled && !current.configured
    ? readCodexRequestCompat({
      header_override_enabled: props.headerOverrideEnabled,
      header_overrides: buildHeaderOverridesObject(props.headerOverrideRows),
      user_agent: current.userAgent
    }).userAgent
    : current.userAgent
  emit('update:modelValue', { ...current, userAgent, ...patch, configured: true })
}
</script>

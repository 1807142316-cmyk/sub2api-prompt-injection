<template>
  <div class="prompt-editor min-w-0">
    <div class="flex min-h-10 flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-3 py-2 dark:border-dark-600">
      <div class="min-w-0">
        <label :for="id" class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ label }}</label>
        <span class="ml-2 text-xs text-gray-500">{{ modelValue.length.toLocaleString() }} 字符</span>
      </div>
      <button type="button" class="editor-icon" title="放大编辑" :aria-label="`放大编辑${label}`" :disabled="disabled" @click="expanded = true">
        <Icon name="externalLink" size="sm" />
      </button>
    </div>
    <textarea
      :id="id"
      :value="modelValue"
      :disabled="disabled"
      :placeholder="placeholder"
      :rows="rows"
      class="prompt-textarea w-full resize-y"
      spellcheck="false"
      @input="updateValue"
    ></textarea>

    <BaseDialog :show="expanded" :title="label" width="full" @close="expanded = false">
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <div class="relative min-w-0 flex-1">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-3 text-gray-400" />
            <input v-model="search" class="input w-full pl-9" placeholder="查找正文" :aria-label="`在${label}中查找`" @keydown.enter.prevent="moveMatch(1)" />
          </div>
          <span class="min-w-16 text-center text-xs text-gray-500" aria-live="polite">{{ matchLabel }}</span>
          <button type="button" class="editor-icon" title="上一处" aria-label="上一处匹配" :disabled="!matches.length" @click="moveMatch(-1)"><Icon name="chevronUp" size="sm" /></button>
          <button type="button" class="editor-icon" title="下一处" aria-label="下一处匹配" :disabled="!matches.length" @click="moveMatch(1)"><Icon name="chevronDown" size="sm" /></button>
        </div>
        <textarea
          ref="expandedTextarea"
          :value="modelValue"
          :disabled="disabled"
          :aria-label="`${label}放大编辑区`"
          :placeholder="placeholder"
          class="prompt-textarea expanded-textarea w-full resize-none rounded-md border border-gray-200 dark:border-dark-600"
          spellcheck="false"
          @input="updateValue"
        ></textarea>
      </div>
      <template #footer>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <span class="text-xs text-gray-500">{{ modelValue.length.toLocaleString() }} 字符 · {{ lineCount.toLocaleString() }} 行</span>
          <button type="button" class="btn btn-primary" @click="expanded = false"><Icon name="check" size="sm" class="mr-1.5" />完成编辑</button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  id: string
  label: string
  modelValue: string
  placeholder?: string
  disabled?: boolean
  rows?: number
}>(), { placeholder: '', disabled: false, rows: 16 })
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const expanded = ref(false)
const expandedTextarea = ref<HTMLTextAreaElement>()
const search = ref('')
const matchIndex = ref(-1)
const lineCount = computed(() => props.modelValue.split('\n').length)
const matches = computed(() => {
  const result: number[] = []
  if (!search.value) return result
  const haystack = props.modelValue
  const needle = search.value
  let cursor = haystack.indexOf(needle)
  while (cursor !== -1 && result.length < 10000) {
    result.push(cursor)
    cursor = haystack.indexOf(needle, cursor + Math.max(1, needle.length))
  }
  return result
})
const matchLabel = computed(() => !search.value ? '' : matches.value.length
  ? `${matchIndex.value >= 0 ? matchIndex.value + 1 : 0} / ${matches.value.length}`
  : '未找到')
watch([search, () => props.modelValue], () => { matchIndex.value = -1 })
watch(expanded, async (value) => {
  if (!value) return
  await nextTick()
  expandedTextarea.value?.focus()
})
function updateValue(event: Event): void {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
}
function moveMatch(direction: number): void {
  const textarea = expandedTextarea.value
  if (!textarea || !matches.value.length) return
  const next = matchIndex.value < 0
    ? direction > 0 ? 0 : matches.value.length - 1
    : (matchIndex.value + direction + matches.value.length) % matches.value.length
  matchIndex.value = next
  const offset = matches.value[next]
  textarea.focus()
  textarea.setSelectionRange(offset, offset + search.value.length)
  const precedingLines = props.modelValue.slice(0, offset).split('\n')
  const style = getComputedStyle(textarea)
  const lineHeight = Number.parseFloat(style.lineHeight) || 24
  const approximateColumns = Math.max(20, Math.floor(textarea.clientWidth / 8))
  const visualLines = precedingLines.reduce((total, line) => total + Math.max(1, Math.ceil(line.length / approximateColumns)), 0) - 1
  textarea.scrollTop = Math.max(0, visualLines * lineHeight - textarea.clientHeight / 3)
}
</script>

<style scoped>
.prompt-editor {
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-radius: 6px;
}
.dark .prompt-editor { border-color: rgb(55 65 81); }
.prompt-textarea {
  display: block;
  min-height: 18rem;
  padding: 1rem;
  border: 0;
  outline: none;
  background: transparent;
  color: rgb(55 65 81);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 14px;
  line-height: 1.75;
  letter-spacing: 0;
  tab-size: 2;
}
.prompt-textarea:focus { box-shadow: inset 0 0 0 2px rgb(20 184 166 / 35%); }
.dark .prompt-textarea { color: rgb(229 231 235); }
.expanded-textarea { height: 58vh; min-height: 16rem; }
.editor-icon {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  color: rgb(107 114 128);
}
.editor-icon:hover { background: rgb(148 163 184 / 15%); }
.editor-icon:disabled { opacity: .35; }
.editor-icon:focus-visible { outline: 2px solid rgb(20 184 166); outline-offset: 2px; }
</style>

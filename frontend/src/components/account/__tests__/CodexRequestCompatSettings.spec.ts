import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CodexRequestCompatSettings from '../CodexRequestCompatSettings.vue'
import { readCodexRequestCompat } from '../credentialsBuilder'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

function mountSettings(overrides: Record<string, unknown> = {}) {
  return mount(CodexRequestCompatSettings, {
    props: {
      modelValue: readCodexRequestCompat(),
      headerOverrideEnabled: false,
      headerOverrideRows: [],
      passthroughEnabled: false,
      responsesMode: 'auto',
      textGenerationEnabled: true,
      ...overrides
    },
    global: { stubs: { Select: true } }
  })
}

describe('CodexRequestCompatSettings', () => {
  it('enables compatibility using the existing custom UA without changing transport', async () => {
    const wrapper = mountSettings({
      headerOverrideEnabled: true,
      headerOverrideRows: [{ name: 'User-Agent', value: 'ProviderAgent/1' }]
    })
    await wrapper.get('[data-testid="codex-request-compat-toggle"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[
      { enabled: true, configured: true, userAgent: 'ProviderAgent/1' }
    ]])
    expect(wrapper.emitted('update:passthroughEnabled')).toBeUndefined()
    expect(wrapper.emitted('update:responsesMode')).toBeUndefined()
    wrapper.unmount()
  })

  it('keeps a saved UA when disabled and edits only its own draft', async () => {
    const draft = { enabled: true, configured: true, userAgent: 'SavedAgent/1' }
    const wrapper = mountSettings({ modelValue: draft })
    await wrapper.get('[data-testid="codex-request-compat-user-agent"]').setValue('EditedAgent/2')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ ...draft, userAgent: 'EditedAgent/2' }])
    await wrapper.get('[data-testid="codex-request-compat-toggle"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[1]).toEqual([{ ...draft, enabled: false }])
    expect(draft.userAgent).toBe('SavedAgent/1')
    wrapper.unmount()
  })

  it('shows override priority and falls back to compatibility for an empty override', async () => {
    const wrapper = mountSettings({
      modelValue: { enabled: true, configured: true, userAgent: 'CompatAgent/1' },
      headerOverrideEnabled: true,
      headerOverrideRows: [{ name: 'User-Agent', value: 'PriorityAgent/2' }]
    })
    expect(wrapper.get('[data-testid="codex-request-compat-effective"]').text())
      .toContain('admin.accounts.codexRequestCompat.source.override')
    expect(wrapper.get('[data-testid="codex-request-compat-effective"]').text()).toContain('PriorityAgent/2')
    await wrapper.setProps({ headerOverrideRows: [{ name: 'User-Agent', value: '' }] })
    expect(wrapper.get('[data-testid="codex-request-compat-effective"]').text()).toContain('CompatAgent/1')
    wrapper.unmount()
  })

  it('lets users repair a bad disabled UA and shows accessible validation', () => {
    const wrapper = mountSettings({
      modelValue: { enabled: false, configured: true, userAgent: 'Bad\nAgent' }
    })
    expect(wrapper.get('textarea').attributes('aria-invalid')).toBe('true')
    expect(wrapper.get('[role="alert"]').text()).toContain('userAgentInvalid')
    wrapper.unmount()
  })

  it('changes related transport settings independently', async () => {
    const wrapper = mountSettings({
      modelValue: { enabled: true, configured: true, userAgent: 'CompatAgent/1' }
    })
    await wrapper.get('[data-testid="codex-request-compat-passthrough"]').trigger('click')
    expect(wrapper.emitted('update:passthroughEnabled')).toEqual([[true]])
    wrapper.getComponent({ name: 'Select' }).vm.$emit('update:modelValue', 'force_responses')
    expect(wrapper.emitted('update:responsesMode')).toEqual([['force_responses']])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })
})

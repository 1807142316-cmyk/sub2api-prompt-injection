import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { InstructionLayersConfig } from '@/api/admin/instruction-layers'
import InstructionLayersPanel from '../InstructionLayersPanel.vue'

const mocks = vi.hoisted(() => ({
  get: vi.fn(), put: vi.fn(), users: vi.fn(), keys: vi.fn(), groups: vi.fn(),
  success: vi.fn(), error: vi.fn(), leave: vi.fn(),
}))
vi.mock('@/api/admin/instruction-layers', () => ({
  getInstructionLayers: mocks.get, updateInstructionLayers: mocks.put,
}))
vi.mock('@/api/admin/usage', () => ({ searchUsers: mocks.users, searchApiKeys: mocks.keys }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: mocks.groups }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showSuccess: mocks.success, showError: mocks.error }) }))
vi.mock('vue-router', () => ({ onBeforeRouteLeave: mocks.leave }))
vi.mock('@/composables/useModelWhitelist', () => ({ allModels: [{ value: 'gpt-6-astra', label: 'gpt-6-astra' }] }))

const source = (): InstructionLayersConfig => ({
  enabled: true,
  default: { enabled: true, prefix: '  原前置\n', suffix: '\t原后置\n', model_suffixes: { exact: '', '*': '  兜底\n' } },
  users: { '7': { enabled: true, prefix: '', suffix: 'user tail', model_suffixes: {} } },
  api_keys: { '9': { enabled: false, prefix: 'private key prefix', suffix: '', model_suffixes: {} } },
  groups: {},
})
const SelectStub = {
  props: ['modelValue', 'options', 'id', 'ariaLabel'],
  emits: ['update:modelValue', 'search'],
  template: '<select :id="id" :aria-label="ariaLabel" :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :value="option.value" :key="String(option.value)">{{ option.label }}</option></select>',
}
const ToggleStub = {
  props: ['modelValue', 'id'], emits: ['update:modelValue'],
  template: '<input :id="id" type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />',
}
const EditorStub = {
  props: ['id', 'modelValue', 'label'], emits: ['update:modelValue'],
  template: '<div><label :for="id">{{ label }}</label><textarea :id="id" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)"></textarea></div>',
}
async function setup() {
  const wrapper = mount(InstructionLayersPanel, {
    global: { stubs: { Select: SelectStub, Toggle: ToggleStub, PromptTextEditor: EditorStub, Icon: true } },
  })
  await flushPromises()
  return wrapper
}

describe('InstructionLayersPanel editing flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.get.mockResolvedValue(source())
    mocks.put.mockImplementation(async (value: InstructionLayersConfig) => structuredClone(value))
    mocks.users.mockResolvedValue([])
    mocks.keys.mockResolvedValue([{ id: 9, name: '测试客户端', user_id: 7 }])
    mocks.groups.mockResolvedValue([
      { id: 3, name: '日常对话', status: 'active' },
      { id: 4, name: '代码开发', status: 'active' },
      { id: 5, name: '备用分组', status: 'inactive' },
    ])
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  it('saves a body edit without altering hidden scopes, whitespace, or blank model overrides', async () => {
    const wrapper = await setup()
    expect(wrapper.find('[data-testid="instruction-layers-save"]').attributes('disabled')).toBeDefined()
    await wrapper.get('textarea#instruction-prefix-default').setValue(' \n 新前置\t')
    await wrapper.get('[data-testid="instruction-layers-save"]').trigger('click')
    await flushPromises()
    const submitted = mocks.put.mock.calls[0][0] as InstructionLayersConfig
    const expected = source()
    expected.default.prefix = ' \n 新前置\t'
    expect(submitted).toEqual(expected)
    expect(wrapper.text()).toContain('已保存')
    expect(wrapper.find('[data-testid="instruction-layers-save"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('represents a blank exact override as an explicit no-append choice', async () => {
    const wrapper = await setup()
    const rule = wrapper.get('[data-testid="model-rule-0"]')
    expect((rule.find('select[id^="instruction-model-mode-"]').element as HTMLSelectElement).value).toBe('none')
    expect(rule.find('textarea').exists()).toBe(false)
    await rule.find('select[id^="instruction-model-mode-"]').setValue('custom')
    expect(rule.find('textarea').exists()).toBe(true)
    expect(wrapper.text()).toContain('请填写“exact”的专用内容')
    expect(wrapper.find('[data-testid="instruction-layers-save"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('creates an independent copied client configuration without requesting key secrets', async () => {
    const wrapper = await setup()
    await wrapper.get('#instruction-new-scope').setValue('api_key')
    await flushPromises()
    const manual = wrapper.findAll('button').find((button) => button.text() === '按编号添加')!
    await manual.trigger('click')
    await wrapper.get('#instruction-new-target').setValue('12')
    await wrapper.findAll('button').find((button) => button.text() === '添加专属配置')!.trigger('click')
    const editor = wrapper.get('textarea[id^="instruction-prefix-api_key"]')
    expect((editor.element as HTMLTextAreaElement).value).toBe(source().default.prefix)
    await editor.setValue('client only')
    await wrapper.get('[data-testid="instruction-layers-save"]').trigger('click')
    await flushPromises()
    const submitted = mocks.put.mock.calls[0][0] as InstructionLayersConfig
    expect(submitted.default).toEqual(source().default)
    expect(submitted.api_keys['12']).toEqual({ ...source().default, prefix: 'client only' })
    expect(mocks.keys).toHaveBeenCalledWith(undefined, '')
    wrapper.unmount()
  })

  it('previews disabled client scope without falling back to its user', async () => {
    const wrapper = await setup()
    const picker = wrapper.get('#instruction-profile-picker')
    const option = picker.findAll('option').find((item) => item.text().includes('测试客户端'))!
    await picker.setValue(option.attributes('value'))
    expect((wrapper.get('#instruction-preview-user').element as HTMLSelectElement).value).toBe('7')
    expect(wrapper.text()).toContain('当前配置已停用，不补用其他配置')
    expect(wrapper.text()).toContain('不添加前置内容')
    expect(wrapper.text()).toContain('不追加后置内容')
    wrapper.unmount()
  })

  it('keeps unsaved text when reload is cancelled and registers leave protection', async () => {
    const wrapper = await setup()
    await wrapper.get('textarea#instruction-prefix-default').setValue('unsaved')
    vi.mocked(window.confirm).mockReturnValue(false)
    await wrapper.get('[aria-label="重新读取已保存配置"]').trigger('click')
    expect(mocks.get).toHaveBeenCalledTimes(1)
    expect((wrapper.get('textarea#instruction-prefix-default').element as HTMLTextAreaElement).value).toBe('unsaved')
    const guard = mocks.leave.mock.calls[0][0] as () => boolean
    expect(guard()).toBe(false)
    wrapper.unmount()
  })

  it('retains the draft after a save failure and can retry successfully', async () => {
    const wrapper = await setup()
    mocks.put.mockRejectedValueOnce(new Error('connection unavailable'))
    await wrapper.get('textarea#instruction-prefix-default').setValue('keep this draft')
    await wrapper.get('[data-testid="instruction-layers-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('草稿已保留')
    expect((wrapper.get('textarea#instruction-prefix-default').element as HTMLTextAreaElement).value).toBe('keep this draft')
    await wrapper.get('[data-testid="instruction-layers-save"]').trigger('click')
    await flushPromises()
    expect(mocks.put).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('已保存')
    wrapper.unmount()
  })

  it('does not expose a writable empty form when configuration loading fails', async () => {
    mocks.get.mockRejectedValueOnce(new Error('offline'))
    const wrapper = await setup()
    expect(wrapper.text()).toContain('暂时无法读取')
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(wrapper.find('[data-testid="instruction-layers-save"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows every group by name with inherit as the initial mode and copies an independent profile', async () => {
    const wrapper = await setup()
    expect(wrapper.findAll('[data-testid^="instruction-group-"]')).toHaveLength(3)
    expect(wrapper.get('[data-testid="instruction-group-5"]').text()).toContain('分组已停用')
    const mode = wrapper.get('#instruction-group-mode-3')
    expect((mode.element as HTMLSelectElement).value).toBe('inherit')
    await mode.setValue('custom')
    const editor = wrapper.get('textarea[id^="instruction-prefix-group-"]')
    expect((editor.element as HTMLTextAreaElement).value).toBe(source().default.prefix)
    await editor.setValue('  日常专用\n')
    await wrapper.get('[data-testid="instruction-layers-save"]').trigger('click')
    await flushPromises()
    const submitted = mocks.put.mock.calls[0][0] as InstructionLayersConfig
    expect(submitted.groups!['3']).toEqual({ ...source().default, enabled: true, prefix: '  日常专用\n' })
    expect(submitted.default).toEqual(source().default)
    wrapper.unmount()
  })

  it('previews group opt-out ahead of user and key choices and retains text when toggled back', async () => {
    const stored = source()
    stored.groups = { '3': { ...stored.default, prefix: 'group retained' } }
    stored.api_keys['9'].enabled = true
    mocks.get.mockResolvedValueOnce(stored)
    const wrapper = await setup()
    await wrapper.get('#instruction-group-mode-3').setValue('disabled')
    await wrapper.get('#instruction-preview-user').setValue('7')
    await wrapper.get('#instruction-preview-client').setValue('9')
    expect(wrapper.text()).toContain('该组不注入，已关闭前置、后置与自动 Skills')
    expect(wrapper.text()).toContain('不添加前置内容')
    await wrapper.get('#instruction-layers-master').setValue(false)
    expect(wrapper.text()).toContain('该组不注入，已关闭前置、后置与自动 Skills')
    await wrapper.get('#instruction-group-mode-3').setValue('custom')
    expect((wrapper.get('textarea[id^="instruction-prefix-group-"]').element as HTMLTextAreaElement).value).toBe('group retained')
    wrapper.unmount()
  })

  it('confirms restoring group inheritance and leaves other scopes intact on save', async () => {
    const stored = source()
    stored.groups = { '3': { ...stored.default, prefix: 'group retained' } }
    mocks.get.mockResolvedValueOnce(stored)
    const wrapper = await setup()
    const mode = wrapper.get('#instruction-group-mode-3')
    vi.mocked(window.confirm).mockReturnValueOnce(false)
    await mode.setValue('inherit')
    expect((mode.element as HTMLSelectElement).value).toBe('custom')
    await mode.setValue('inherit')
    await wrapper.get('[data-testid="instruction-layers-save"]').trigger('click')
    await flushPromises()
    expect((mocks.put.mock.calls[0][0] as InstructionLayersConfig)).toEqual({ ...stored, groups: {} })
    wrapper.unmount()
  })

  it('preserves saved group scopes when the optional name lookup fails', async () => {
    const stored = source()
    stored.groups = { '3': { ...stored.default, prefix: 'group retained' } }
    mocks.get.mockResolvedValueOnce(stored)
    mocks.groups.mockRejectedValueOnce(new Error('offline'))
    const wrapper = await setup()
    expect(wrapper.text()).toContain('分组名称暂时无法读取')
    expect(wrapper.get('[data-testid="instruction-group-3"]').text()).toContain('分组编号 3')
    await wrapper.get('textarea#instruction-prefix-default').setValue('default edit')
    await wrapper.get('[data-testid="instruction-layers-save"]').trigger('click')
    await flushPromises()
    expect((mocks.put.mock.calls[0][0] as InstructionLayersConfig).groups).toEqual(stored.groups)
    wrapper.unmount()
  })
})

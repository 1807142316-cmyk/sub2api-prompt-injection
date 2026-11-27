import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PromptTextEditor from '../PromptTextEditor.vue'

const DialogStub = {
  props: ['show', 'title'],
  template: '<section v-if="show" role="dialog"><slot /><slot name="footer" /></section>',
}
function setup(modelValue = '') {
  return mount(PromptTextEditor, {
    attachTo: document.body,
    props: { id: 'prompt', label: '前置提示词', modelValue },
    global: { stubs: { BaseDialog: DialogStub, Icon: true } },
  })
}

describe('PromptTextEditor', () => {
  it('emits inline and expanded edits without trimming whitespace', async () => {
    const wrapper = setup()
    const text = ' \n保留制表符\t\n '
    await wrapper.get('textarea#prompt').setValue(text)
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([text])
    await wrapper.setProps({ modelValue: text })
    await wrapper.get('[aria-label="放大编辑前置提示词"]').trigger('click')
    const expanded = wrapper.get('textarea[aria-label="前置提示词放大编辑区"]')
    expect((expanded.element as HTMLTextAreaElement).value).toBe(text)
    await expanded.setValue(`${text}\t`)
    expect(wrapper.emitted('update:modelValue')?.[1]).toEqual([`${text}\t`])
    wrapper.unmount()
  })

  it('finds exact offsets after Unicode text and navigates matches in both directions', async () => {
    const text = 'İx needle\nneedle'
    const wrapper = setup(text)
    await wrapper.get('[aria-label="放大编辑前置提示词"]').trigger('click')
    await wrapper.get('[aria-label="在前置提示词中查找"]').setValue('needle')
    const expanded = wrapper.get('textarea[aria-label="前置提示词放大编辑区"]').element as HTMLTextAreaElement
    await wrapper.get('[aria-label="下一处匹配"]').trigger('click')
    expect(expanded.selectionStart).toBe(text.indexOf('needle'))
    expect(expanded.selectionEnd).toBe(text.indexOf('needle') + 6)
    await wrapper.get('[aria-label="下一处匹配"]').trigger('click')
    expect(expanded.selectionStart).toBe(text.lastIndexOf('needle'))
    await wrapper.get('[aria-label="上一处匹配"]').trigger('click')
    expect(expanded.selectionStart).toBe(text.indexOf('needle'))
    expect(wrapper.text()).toContain('1 / 2')
    wrapper.unmount()
  })

  it('disables both editing surfaces when saving starts', async () => {
    const wrapper = setup('draft')
    await wrapper.get('[aria-label="放大编辑前置提示词"]').trigger('click')
    await wrapper.setProps({ disabled: true })
    expect(wrapper.findAll('textarea').every((textarea) => textarea.attributes('disabled') !== undefined)).toBe(true)
    expect(wrapper.get('[aria-label="放大编辑前置提示词"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
})

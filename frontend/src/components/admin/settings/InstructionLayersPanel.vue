<template>
  <section class="instruction-panel min-w-0" aria-labelledby="instruction-layers-title" data-testid="instruction-layers-panel">
    <div v-if="loading" class="flex min-h-64 items-center justify-center gap-2 text-sm text-gray-500" role="status">
      <Icon name="refresh" size="sm" class="animate-spin" />正在读取提示词配置…
    </div>
    <div v-else-if="loadFailed" class="border-l-2 border-red-400 px-4 py-5" role="alert">
      <p class="text-sm text-red-700 dark:text-red-300">提示词配置暂时无法读取。</p>
      <button type="button" class="btn btn-secondary btn-sm mt-3" @click="loadConfig"><Icon name="refresh" size="sm" class="mr-1.5" />重试</button>
    </div>
    <template v-else>
      <fieldset :disabled="saving" class="min-w-0 space-y-7">
        <div class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-200 pb-5 dark:border-dark-600">
          <div class="min-w-0">
            <h2 id="instruction-layers-title" class="text-base font-semibold text-gray-900 dark:text-white">自定义提示词</h2>
            <p class="mt-1 flex items-center gap-2 text-sm" :class="enabled ? 'text-emerald-700 dark:text-emerald-400' : 'text-gray-500'">
              <span class="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true"></span>
              {{ enabled ? '已启用前置与后置提示词' : '前置与后置提示词已暂停' }}
              <span v-if="dirty" class="text-amber-700 dark:text-amber-400">· 尚未保存</span>
            </p>
          </div>
          <div class="flex items-center gap-3">
            <label for="instruction-layers-master" class="text-sm text-gray-700 dark:text-gray-300">启用自定义提示词</label>
            <Toggle id="instruction-layers-master" v-model="enabled" aria-label="启用自定义提示词" />
          </div>
        </div>
        <div v-if="!enabled" class="border-l-2 border-amber-400 pl-3 text-sm text-gray-600 dark:text-gray-300">
          当前暂停追加前置与后置内容。任务 Skills 仍按原规则生效；选择“不注入提示词”的分组也会关闭自动 Skills。
        </div>

        <section class="min-w-0 border-b border-gray-200 pb-6 dark:border-dark-600" aria-labelledby="instruction-groups-title">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 id="instruction-groups-title" class="text-base font-semibold text-gray-900 dark:text-white">按分组设置</h3>
              <p class="mt-1 text-sm leading-6 text-gray-500">每组可继承全局或使用独立内容。“不注入提示词”会关闭该组的前置、后置和自动 Skills。</p>
            </div>
            <input v-if="groupRows.length > 8" v-model="groupFilter" class="input max-w-64" type="search" placeholder="筛选分组名称" aria-label="筛选分组" />
          </div>
          <p v-if="groupsLoading" class="mt-4 text-sm text-gray-500" role="status">正在读取分组…</p>
          <div v-if="groupsError" class="mt-4 flex flex-wrap items-center gap-3 text-sm text-amber-700 dark:text-amber-300" role="alert">
            <span>{{ groupsError }}</span><button type="button" class="btn btn-secondary btn-sm" @click="loadGroups"><Icon name="refresh" size="sm" class="mr-1.5" />重试</button>
          </div>
          <div v-if="filteredGroups.length" class="mt-4 divide-y divide-gray-100 border-t border-gray-100 dark:divide-dark-700 dark:border-dark-700">
            <div v-for="group in filteredGroups" :key="group.id" class="grid min-w-0 grid-cols-[minmax(0,1fr)_2.5rem] items-center gap-3 py-3 sm:grid-cols-[minmax(0,1fr)_210px_2.5rem]" :data-testid="`instruction-group-${group.id}`">
              <div class="col-span-2 min-w-0 sm:col-span-1">
                <p class="break-words text-sm font-medium text-gray-800 dark:text-gray-100">{{ group.name }}</p>
                <p class="mt-1 text-xs text-gray-500">编号 {{ group.id }}<span v-if="group.status !== 'active'"> · {{ group.status === 'unknown' ? '分组信息未找到' : '分组已停用' }}</span></p>
              </div>
              <select :id="`instruction-group-mode-${group.id}`" :value="groupMode(group.id)" class="input w-full" :aria-label="`${group.name}的注入方式`" @change="changeGroupMode(group.id, $event)">
                <option value="inherit">继承全局</option>
                <option value="custom">使用独立提示词</option>
                <option value="disabled">不注入提示词（含 Skills）</option>
              </select>
              <button type="button" class="icon-button" :disabled="groupMode(group.id) !== 'custom'" :title="`编辑${group.name}的提示词`" :aria-label="`编辑${group.name}的提示词`" @click="editGroup(group.id)"><Icon name="edit" size="sm" /></button>
            </div>
          </div>
          <p v-else-if="!groupsLoading && !groupsError" class="mt-4 text-sm text-gray-500">{{ groupFilter ? '没有匹配的分组。' : '暂无分组，当前使用全局默认。' }}</p>
          <p v-if="groupError" class="mt-3 text-sm text-red-600" role="alert">{{ groupError }}</p>
        </section>

        <div ref="editorSection" class="grid min-w-0 gap-6 xl:grid-cols-[minmax(0,1fr)_260px]">
          <div class="min-w-0 space-y-5">
            <div class="flex flex-wrap items-end justify-between gap-3">
              <div class="min-w-0 flex-1">
                <label for="instruction-profile-picker" class="input-label">正在编辑</label>
                <Select id="instruction-profile-picker" :model-value="selectedKey" :options="profileOptions" :searchable="profiles.length > 5" aria-label="选择提示词配置" @update:model-value="selectProfile" />
              </div>
              <button v-if="selectedProfile?.scope !== 'default'" type="button" class="btn btn-secondary btn-sm mb-0.5" @click="selectedKey = 'default'">返回全局默认</button>
            </div>
            <template v-if="selectedProfile">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="min-w-0 text-sm text-gray-500 dark:text-gray-400">
                  {{ selectedProfile.scope === 'default' ? '没有匹配到专属配置时，使用这份全局内容。' : '此对象使用整份专属配置，留空的内容也不会补用全局默认。' }}
                </div>
                <div class="flex shrink-0 items-center gap-2">
                  <label :for="`instruction-profile-enabled-${selectedProfile.key}`" class="text-sm text-gray-700 dark:text-gray-300">{{ selectedProfile.scope === 'group' ? '允许本页提示词与自动 Skills' : '使用这份配置' }}</label>
                  <Toggle :id="`instruction-profile-enabled-${selectedProfile.key}`" v-model="selectedProfile.enabled" :aria-label="`启用${profileLabel(selectedProfile)}`" />
                </div>
              </div>
              <p v-if="!selectedProfile.enabled" class="border-l-2 border-amber-400 pl-3 text-sm text-amber-800 dark:text-amber-300">
                {{ selectedProfile.scope === 'group' ? '该组不注入本页前置、后置或自动 Skills；用户和客户端专属配置也不会覆盖这一设置。客户端原指令及协议兼容处理沿用原有设置。' : selectedProfile.scope === 'default' ? '全局默认已停用；专属配置按各自的开关生效。' : '此对象不追加本页提示词，也不会改用分组或全局默认配置。' }}
              </p>
              <div class="min-w-0">
                <div class="flex items-center border-b border-gray-200 dark:border-dark-600" role="tablist" aria-label="提示词位置">
                  <button id="instruction-prefix-tab" type="button" role="tab" :aria-selected="editorTab === 'prefix'" aria-controls="instruction-body-panel" :tabindex="editorTab === 'prefix' ? 0 : -1" :class="['position-tab', editorTab === 'prefix' && 'position-tab-active']" @click="editorTab = 'prefix'" @keydown.right.prevent="changeEditorTab('suffix')">
                    前置提示词 <span class="ml-1.5 text-xs font-normal opacity-60">{{ selectedProfile.prefix.length.toLocaleString() }}</span>
                  </button>
                  <button id="instruction-suffix-tab" type="button" role="tab" :aria-selected="editorTab === 'suffix'" aria-controls="instruction-body-panel" :tabindex="editorTab === 'suffix' ? 0 : -1" :class="['position-tab', editorTab === 'suffix' && 'position-tab-active']" @click="editorTab = 'suffix'" @keydown.left.prevent="changeEditorTab('prefix')">
                    后置提示词 <span class="ml-1.5 text-xs font-normal opacity-60">{{ selectedProfile.suffix.length.toLocaleString() }}</span>
                  </button>
                </div>
                <div id="instruction-body-panel" role="tabpanel" :aria-labelledby="`instruction-${editorTab}-tab`" class="space-y-3 pt-4">
                  <p class="text-sm text-gray-500 dark:text-gray-400">{{ editorTab === 'prefix' ? '放在客户端原指令之前，当前配置下的所有模型共用。' : '放在客户端原指令之后；模型专用规则可替换这段内容。' }}</p>
                  <PromptTextEditor
                    :id="`instruction-${editorTab}-${selectedProfile.key}`" :key="`${selectedProfile.key}-${editorTab}`"
                    v-model="selectedProfile[editorTab]" :label="editorTab === 'prefix' ? '前置提示词' : '后置提示词'"
                    :placeholder="editorTab === 'prefix' ? '填写需要提前加入的规则，留空则不添加。' : '填写需要补充的规则，留空则不添加。'" :disabled="saving"
                  />
                </div>
              </div>
            </template>
          </div>
          <aside class="min-w-0 border-t border-gray-200 pt-5 dark:border-dark-600 xl:border-l xl:border-t-0 xl:pl-6 xl:pt-0" aria-label="当前配置摘要">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">当前配置</h3>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex justify-between gap-3"><dt class="text-gray-500">{{ dirty ? '草稿状态' : '状态' }}</dt><dd :class="enabled && selectedProfile?.enabled ? 'text-emerald-700 dark:text-emerald-400' : 'text-gray-500'">{{ enabled && selectedProfile?.enabled ? '启用' : '暂停' }}</dd></div>
              <div class="flex justify-between gap-3"><dt class="text-gray-500">前置提示词</dt><dd>{{ (selectedProfile?.prefix.length || 0).toLocaleString() }} 字符</dd></div>
              <div class="flex justify-between gap-3"><dt class="text-gray-500">后置提示词</dt><dd>{{ (selectedProfile?.suffix.length || 0).toLocaleString() }} 字符</dd></div>
              <div class="flex justify-between gap-3"><dt class="text-gray-500">模型专用规则</dt><dd>{{ selectedProfile?.modelSuffixes.length || 0 }} 条</dd></div>
            </dl>
            <div class="mt-5 border-t border-gray-200 pt-4 dark:border-dark-600">
              <p class="text-xs font-medium text-gray-700 dark:text-gray-300">追加顺序</p>
              <ol class="mt-3 space-y-2 text-xs text-gray-500">
                <li class="flex items-center gap-2"><span class="order-number">1</span>前置提示词</li>
                <li class="flex items-center gap-2"><span class="order-number">2</span>客户端原指令</li>
                <li class="flex items-center gap-2"><span class="order-number">3</span>后置提示词</li>
                <li class="flex items-center gap-2"><span class="order-number">4</span>按任务匹配的 Skills</li>
              </ol>
            </div>
            <button type="button" class="mt-5 inline-flex items-center gap-1.5 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="openPreview"><Icon name="eye" size="sm" />查看生效预览</button>
          </aside>
        </div>

        <section v-if="selectedProfile" class="min-w-0 border-t border-gray-200 pt-6 dark:border-dark-600" aria-labelledby="instruction-model-rules-title">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 id="instruction-model-rules-title" class="text-base font-semibold text-gray-900 dark:text-white">模型专用规则</h3>
              <p class="mt-1 text-sm text-gray-500">仅替换对应模型的后置提示词，前置提示词保持共用。</p>
              <p class="mt-1 text-xs text-gray-500">使用模型别名的分组，请按映射后的模型名称配置。实时连接的客户端需在保存后重新连接。</p>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="addModelSuffix"><Icon name="plus" size="sm" class="mr-1.5" />添加模型规则</button>
          </div>
          <p v-if="!selectedProfile.modelSuffixes.length" class="py-6 text-sm text-gray-500">所有模型都使用上方的后置提示词。</p>
          <div v-else class="mt-4 divide-y divide-gray-200 dark:divide-dark-600">
            <div v-for="(rule, index) in selectedProfile.modelSuffixes" :key="rule.key" class="min-w-0 space-y-3 py-5 first:pt-0" :data-testid="`model-rule-${index}`">
              <div class="grid min-w-0 gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
                <div class="min-w-0">
                  <label :for="`instruction-model-${rule.key}`" class="input-label">模型名称</label>
                  <Select :id="`instruction-model-${rule.key}`" :model-value="rule.model" :options="modelOptions" searchable creatable creatable-prefix="使用模型" placeholder="搜索或输入模型名称" search-placeholder="输入完整模型名称" :aria-label="`第 ${index + 1} 条规则的模型`" @update:model-value="setRuleModel(rule, $event)" />
                </div>
                <div class="min-w-0">
                  <label :for="`instruction-model-mode-${rule.key}`" class="input-label">该模型的后置提示词</label>
                  <select :id="`instruction-model-mode-${rule.key}`" v-model="rule.mode" class="input w-full" :aria-label="`第 ${index + 1} 条规则的处理方式`">
                    <option value="custom">使用专用内容</option>
                    <option value="none">不追加后置提示词</option>
                  </select>
                </div>
                <button type="button" class="icon-button self-end text-red-500" :title="`删除第 ${index + 1} 条模型规则`" :aria-label="`删除第 ${index + 1} 条模型规则`" @click="removeModelSuffix(rule)"><Icon name="trash" size="sm" /></button>
              </div>
              <PromptTextEditor v-if="rule.mode === 'custom'" :id="`instruction-model-body-${rule.key}`" v-model="rule.text" :label="`${rule.model === '*' ? '其余模型' : rule.model || '该模型'}的专用内容`" placeholder="填写该模型的完整后置提示词。" :rows="8" :disabled="saving" />
              <p v-else class="border-l-2 border-gray-300 pl-3 text-sm text-gray-500">本页不再为该模型追加后置内容，前置提示词及其他处理沿用原有设置。</p>
            </div>
          </div>
        </section>

        <details :open="scopeOpen" class="border-t border-gray-200 pt-5 dark:border-dark-600" @toggle="scopeOpen = ($event.target as HTMLDetailsElement).open">
          <summary class="cursor-pointer text-sm font-semibold text-gray-800 dark:text-gray-100">指定用户或客户端 <span class="ml-2 font-normal text-gray-500">{{ individualProfiles.length }} 份专属配置</span></summary>
          <div class="mt-4 space-y-5">
            <p class="max-w-3xl text-sm leading-6 text-gray-500">客户端专属优先，其次是用户、分组、全局默认，按整份配置选用。用户或客户端专属停用后不再向下继承；分组“不注入”始终优先，并关闭自动 Skills。</p>
            <div v-if="individualProfiles.length" class="divide-y divide-gray-100 border-y border-gray-100 dark:divide-dark-700 dark:border-dark-700">
              <div v-for="profile in individualProfiles" :key="profile.key" class="flex min-w-0 flex-wrap items-center justify-between gap-3 py-3">
                <button type="button" class="min-w-0 flex-1 text-left text-sm text-gray-800 hover:text-primary-600 dark:text-gray-200" @click="selectProfile(profile.key)">
                  <span class="block break-words font-medium">{{ profileLabel(profile) }}</span>
                  <span class="mt-1 block text-xs text-gray-500">{{ profile.enabled ? '使用专属内容' : '此对象不注入' }}</span>
                </button>
                <button type="button" class="icon-button" title="编辑专属配置" :aria-label="`编辑${profileLabel(profile)}`" @click="selectProfile(profile.key)"><Icon name="edit" size="sm" /></button>
                <button type="button" class="icon-button text-red-500" title="删除专属配置" :aria-label="`删除${profileLabel(profile)}`" @click="removeProfile(profile)"><Icon name="trash" size="sm" /></button>
              </div>
            </div>
            <div class="grid min-w-0 items-start gap-4 lg:grid-cols-[180px_minmax(0,1fr)_220px_auto]">
              <div>
                <label for="instruction-new-scope" class="input-label">使用对象</label>
                <select id="instruction-new-scope" v-model="newScope" class="input w-full"><option value="user">指定用户</option><option value="api_key">指定客户端</option></select>
              </div>
              <div class="min-w-0">
                <label for="instruction-new-target" class="input-label">{{ newScope === 'user' ? '用户邮箱' : '客户端名称' }}</label>
                <Select v-if="!manualTarget" id="instruction-new-target" :model-value="newScopeId" :options="targetOptions" remote :loading="targetLoading" :placeholder="newScope === 'user' ? '输入邮箱搜索' : '搜索或选择客户端'" :empty-text="newScope === 'user' ? '输入邮箱后显示匹配用户' : '未找到客户端'" :aria-label="newScope === 'user' ? '搜索用户邮箱' : '搜索客户端名称'" @search="searchTargets" @update:model-value="setNewTarget" />
                <input v-else id="instruction-new-target" v-model="newScopeId" class="input w-full" inputmode="numeric" placeholder="填写对象编号" aria-label="手动填写对象编号" />
                <button type="button" class="mt-2 text-xs text-primary-600 hover:underline dark:text-primary-400" @click="manualTarget = !manualTarget; newScopeId = ''">{{ manualTarget ? '返回名称搜索' : '按编号添加' }}</button>
                <p v-if="targetError" class="mt-1 text-xs text-red-600" role="alert">{{ targetError }}</p>
              </div>
              <div>
                <label for="instruction-new-start" class="input-label">初始内容</label>
                <select id="instruction-new-start" v-model="newProfileMode" class="input w-full"><option value="copy">复制全局默认</option><option value="disabled">此对象不注入</option></select>
              </div>
              <button type="button" class="btn btn-secondary lg:mt-6" @click="addProfile"><Icon name="plus" size="sm" class="mr-1.5" />添加专属配置</button>
            </div>
            <p v-if="addError" class="text-sm text-red-600" role="alert">{{ addError }}</p>
          </div>
        </details>

        <details ref="previewDetails" :open="previewOpen" class="border-t border-gray-200 pt-5 dark:border-dark-600" @toggle="previewOpen = ($event.target as HTMLDetailsElement).open">
          <summary class="cursor-pointer text-sm font-semibold text-gray-800 dark:text-gray-100">生效预览 <span class="ml-2 font-normal text-gray-500">本页前置与后置提示词</span></summary>
          <div class="mt-4 space-y-5">
            <div class="grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <div class="min-w-0">
                <label for="instruction-preview-group" class="input-label">分组</label>
                <Select id="instruction-preview-group" :model-value="previewGroupId" :options="previewGroupOptions" searchable aria-label="预览分组" @update:model-value="previewGroupId = stringValue($event)" />
              </div>
              <div class="min-w-0">
                <label for="instruction-preview-model" class="input-label">模型</label>
                <Select id="instruction-preview-model" :model-value="previewModel" :options="previewModelOptions" searchable creatable creatable-prefix="使用模型" aria-label="预览模型" @update:model-value="previewModel = stringValue($event)" />
              </div>
              <div class="min-w-0">
                <label for="instruction-preview-user" class="input-label">用户</label>
                <Select id="instruction-preview-user" :model-value="previewUserId" :options="previewUserOptions" searchable creatable creatable-prefix="使用编号" aria-label="预览用户" @update:model-value="previewUserId = stringValue($event)" />
              </div>
              <div class="min-w-0">
                <label for="instruction-preview-client" class="input-label">客户端</label>
                <Select id="instruction-preview-client" :model-value="previewKeyId" :options="previewKeyOptions" searchable creatable creatable-prefix="使用编号" aria-label="预览客户端" @update:model-value="setPreviewKey" />
              </div>
            </div>
            <p v-if="previewError" class="text-sm text-amber-700" role="alert">{{ previewError }}</p>
            <template v-else-if="preview">
              <p class="flex flex-wrap items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
                <Icon name="eye" size="sm" /><span>采用：<strong>{{ resolvedProfileName }}</strong></span>
                <span class="text-gray-500">· {{ preview.reason === 'group_opted_out' ? '该组不注入，已关闭前置、后置与自动 Skills' : preview.reason === 'master_disabled' ? '总开关已关闭' : preview.reason === 'profile_disabled' ? '当前配置已停用，不补用其他配置' : '当前草稿的生效结果' }}</span>
              </p>
              <ol class="preview-order min-w-0 space-y-4">
                <li class="preview-step">
                  <span class="order-number">1</span><div class="min-w-0 flex-1">
                    <p class="text-sm font-medium text-gray-800 dark:text-gray-100">前置提示词 <span class="ml-2 text-xs font-normal text-gray-500">{{ preview.prefix.length.toLocaleString() }} 字符</span></p>
                    <p v-if="!preview.prefix" class="mt-1 text-sm text-gray-500">不添加前置内容</p>
                    <details v-else class="mt-2"><summary class="cursor-pointer text-xs text-primary-600 dark:text-primary-400">查看正文</summary><pre class="preview-body">{{ preview.prefix }}</pre></details>
                  </div>
                </li>
                <li class="preview-step"><span class="order-number">2</span><div><p class="text-sm font-medium text-gray-800 dark:text-gray-100">客户端原指令</p><p class="mt-1 text-sm text-gray-500">实际请求时填入，此处保留占位。</p></div></li>
                <li class="preview-step">
                  <span class="order-number">3</span><div class="min-w-0 flex-1">
                    <p class="text-sm font-medium text-gray-800 dark:text-gray-100">后置提示词 <span class="ml-2 text-xs font-normal text-gray-500">{{ preview.suffix.length.toLocaleString() }} 字符</span></p>
                    <p class="mt-1 text-xs text-gray-500">来源：{{ suffixSourceLabel }}</p>
                    <p v-if="!preview.suffix" class="mt-1 text-sm text-gray-500">不追加后置内容</p>
                    <details v-else class="mt-2"><summary class="cursor-pointer text-xs text-primary-600 dark:text-primary-400">查看正文</summary><pre class="preview-body">{{ preview.suffix }}</pre></details>
                  </div>
                </li>
              </ol>
              <p class="text-xs leading-5 text-gray-500">{{ preview.reason === 'group_opted_out' ? '本页前后置提示词和自动 Skills 均已关闭。客户端原指令及协议兼容处理沿用原有设置。本预览不调用模型。' : '预览不调用模型。客户端原指令与按任务选择的 Skills 未展开，实际请求会另行加入。' }}</p>
              <p class="text-xs leading-5 text-gray-500">分组以此处选择为准；实际请求使用客户端密钥绑定的分组。</p>
              <p v-if="previewKeyId && !knownKeys[previewKeyId]" class="text-xs text-amber-700">此客户端的所属用户尚未确认；未命中客户端专属配置时，会采用上方选择的用户和分组。</p>
            </template>
          </div>
        </details>
        <details class="border-t border-gray-200 pt-5 dark:border-dark-600" @toggle="jsonOpen = ($event.target as HTMLDetailsElement).open">
          <summary class="cursor-pointer text-xs text-gray-500">高级：原始配置</summary>
          <pre v-if="jsonOpen && validation.config" class="preview-body mt-3">{{ jsonPreview }}</pre>
          <p v-else-if="jsonOpen" class="mt-3 text-sm text-gray-500">完成上面的必填项后，可查看待保存配置。</p>
        </details>
      </fieldset>
      <div v-if="validation.errors.length" class="mt-5 border-l-2 border-amber-400 px-3 py-1 text-sm text-amber-800 dark:text-amber-300" role="alert">
        <p class="font-medium">还有 {{ validation.errors.length }} 项需要补充：</p>
        <ul class="mt-2 list-inside list-disc space-y-1"><li v-for="error in validation.errors" :key="error">{{ error }}</li></ul>
      </div>
      <div class="save-bar sticky bottom-0 z-20 mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-gray-200 py-4 dark:border-dark-600">
        <div class="min-w-0 text-sm" role="status" aria-live="polite">
          <span v-if="saving" class="text-gray-500">正在保存全部配置…</span>
          <span v-else-if="dirty" class="text-amber-700 dark:text-amber-400">有未保存更改</span>
          <span v-else class="inline-flex items-center gap-1.5 text-gray-500"><Icon name="checkCircle" size="sm" class="text-emerald-600" />{{ lastSavedAt ? `已保存 · ${lastSavedAt}` : '配置已同步' }}</span>
          <p v-if="saveError" class="mt-1 text-xs text-red-600">{{ saveError }}</p>
        </div>
        <div class="flex shrink-0 gap-2">
          <button type="button" class="icon-button" :disabled="saving" title="重新读取已保存配置" aria-label="重新读取已保存配置" @click="reloadConfig"><Icon name="refresh" size="sm" /></button>
          <button type="button" class="btn btn-primary" :disabled="saving || !dirty || validation.errors.length > 0" data-testid="instruction-layers-save" @click="saveConfig"><Icon :name="saving ? 'refresh' : 'check'" size="sm" class="mr-1.5" :class="saving && 'animate-spin'" />{{ saving ? '正在保存…' : '保存全部配置' }}</button>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores'
import { allModels } from '@/composables/useModelWhitelist'
import { searchApiKeys, searchUsers, type SimpleApiKey, type SimpleUser } from '@/api/admin/usage'
import { getAllIncludingInactive } from '@/api/admin/groups'
import { getInstructionLayers, updateInstructionLayers, type InstructionLayersConfig } from '@/api/admin/instruction-layers'
import PromptTextEditor from './PromptTextEditor.vue'
import { basicProfileLabel, canonicalScopeId, emptyProfile, profileFromConfig, resolveLayers, validateDrafts, type ModelRuleDraft, type ProfileDraft } from './instructionLayersEditor'

const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const loadFailed = ref(false)
const enabled = ref(false)
const profiles = ref<ProfileDraft[]>([])
const selectedKey = ref('default')
const editorTab = ref<'prefix' | 'suffix'>('prefix')
const savedState = ref('')
const lastSavedAt = ref('')
const saveError = ref('')
const scopeOpen = ref(false)
const previewOpen = ref(false)
const jsonOpen = ref(false)
const previewDetails = ref<HTMLDetailsElement>()
const editorSection = ref<HTMLDivElement>()
const newScope = ref<'user' | 'api_key'>('user')
const newScopeId = ref('')
const manualTarget = ref(false)
const newProfileMode = ref<'copy' | 'disabled'>('copy')
const addError = ref('')
const targetError = ref('')
const targetLoading = ref(false)
const targetOptions = ref<{ value: string; label: string }[]>([])
const knownUsers = ref<Record<string, SimpleUser>>({})
const knownKeys = ref<Record<string, SimpleApiKey>>({})
type NamedGroup = { id: string; name: string; status: string }
type GroupMode = 'inherit' | 'custom' | 'disabled'
const knownGroups = ref<Record<string, NamedGroup>>({})
const groupsLoading = ref(false)
const groupsError = ref('')
const groupError = ref('')
const groupFilter = ref('')
const previewModel = ref('gpt-6-astra')
const previewGroupId = ref('')
const previewUserId = ref('')
const previewKeyId = ref('')
let keySequence = 0
let searchSequence = 0
let alive = true
const nextKey = (prefix: string): string => `${prefix}-${++keySequence}`
const stringValue = (value: unknown): string => typeof value === 'string' ? value : ''
const selectedProfile = computed(() => profiles.value.find((profile) => profile.key === selectedKey.value))
const stateString = computed(() => JSON.stringify({ enabled: enabled.value, profiles: profiles.value }))
const dirty = computed(() => !loading.value && !loadFailed.value && stateString.value !== savedState.value)
const validation = computed(() => validateDrafts(enabled.value, profiles.value))
const jsonPreview = computed(() => jsonOpen.value ? JSON.stringify(validation.value.config, null, 2) : '')
const profileOptions = computed(() => profiles.value.map((profile) => ({ value: profile.key, label: profileLabel(profile) })))
const individualProfiles = computed(() => profiles.value.filter((profile) => profile.scope === 'user' || profile.scope === 'api_key'))
const groupRows = computed(() => {
  const rows = { ...knownGroups.value }
  for (const profile of profiles.value) {
    if (profile.scope === 'group' && !rows[profile.scopeId]) {
      rows[profile.scopeId] = { id: profile.scopeId, name: `分组编号 ${profile.scopeId}`, status: 'unknown' }
    }
  }
  return Object.values(rows)
})
const filteredGroups = computed(() => {
  const keyword = groupFilter.value.trim().toLocaleLowerCase()
  return groupRows.value.filter((group) => !keyword || `${group.name} ${group.id}`.toLocaleLowerCase().includes(keyword))
})
const previewGroupOptions = computed(() => [
  { value: '', label: '未指定分组' },
  ...groupRows.value.map((group) => ({ value: group.id, label: group.name })),
])
const modelOptions = computed(() => {
  const names = new Set<string>(allModels.map((model) => model.value))
  for (const profile of profiles.value) for (const rule of profile.modelSuffixes) if (rule.model && rule.model !== '*') names.add(rule.model)
  return [{ value: '*', label: '其他未单独配置的模型（*）' }, ...[...names].map((model) => ({ value: model, label: model }))]
})
const previewModelOptions = computed(() => modelOptions.value.filter((option) => option.value !== '*'))
const previewUserOptions = computed(() => {
  const ids = new Set([...Object.keys(knownUsers.value), ...profiles.value.filter((profile) => profile.scope === 'user').map((profile) => profile.scopeId)])
  return [{ value: '', label: '未指定用户' }, ...[...ids].map((id) => ({ value: id, label: knownUsers.value[id]?.email || `用户编号 ${id}` }))]
})
const previewKeyOptions = computed(() => {
  const ids = new Set([...Object.keys(knownKeys.value), ...profiles.value.filter((profile) => profile.scope === 'api_key').map((profile) => profile.scopeId)])
  return [{ value: '', label: '未指定客户端' }, ...[...ids].map((id) => ({ value: id, label: knownKeys.value[id]?.name || `客户端编号 ${id}` }))]
})
const previewError = computed(() => {
  if (!validation.value.config) return '请先完成待保存配置中的必填项，再查看预览。'
  if (!previewModel.value.trim() || previewModel.value === '*') return '请选择一个具体模型进行预览。'
  if (previewUserId.value && !canonicalScopeId(previewUserId.value)) return '用户编号格式不正确。'
  if (previewKeyId.value && !canonicalScopeId(previewKeyId.value)) return '客户端编号格式不正确。'
  if (previewGroupId.value && !canonicalScopeId(previewGroupId.value)) return '分组编号格式不正确。'
  return ''
})
const preview = computed(() => validation.value.config && !previewError.value
  ? resolveLayers(validation.value.config, previewModel.value, canonicalScopeId(previewUserId.value) || '', canonicalScopeId(previewKeyId.value) || '', canonicalScopeId(previewGroupId.value) || '')
  : null)
const resolvedProfileName = computed(() => {
  const result = preview.value
  if (!result) return ''
  const profile = profiles.value.find((item) => item.scope === result.scope && item.scopeId === result.scopeId)
  return profile ? profileLabel(profile) : '全局默认'
})
const suffixSourceLabel = computed(() => {
  if (!preview.value?.enabled) return '当前配置已暂停'
  if (preview.value.suffixSource === 'exact') return `${previewModel.value} 的模型专用规则`
  if (preview.value.suffixSource === 'fallback') return '其他未单独配置的模型（*）'
  return '当前配置的后置提示词'
})

function profileLabel(profile: ProfileDraft): string {
  if (profile.scope === 'group' && knownGroups.value[profile.scopeId]) return `分组 · ${knownGroups.value[profile.scopeId].name}`
  if (profile.scope === 'user' && knownUsers.value[profile.scopeId]) return `用户 · ${knownUsers.value[profile.scopeId].email}`
  if (profile.scope === 'api_key' && knownKeys.value[profile.scopeId]) return `客户端 · ${knownKeys.value[profile.scopeId].name}（${profile.scopeId}）`
  return basicProfileLabel(profile)
}
function selectProfile(value: unknown): void {
  if (typeof value === 'string') selectedKey.value = value
}
function groupMode(id: string): GroupMode {
  const profile = profiles.value.find((item) => item.scope === 'group' && item.scopeId === id)
  return !profile ? 'inherit' : profile.enabled ? 'custom' : 'disabled'
}
async function editGroup(id: string): Promise<void> {
  const profile = profiles.value.find((item) => item.scope === 'group' && item.scopeId === id)
  if (!profile) return
  selectedKey.value = profile.key
  await nextTick()
  editorSection.value?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
}
function changeGroupMode(id: string, event: Event): void {
  const input = event.target as HTMLSelectElement
  const mode = input.value as GroupMode
  groupError.value = ''
  const existing = profiles.value.find((item) => item.scope === 'group' && item.scopeId === id)
  if (mode === 'inherit') {
    if (existing && !window.confirm(`恢复“${knownGroups.value[id]?.name || `分组 ${id}`}”的全局继承？这份分组独立配置会被移除，用户和客户端专属配置仍按优先级生效。`)) {
      input.value = groupMode(id)
      return
    }
    if (existing) {
      profiles.value = profiles.value.filter((item) => item.key !== existing.key)
      if (selectedKey.value === existing.key) selectedKey.value = 'default'
    }
    return
  }
  if (!existing && profiles.value.length >= 128) {
    groupError.value = '最多可保存 128 份配置，请先移除不再需要的专属配置。'
    input.value = groupMode(id)
    return
  }
  const profile = existing || emptyProfile('group', id, nextKey('group'))
  if (!existing) {
    const defaults = profiles.value.find((item) => item.scope === 'default')
    if (mode === 'custom' && defaults) {
      profile.prefix = defaults.prefix
      profile.suffix = defaults.suffix
      profile.modelSuffixes = defaults.modelSuffixes.map((rule) => ({ ...rule, key: nextKey('model') }))
    }
    profiles.value.push(profile)
  }
  profile.enabled = mode === 'custom'
  selectedKey.value = profile.key
}
async function changeEditorTab(tab: 'prefix' | 'suffix'): Promise<void> {
  editorTab.value = tab
  await nextTick()
  document.getElementById(`instruction-${tab}-tab`)?.focus()
}
function applyLoaded(config: InstructionLayersConfig): void {
  enabled.value = config.enabled === true
  profiles.value = [
    profileFromConfig('default', '', config.default, nextKey),
    ...Object.entries(config.groups || {}).map(([id, value]) => profileFromConfig('group', id, value, nextKey)),
    ...Object.entries(config.users || {}).map(([id, value]) => profileFromConfig('user', id, value, nextKey)),
    ...Object.entries(config.api_keys || {}).map(([id, value]) => profileFromConfig('api_key', id, value, nextKey)),
  ]
  selectedKey.value = 'default'
  savedState.value = stateString.value
}
function setRuleModel(rule: ModelRuleDraft, value: unknown): void {
  if (typeof value === 'string') rule.model = value
}
function addModelSuffix(): void {
  selectedProfile.value?.modelSuffixes.push({ key: nextKey('model'), model: '', mode: 'custom', text: '' })
}
function removeModelSuffix(rule: ModelRuleDraft): void {
  if (!window.confirm(`删除“${rule.model === '*' ? '其他模型' : rule.model || '未命名模型'}”的规则后，将恢复使用匹配的通用内容。确定删除吗？`)) return
  if (selectedProfile.value) selectedProfile.value.modelSuffixes = selectedProfile.value.modelSuffixes.filter((item) => item.key !== rule.key)
}
function addProfile(): void {
  addError.value = ''
  const id = canonicalScopeId(newScopeId.value)
  if (!id) { addError.value = '请选择使用对象，或填写不含前导零的正整数编号。'; return }
  const duplicate = profiles.value.find((profile) => profile.scope === newScope.value && profile.scopeId === id)
  if (duplicate) { addError.value = '该对象已有专属配置，已为你选中。'; selectedKey.value = duplicate.key; return }
  if (profiles.value.length >= 128) { addError.value = '最多可保存 128 份配置。'; return }
  const draft = emptyProfile(newScope.value, id, nextKey(newScope.value))
  const defaults = profiles.value.find((profile) => profile.scope === 'default')
  if (newProfileMode.value === 'copy' && defaults) {
    draft.enabled = defaults.enabled
    draft.prefix = defaults.prefix
    draft.suffix = defaults.suffix
    draft.modelSuffixes = defaults.modelSuffixes.map((rule) => ({ ...rule, key: nextKey('model') }))
  } else draft.enabled = false
  profiles.value.push(draft)
  selectedKey.value = draft.key
  newScopeId.value = ''
}
function removeProfile(profile: ProfileDraft): void {
  if (profile.scope === 'default' || !window.confirm(`删除“${profileLabel(profile)}”的专属配置？该对象会恢复按用户、分组、全局默认的优先级选择可用配置。`)) return
  profiles.value = profiles.value.filter((item) => item.key !== profile.key)
  if (selectedKey.value === profile.key) selectedKey.value = 'default'
}
function setNewTarget(value: unknown): void {
  newScopeId.value = stringValue(value)
  addError.value = ''
}
function setPreviewKey(value: unknown): void {
  previewKeyId.value = stringValue(value)
  const owner = knownKeys.value[previewKeyId.value]?.user_id
  if (owner) previewUserId.value = String(owner)
}
async function searchTargets(keyword = ''): Promise<void> {
  const sequence = ++searchSequence
  const scope = newScope.value
  targetError.value = ''
  if (scope === 'user' && !keyword.trim()) {
    targetOptions.value = Object.values(knownUsers.value).map((user) => ({ value: String(user.id), label: user.email }))
    targetLoading.value = false
    return
  }
  targetLoading.value = true
  try {
    if (scope === 'user') {
      const users = await searchUsers(keyword.trim())
      if (!alive || sequence !== searchSequence || newScope.value !== scope) return
      const active = users.filter((user) => !user.deleted && Number.isSafeInteger(user.id) && user.id > 0)
      for (const user of active) knownUsers.value[String(user.id)] = { id: user.id, email: user.email, deleted: false }
      targetOptions.value = active.map((user) => ({ value: String(user.id), label: `${user.email}（${user.id}）` }))
    } else {
      const keys = await searchApiKeys(undefined, keyword.trim())
      if (!alive || sequence !== searchSequence || newScope.value !== scope) return
      const active = keys.filter((key) => Number.isSafeInteger(key.id) && key.id > 0)
      for (const key of active) knownKeys.value[String(key.id)] = { id: key.id, name: key.name, user_id: key.user_id }
      targetOptions.value = active.map((key) => ({ value: String(key.id), label: `${key.name || '未命名客户端'}（${key.id}）` }))
    }
  } catch {
    if (alive && sequence === searchSequence) targetError.value = '名称列表暂时无法读取，可以重试或按编号添加。'
  } finally {
    if (alive && sequence === searchSequence) targetLoading.value = false
  }
}
async function loadKeyNames(): Promise<void> {
  try {
    const keys = await searchApiKeys()
    if (!alive) return
    for (const key of keys) if (Number.isSafeInteger(key.id) && key.id > 0) knownKeys.value[String(key.id)] = { id: key.id, name: key.name, user_id: key.user_id }
  } catch { /* Name lookup is optional; saved numeric bindings remain editable. */ }
}
async function loadGroups(): Promise<void> {
  if (groupsLoading.value) return
  groupsLoading.value = true
  groupsError.value = ''
  try {
    const groups = await getAllIncludingInactive()
    if (!alive) return
    knownGroups.value = Object.fromEntries(groups
      .filter((group) => Number.isSafeInteger(group.id) && group.id > 0)
      .map((group) => [String(group.id), { id: String(group.id), name: group.name || '未命名分组', status: group.status }]))
  } catch {
    if (alive) groupsError.value = '分组名称暂时无法读取；已保存的分组配置仍保留。'
  } finally {
    if (alive) groupsLoading.value = false
  }
}
async function openPreview(): Promise<void> {
  previewOpen.value = true
  await nextTick()
  previewDetails.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
async function loadConfig(): Promise<void> {
  loading.value = true
  loadFailed.value = false
  saveError.value = ''
  try {
    const config = await getInstructionLayers()
    if (alive) applyLoaded(config)
  } catch {
    if (alive) { loadFailed.value = true; appStore.showError('提示词配置读取失败，请重试。') }
  } finally {
    if (alive) loading.value = false
  }
}
function reloadConfig(): void {
  if (saving.value || (dirty.value && !window.confirm('重新读取会丢弃当前所有未保存的提示词更改，确定继续吗？'))) return
  void loadConfig()
}
async function saveConfig(): Promise<void> {
  const config = validation.value.config
  if (saving.value || !dirty.value || !config) return
  const previous = selectedProfile.value
  saving.value = true
  saveError.value = ''
  try {
    const saved = await updateInstructionLayers(config)
    if (!alive) return
    applyLoaded(saved)
    const target = profiles.value.find((profile) => profile.scope === previous?.scope && profile.scopeId === previous?.scopeId)
    if (target) selectedKey.value = target.key
    lastSavedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' })
    appStore.showSuccess('全部提示词配置已保存。')
  } catch (error: unknown) {
    if (alive) {
      const message = typeof error === 'object' && error !== null && 'message' in error ? String(error.message) : ''
      saveError.value = `保存失败，草稿已保留。${message ? ` ${message.slice(0, 160)}` : '请稍后重试。'}`
      appStore.showError('提示词保存失败，当前草稿已保留。')
    }
  } finally {
    if (alive) saving.value = false
  }
}
function beforeUnload(event: BeforeUnloadEvent): void {
  if (!dirty.value) return
  event.preventDefault()
  event.returnValue = ''
}
watch(newScope, () => {
  newScopeId.value = ''
  targetOptions.value = []
  addError.value = ''
  targetError.value = ''
  void searchTargets()
})
watch(selectedKey, () => {
  const profile = selectedProfile.value
  if (!profile) return
  previewGroupId.value = profile.scope === 'group' ? profile.scopeId : ''
  previewUserId.value = profile.scope === 'user' ? profile.scopeId : ''
  previewKeyId.value = profile.scope === 'api_key' ? profile.scopeId : ''
  if (previewKeyId.value && knownKeys.value[previewKeyId.value]?.user_id) previewUserId.value = String(knownKeys.value[previewKeyId.value].user_id)
})
onBeforeRouteLeave(() => !dirty.value || window.confirm('提示词尚未保存，离开后会丢失当前草稿。确定离开吗？'))
onMounted(() => {
  window.addEventListener('beforeunload', beforeUnload)
  void loadConfig()
  void loadKeyNames()
  void loadGroups()
})
onBeforeUnmount(() => {
  alive = false
  ++searchSequence
  window.removeEventListener('beforeunload', beforeUnload)
})
</script>

<style scoped>
.instruction-panel { color: rgb(55 65 81); }
.dark .instruction-panel { color: rgb(209 213 219); }
.position-tab { min-height: 2.75rem; padding: .65rem .9rem; border-bottom: 2px solid transparent; color: rgb(107 114 128); font-size: 14px; white-space: nowrap; }
.position-tab-active { border-color: rgb(13 148 136); color: rgb(15 118 110); font-weight: 600; }
.dark .position-tab-active { color: rgb(94 234 212); }
.position-tab:focus-visible { outline: 2px solid rgb(20 184 166); outline-offset: -2px; }
.icon-button { display: inline-flex; flex-shrink: 0; width: 2.5rem; height: 2.5rem; align-items: center; justify-content: center; border: 1px solid rgb(229 231 235); border-radius: 6px; }
.icon-button:hover { background: rgb(148 163 184 / 12%); }
.icon-button:disabled { opacity: .4; }
.icon-button:focus-visible { outline: 2px solid rgb(20 184 166); outline-offset: 2px; }
.dark .icon-button { border-color: rgb(55 65 81); }
.order-number { display: inline-flex; flex-shrink: 0; width: 1.4rem; height: 1.4rem; align-items: center; justify-content: center; border: 1px solid rgb(209 213 219); border-radius: 50%; color: rgb(107 114 128); font-size: 11px; }
.preview-step { display: flex; min-width: 0; align-items: flex-start; gap: .75rem; }
.preview-body { max-height: 20rem; overflow: auto; margin-top: .5rem; padding: .85rem; border-left: 2px solid rgb(203 213 225); background: rgb(148 163 184 / 6%); white-space: pre-wrap; overflow-wrap: anywhere; font-size: 12px; line-height: 1.7; }
.save-bar { background: rgb(255 255 255 / 96%); }
.dark .save-bar { background: rgb(17 24 39 / 96%); }
.instruction-panel :deep(.input) { min-width: 0; }
@media (max-width: 480px) {
  .position-tab { padding-inline: .6rem; }
  .save-bar { gap: .6rem; }
  .save-bar > div:first-child { font-size: 12px; }
}
</style>

<template>
  <div>
    <el-button type="warning" size="small" @click="openDialog">
      仓库名字：{{ currentRepoName || '（未命名）' }}
    </el-button>

    <el-dialog v-model="dialogVisible" title="仓库设置" width="760px">
      <div v-loading="loadingRepoInfo" class="repo-settings-content">
        <div class="section-title">基础信息</div>
        <div class="form-grid">
          <label class="field-label" for="repo-name-input">仓库名字</label>
          <el-input
            id="repo-name-input"
            v-model="inputRepoName"
            placeholder="请输入仓库名字"
            clearable
            :disabled="isBusy || isBasicRepo"
          />

          <label class="field-label">绑定模板</label>
          <div>
            <el-select v-model="typeForm.repoTypeKey" class="w-full" :disabled="isBusy" placeholder="请选择仓库类型模板">
              <el-option v-for="item in selectableRepoTypeOptions" :key="item.key" :label="`${item.name} (${item.key})`" :value="item.key">
                <div class="type-option-row">
                  <span>{{ item.name }}</span>
                  <span class="type-option-key">{{ item.key }}</span>
                </div>
              </el-option>
            </el-select>
            <div v-if="selectedRepoTypeDescription" class="help-text">{{ selectedRepoTypeDescription }}</div>
          </div>
        </div>

        <div class="section-title mt-4">本仓库 Overlay</div>
        <div class="help-text">不勾选“自定义”时，该项会继承所绑定模板的默认值。</div>

        <div class="overlay-box">
          <div class="overlay-row">
            <el-checkbox v-model="typeForm.customizeAddButton" :disabled="isBusy">自定义“允许添加文件”</el-checkbox>
            <el-switch v-model="typeForm.addButton" :disabled="isBusy || !typeForm.customizeAddButton" inline-prompt active-text="开" inactive-text="关" />
          </div>
          <div class="overlay-row">
            <el-checkbox v-model="typeForm.customizeAddDirectoryButton" :disabled="isBusy">自定义“允许添加目录”</el-checkbox>
            <el-switch v-model="typeForm.addDirectoryButton" :disabled="isBusy || !typeForm.customizeAddDirectoryButton" inline-prompt active-text="开" inactive-text="关" />
          </div>
          <div class="overlay-row">
            <el-checkbox v-model="typeForm.customizeDeleteButton" :disabled="isBusy">自定义“允许删除”</el-checkbox>
            <el-switch v-model="typeForm.deleteButton" :disabled="isBusy || !typeForm.customizeDeleteButton" inline-prompt active-text="开" inactive-text="关" />
          </div>
          <div class="overlay-row">
            <el-checkbox v-model="typeForm.customizeAutoNormalize" :disabled="isBusy">自定义“自动归类”</el-checkbox>
            <el-switch v-model="typeForm.autoNormalize" :disabled="isBusy || !typeForm.customizeAutoNormalize" inline-prompt active-text="开" inactive-text="关" />
          </div>
          <div class="overlay-row">
            <el-checkbox v-model="typeForm.customizeShowMD5" :disabled="isBusy">自定义“显示 MD5”</el-checkbox>
            <el-switch v-model="typeForm.showMD5" :disabled="isBusy || !typeForm.customizeShowMD5" inline-prompt active-text="开" inactive-text="关" />
          </div>
          <div class="overlay-row">
            <el-checkbox v-model="typeForm.customizeShowSize" :disabled="isBusy">自定义“显示大小”</el-checkbox>
            <el-switch v-model="typeForm.showSize" :disabled="isBusy || !typeForm.customizeShowSize" inline-prompt active-text="开" inactive-text="关" />
          </div>
          <div class="overlay-row">
            <el-checkbox v-model="typeForm.customizeSingleMove" :disabled="isBusy">自定义“单条移动”</el-checkbox>
            <el-switch v-model="typeForm.singleMove" :disabled="isBusy || !typeForm.customizeSingleMove" inline-prompt active-text="开" inactive-text="关" />
          </div>
          <div class="overlay-row">
            <el-checkbox v-model="typeForm.customizeManualEditorMode" :disabled="isBusy">自定义“手动编辑器”</el-checkbox>
            <el-select v-model="typeForm.manualEditorMode" class="manual-editor-mode-select" :disabled="isBusy || !typeForm.customizeManualEditorMode">
              <el-option label="元数据编辑" value="metadata-editor" />
              <el-option label="旧版类型编辑" value="legacy-type-editor" />
            </el-select>
          </div>
          <div class="overlay-row rulebook-overlay-row">
            <el-checkbox v-model="typeForm.customizeRulebook" :disabled="isBusy">自定义 RuleBook</el-checkbox>
            <RuleBookSelector
              v-model:name="typeForm.rulebookName"
              v-model:version="typeForm.rulebookVersion"
              :disabled="isBusy || !typeForm.customizeRulebook"
            />
          </div>
        </div>

        <div class="preview-box">
          <div class="preview-title">当前生效预览</div>
          <div class="preview-grid">
            <span>模板</span>
            <span>{{ selectedRepoTypeOption?.name || typeForm.repoTypeKey || '-' }}</span>
            <span>允许添加文件</span>
            <span>{{ previewSettings.addButton ? '是' : '否' }}</span>
            <span>允许添加目录</span>
            <span>{{ previewSettings.addDirectoryButton ? '是' : '否' }}</span>
            <span>允许删除</span>
            <span>{{ previewSettings.deleteButton ? '是' : '否' }}</span>
            <span>自动归类</span>
            <span>{{ previewSettings.autoNormalize ? '是' : '否' }}</span>
            <span>显示 MD5</span>
            <span>{{ previewSettings.showMD5 ? '是' : '否' }}</span>
            <span>显示大小</span>
            <span>{{ previewSettings.showSize ? '是' : '否' }}</span>
            <span>单条移动</span>
            <span>{{ previewSettings.singleMove ? '是' : '否' }}</span>
            <span>手动编辑器</span>
            <span>{{ previewSettings.manualEditorMode === 'metadata-editor' ? '元数据编辑' : '旧版类型编辑' }}</span>
            <span>RuleBook</span>
            <span>{{ previewSettings.ruleBookName || '-' }} @ {{ previewSettings.ruleBookVersion || '-' }}</span>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="repo-settings-footer">
          <el-button type="danger" :loading="deleting" :disabled="saving || isBasicRepo" @click="deleteRepo">
            {{ deleteButtonText }}
          </el-button>
          <div class="footer-right-actions">
            <el-button :disabled="isBusy" @click="restoreTemplateInheritance">恢复全部继承</el-button>
            <el-button :disabled="isBusy" @click="closeDialog">关闭</el-button>
            <el-button type="success" :disabled="!canSaveSettings || deleting" :loading="saving" @click="saveRepoSettings">
              保存设置
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import emitter from '../eventBus'
import RuleBookSelector from './RuleBookSelector.vue'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  }
})

const emit = defineEmits(['deleted'])

const dialogVisible = ref(false)
const currentRepoName = ref('加载中...')
const inputRepoName = ref('')
const isBasicRepo = ref(false)
const loadingRepoInfo = ref(false)
const saving = ref(false)
const deleting = ref(false)
const repoTypeOptions = ref([])
const originalName = ref('')
const originalSettingsSignature = ref('')
const currentEffectiveSettings = ref(defaultEffectiveSettings())

const typeForm = reactive(createTypeForm())

const isBusy = computed(() => saving.value || deleting.value || loadingRepoInfo.value)
const deleteButtonText = computed(() => (isBasicRepo.value ? '不能删除基础漫画仓库' : '删除仓库，不会删除任何实际内容'))
const selectedRepoTypeOption = computed(() => {
  return repoTypeOptions.value.find((item) => item.key === typeForm.repoTypeKey) || null
})
const selectableRepoTypeOptions = computed(() => {
  const selectedKey = String(typeForm.repoTypeKey || '').trim()
  return repoTypeOptions.value.filter((item) => item.enabled !== false || item.key === selectedKey)
})
const selectedRepoTypeDescription = computed(() => {
  return String(selectedRepoTypeOption.value?.description || '').trim()
})
const previewSettings = computed(() => {
  const base = selectedRepoTypeOption.value
    ? {
        addButton: !!selectedRepoTypeOption.value.add_button,
        addDirectoryButton: !!selectedRepoTypeOption.value.add_directory_button,
        deleteButton: !!selectedRepoTypeOption.value.delete_button,
        autoNormalize: !!selectedRepoTypeOption.value.auto_normalize,
        showMD5: !!selectedRepoTypeOption.value.show_md5,
        showSize: !!selectedRepoTypeOption.value.show_size,
        singleMove: !!selectedRepoTypeOption.value.single_move,
        manualEditorMode: selectedRepoTypeOption.value.manual_editor_mode || 'legacy-type-editor',
        ruleBookName: selectedRepoTypeOption.value.rulebook_name || 'noop',
        ruleBookVersion: selectedRepoTypeOption.value.rulebook_version || 'v1'
      }
    : { ...currentEffectiveSettings.value }

  if (typeForm.customizeAddButton) base.addButton = !!typeForm.addButton
  if (typeForm.customizeAddDirectoryButton) base.addDirectoryButton = !!typeForm.addDirectoryButton
  if (typeForm.customizeDeleteButton) base.deleteButton = !!typeForm.deleteButton
  if (typeForm.customizeAutoNormalize) base.autoNormalize = !!typeForm.autoNormalize
  if (typeForm.customizeShowMD5) base.showMD5 = !!typeForm.showMD5
  if (typeForm.customizeShowSize) base.showSize = !!typeForm.showSize
  if (typeForm.customizeSingleMove) base.singleMove = !!typeForm.singleMove
  if (typeForm.customizeManualEditorMode) base.manualEditorMode = String(typeForm.manualEditorMode || '').trim() || 'legacy-type-editor'
  if (typeForm.customizeRulebook) {
    base.ruleBookName = String(typeForm.rulebookName || '').trim() || 'noop'
    base.ruleBookVersion = String(typeForm.rulebookVersion || '').trim() || 'v1'
  }

  return base
})
const canSaveSettings = computed(() => {
  if (isBusy.value) return false
  if (String(inputRepoName.value || '').trim() === '') return false
  return buildCurrentSettingsSignature() !== originalSettingsSignature.value || String(inputRepoName.value || '').trim() !== originalName.value
})

function createTypeForm() {
  return {
    repoTypeKey: 'manga',
    customizeAddButton: false,
    addButton: true,
    customizeAddDirectoryButton: false,
    addDirectoryButton: false,
    customizeDeleteButton: false,
    deleteButton: true,
    customizeAutoNormalize: false,
    autoNormalize: false,
    customizeShowMD5: false,
    showMD5: true,
    customizeShowSize: false,
    showSize: true,
    customizeSingleMove: false,
    singleMove: true,
    customizeManualEditorMode: false,
    manualEditorMode: 'legacy-type-editor',
    customizeRulebook: false,
    rulebookName: 'noop',
    rulebookVersion: 'v1'
  }
}

function defaultEffectiveSettings() {
  return {
    addButton: true,
    addDirectoryButton: false,
    deleteButton: true,
    autoNormalize: false,
    showMD5: true,
    showSize: true,
    singleMove: true,
    manualEditorMode: 'legacy-type-editor',
    ruleBookName: 'noop',
    ruleBookVersion: 'v1'
  }
}

function buildSettingsOverridePayload() {
  const payload = {}
  if (typeForm.customizeAddButton) payload.add_button = !!typeForm.addButton
  if (typeForm.customizeAddDirectoryButton) payload.add_directory_button = !!typeForm.addDirectoryButton
  if (typeForm.customizeDeleteButton) payload.delete_button = !!typeForm.deleteButton
  if (typeForm.customizeAutoNormalize) payload.auto_normalize = !!typeForm.autoNormalize
  if (typeForm.customizeShowMD5) payload.show_md5 = !!typeForm.showMD5
  if (typeForm.customizeShowSize) payload.show_size = !!typeForm.showSize
  if (typeForm.customizeSingleMove) payload.single_move = !!typeForm.singleMove
  if (typeForm.customizeManualEditorMode) payload.manual_editor_mode = String(typeForm.manualEditorMode || '').trim() || 'legacy-type-editor'
  if (typeForm.customizeRulebook) {
    payload.rulebook_name = String(typeForm.rulebookName || '').trim() || 'noop'
    payload.rulebook_version = String(typeForm.rulebookVersion || '').trim() || 'v1'
  }
  return payload
}

function buildCurrentSettingsSignature() {
  return JSON.stringify({
    repoTypeKey: String(typeForm.repoTypeKey || '').trim(),
    settingsOverride: buildSettingsOverridePayload()
  })
}

function restoreTemplateInheritance() {
  typeForm.customizeAddButton = false
  typeForm.customizeAddDirectoryButton = false
  typeForm.customizeDeleteButton = false
  typeForm.customizeAutoNormalize = false
  typeForm.customizeShowMD5 = false
  typeForm.customizeShowSize = false
  typeForm.customizeSingleMove = false
  typeForm.customizeManualEditorMode = false
  typeForm.customizeRulebook = false
}

function applyTypeSettingsPayload(payload) {
  const override = payload?.settings_override || {}
  const effective = payload?.effective || {}

  typeForm.repoTypeKey = payload?.repo_type_key || 'manga'

  typeForm.customizeAddButton = typeof override?.add_button === 'boolean'
  typeForm.addButton = typeForm.customizeAddButton ? !!override.add_button : !!effective.add_button

  typeForm.customizeAddDirectoryButton = typeof override?.add_directory_button === 'boolean'
  typeForm.addDirectoryButton = typeForm.customizeAddDirectoryButton ? !!override.add_directory_button : !!effective.add_directory_button

  typeForm.customizeDeleteButton = typeof override?.delete_button === 'boolean'
  typeForm.deleteButton = typeForm.customizeDeleteButton ? !!override.delete_button : !!effective.delete_button

  typeForm.customizeAutoNormalize = typeof override?.auto_normalize === 'boolean'
  typeForm.autoNormalize = typeForm.customizeAutoNormalize ? !!override.auto_normalize : !!effective.auto_normalize

  typeForm.customizeShowMD5 = typeof override?.show_md5 === 'boolean'
  typeForm.showMD5 = typeForm.customizeShowMD5 ? !!override.show_md5 : !!effective.show_md5

  typeForm.customizeShowSize = typeof override?.show_size === 'boolean'
  typeForm.showSize = typeForm.customizeShowSize ? !!override.show_size : !!effective.show_size

  typeForm.customizeSingleMove = typeof override?.single_move === 'boolean'
  typeForm.singleMove = typeForm.customizeSingleMove ? !!override.single_move : !!effective.single_move

  typeForm.customizeManualEditorMode = typeof override?.manual_editor_mode === 'string'
  typeForm.manualEditorMode = typeForm.customizeManualEditorMode ? String(override.manual_editor_mode || 'legacy-type-editor') : String(effective?.manual_editor_mode || 'legacy-type-editor')

  typeForm.customizeRulebook = typeof override?.rulebook_name === 'string' || typeof override?.rulebook_version === 'string'
  typeForm.rulebookName = String(override?.rulebook_name || effective?.rulebook_name || 'noop')
  typeForm.rulebookVersion = String(override?.rulebook_version || effective?.rulebook_version || 'v1')

  currentEffectiveSettings.value = {
    addButton: !!effective.add_button,
    addDirectoryButton: !!effective.add_directory_button,
    deleteButton: !!effective.delete_button,
    autoNormalize: !!effective.auto_normalize,
    showMD5: !!effective.show_md5,
    showSize: !!effective.show_size,
    singleMove: !!effective.single_move,
    manualEditorMode: effective?.manual_editor_mode || 'legacy-type-editor',
    ruleBookName: effective?.rulebook_name || 'noop',
    ruleBookVersion: effective?.rulebook_version || 'v1'
  }

  originalSettingsSignature.value = buildCurrentSettingsSignature()
}

async function parseErrorMessage(res, fallback) {
  try {
    const data = await res.clone().json()
    if (data && data.error) {
      return `${fallback}: ${data.error}`
    }
    if (data && data.message) {
      return `${fallback}: ${data.message}`
    }
  } catch (_) {
    // ignore json parse errors
  }

  try {
    const text = await res.text()
    if (text) {
      return `${fallback}: ${text}`
    }
  } catch (_) {
    // ignore text parse errors
  }

  return `${fallback} (HTTP ${res.status})`
}

async function fetchCurrentRepoName() {
  if (!props.repoId) {
    currentRepoName.value = ''
    inputRepoName.value = ''
    isBasicRepo.value = false
    return
  }

  try {
    const res = await fetch('/api/repos')
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库信息失败'))
    }

    const list = await res.json()
    const repos = Array.isArray(list) ? list : []
    const repo = repos.find((item) => Number(item.id) === Number(props.repoId))
    if (!repo) {
      currentRepoName.value = '（仓库不存在）'
      inputRepoName.value = ''
      isBasicRepo.value = false
      return
    }

    currentRepoName.value = repo.name || ''
    originalName.value = currentRepoName.value
    isBasicRepo.value = !!repo.basic
    if (!dialogVisible.value) {
      inputRepoName.value = currentRepoName.value
    }
  } catch (e) {
    console.error('[RepoSettingsButton] fetchCurrentRepoName failed', e)
    ElMessage.error(e.message || '获取仓库信息失败')
  }
}

async function fetchRepoTypeOptions() {
  const res = await fetch('/api/repo-types?include_disabled=true')
  if (!res.ok) {
    throw new Error(await parseErrorMessage(res, '获取仓库类型失败'))
  }

  const data = await res.json()
  repoTypeOptions.value = Array.isArray(data?.items) ? data.items : []
}

async function refreshRepoInfoState(showErrorToast = false) {
  if (!props.repoId) {
    isBasicRepo.value = false
    return
  }

  loadingRepoInfo.value = true
  try {
    const [repoInfoRes, typeSettingsRes] = await Promise.all([
      fetch(`/api/repos/${props.repoId}/repo-info`),
      fetch(`/api/repos/${props.repoId}/type-settings`),
      fetchRepoTypeOptions()
    ])

    if (!repoInfoRes.ok) {
      throw new Error(await parseErrorMessage(repoInfoRes, '获取 repo info 失败'))
    }
    if (!typeSettingsRes.ok) {
      throw new Error(await parseErrorMessage(typeSettingsRes, '获取仓库类型设置失败'))
    }

    const repoInfo = await repoInfoRes.json()
    const typeSettings = await typeSettingsRes.json()
    isBasicRepo.value = !!repoInfo?.basic
    if (typeof repoInfo?.name === 'string' && repoInfo.name !== '') {
      currentRepoName.value = repoInfo.name
      originalName.value = repoInfo.name
    }
    inputRepoName.value = currentRepoName.value
    applyTypeSettingsPayload(typeSettings)
  } catch (e) {
    console.error('[RepoSettingsButton] refreshRepoInfoState failed', e)
    if (showErrorToast) {
      ElMessage.error(e.message || '获取仓库设置失败')
    }
  } finally {
    loadingRepoInfo.value = false
  }
}

async function openDialog() {
  inputRepoName.value = currentRepoName.value
  dialogVisible.value = true
  await refreshRepoInfoState(true)
}

function closeDialog() {
  dialogVisible.value = false
}

async function saveRepoSettings() {
  if (!canSaveSettings.value) return

  const nextName = String(inputRepoName.value || '').trim()
  saving.value = true
  try {
    if (!isBasicRepo.value && nextName !== currentRepoName.value) {
      const nameRes = await fetch(`/api/repos/${props.repoId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: nextName })
      })
      if (!nameRes.ok) {
        throw new Error(await parseErrorMessage(nameRes, '修改仓库名字失败'))
      }
      const nameData = await nameRes.json()
      currentRepoName.value = nameData?.name || nextName
      originalName.value = currentRepoName.value
      inputRepoName.value = currentRepoName.value
    }

    const typeRes = await fetch(`/api/repos/${props.repoId}/type-settings`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        repo_type_key: String(typeForm.repoTypeKey || '').trim(),
        settings_override: buildSettingsOverridePayload()
      })
    })
    if (!typeRes.ok) {
      throw new Error(await parseErrorMessage(typeRes, '保存仓库类型设置失败'))
    }

    const data = await typeRes.json()
    applyTypeSettingsPayload(data)
    dialogVisible.value = false
    emitter.emit('refresh-all')
    emitter.emit('refresh-repo', { repoId: props.repoId })
    ElMessage.success('仓库设置已保存')
  } catch (e) {
    console.error('[RepoSettingsButton] saveRepoSettings failed', e)
    ElMessage.error(e.message || '保存仓库设置失败')
  } finally {
    saving.value = false
  }
}

async function deleteRepo() {
  if (isBasicRepo.value) {
    ElMessage.warning('基础漫画仓库不允许删除')
    return
  }

  const ok = window.confirm('确认删除这个仓库记录吗？不会删除任何实际元素文件。')
  if (!ok) return

  deleting.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}`, { method: 'DELETE' })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '删除仓库失败'))
    }

    dialogVisible.value = false
    emit('deleted')
    emitter.emit('refresh-all')
    ElMessage.success('仓库已删除')
  } catch (e) {
    console.error('[RepoSettingsButton] deleteRepo failed', e)
    ElMessage.error(e.message || '删除仓库失败')
  } finally {
    deleting.value = false
  }
}

watch(
  () => props.repoId,
  () => {
    dialogVisible.value = false
    fetchCurrentRepoName()
  },
  { immediate: true }
)
</script>

<style scoped>
.repo-settings-content {
  min-height: 260px;
}

.section-title {
  margin-bottom: 10px;
  color: #334155;
  font-size: 14px;
  font-weight: 700;
}

.form-grid {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
}

.field-label {
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.help-text {
  margin-top: 6px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.type-option-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.type-option-key {
  color: #94a3b8;
  font-size: 12px;
}

.overlay-box {
  margin-top: 10px;
  border: 1px solid #dbe3ee;
  border-radius: 12px;
  background: #f8fafc;
  padding: 12px;
}

.overlay-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 0;
}

.rulebook-overlay-row {
  align-items: flex-start;
}

.rulebook-inputs {
  flex: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 90px;
  gap: 8px;
}

.preview-box {
  margin-top: 14px;
  border: 1px solid #dbe3ee;
  border-radius: 12px;
  padding: 12px;
  background: #fff;
}

.preview-title {
  margin-bottom: 8px;
  color: #334155;
  font-size: 13px;
  font-weight: 700;
}

.preview-grid {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 6px 10px;
  color: #475569;
  font-size: 13px;
}

.repo-settings-footer {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.footer-right-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>

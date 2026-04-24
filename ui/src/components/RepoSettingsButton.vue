<template>
  <div>
    <el-button type="warning" size="small" @click="openDialog">
      仓库：{{ currentRepoName || '（未命名）' }}
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

        <div class="collapsible-section-stack">
          <CollapsibleSection v-model:collapsed="bulkActionsCollapsed" title="批量操作">
            <div class="settings-action-list">
              <div class="settings-action-card">
                <div class="settings-action-copy">
                  <div class="settings-action-title">刷新仓库扫描</div>
                  <p class="settings-action-desc">重新扫描当前仓库根路径，补充新增文件并更新失踪标记。</p>
                </div>
                <el-button
                  v-if="!isBasicRepo"
                  type="info"
                  plain
                  :loading="normalizingIncremental"
                  :disabled="isBusy"
                  @click="refreshRepo"
                >
                  刷新
                </el-button>
                <div v-else class="settings-action-disabled-hint">基础漫画仓库不支持自动扫描</div>
              </div>

              <div class="settings-action-card settings-action-card-danger">
                <div class="settings-action-copy">
                  <div class="settings-action-title">删除所有失效项</div>
                  <p class="settings-action-desc">批量删除当前仓库中已标记为“文件失踪”的记录，不会删除任何实际文件。</p>
                </div>
                <el-button
                  type="danger"
                  plain
                  :loading="deletingMissingEntries"
                  :disabled="isBusy"
                  @click="deleteMissingRepoEntries"
                >
                  删除所有失效项
                </el-button>
              </div>

              <div class="settings-action-card settings-action-card-warn">
                <div class="settings-action-copy">
                  <div class="settings-action-title">批量生成提案队列</div>
                  <p class="settings-action-desc">批量检查当前仓库记录的 refresh metadata 提案，只生成候选，不会自动落库。</p>
                </div>
                <el-button
                  type="warning"
                  plain
                  :loading="generatingProposalQueue"
                  :disabled="isBusy"
                  @click="generateRefreshProposalQueue"
                >
                  生成提案队列
                </el-button>
              </div>

              <div class="settings-action-card settings-action-card-warn">
                <div class="settings-action-copy">
                  <div class="settings-action-title">查看提案队列</div>
                  <p class="settings-action-desc">打开当前仓库已缓存的待确认提案队列；如果没有提案，不会显示额外内容。</p>
                </div>
                <el-button
                  type="warning"
                  plain
                  :disabled="isBusy"
                  @click="openRefreshProposalQueue"
                >
                  查看提案队列
                </el-button>
              </div>
            </div>
          </CollapsibleSection>

          <CollapsibleSection v-model:collapsed="overlayCollapsed" title="本仓库 Overlay">
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
              <div class="overlay-row">
                <el-checkbox v-model="typeForm.customizeMetadataDisplayMode" :disabled="isBusy">自定义“metadata 展示”</el-checkbox>
                <el-select v-model="typeForm.metadataDisplayMode" class="manual-editor-mode-select" :disabled="isBusy || !typeForm.customizeMetadataDisplayMode">
                  <el-option label="不显示" value="hidden" />
                  <el-option label="自动显示识别字段" value="auto" />
                  <el-option label="只显示指定字段" value="selected" />
                </el-select>
              </div>
              <div class="overlay-row overlay-row-textarea">
                <el-checkbox v-model="typeForm.customizeMetadataDisplayFields" :disabled="isBusy">自定义“metadata 字段”</el-checkbox>
                <div class="overlay-textarea-wrap">
                  <el-input
                    v-model="typeForm.metadataDisplayFields"
                    type="textarea"
                    :rows="3"
                    :disabled="isBusy || !typeForm.customizeMetadataDisplayFields || previewSettings.metadataDisplayMode !== 'selected'"
                    placeholder="用逗号或换行分隔，例如 title, series_name, author_name"
                  />
                  <div class="help-text">只在“只显示指定字段”时生效；这些字段会控制信息弹窗展示和 metadata 编辑项。</div>
                </div>
              </div>
              <div class="overlay-row">
                <el-checkbox v-model="typeForm.customizeArchiveSubdir" :disabled="isBusy">自定义“archive 子目录”</el-checkbox>
                <el-input
                  v-model="typeForm.archiveSubdir"
                  class="manual-editor-mode-select"
                  :disabled="isBusy || !typeForm.customizeArchiveSubdir"
                  placeholder="例如 archives"
                />
              </div>
              <div class="overlay-row">
                <el-checkbox v-model="typeForm.customizeMaterializedSubdir" :disabled="isBusy">自定义“materialized 子目录”</el-checkbox>
                <div class="overlay-inline-stack">
                  <el-input
                    v-model="typeForm.materializedSubdir"
                    class="manual-editor-mode-select"
                    :disabled="isBusy || !typeForm.customizeMaterializedSubdir"
                    placeholder="/ 或例如 library"
                  />
                  <div class="help-text">填 `/` 表示仓库根目录本身；此模式下普通扫描会自动跳过 archive 子目录。</div>
                </div>
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
          </CollapsibleSection>

          <CollapsibleSection v-model:collapsed="previewCollapsed" title="当前生效预览">
            <div class="preview-box">
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
                <span>metadata 展示</span>
                <span>{{ metadataDisplayModeLabel(previewSettings.metadataDisplayMode) }}</span>
                <span>metadata 字段</span>
                <span>{{ previewSettings.metadataDisplayMode === 'selected' ? (previewSettings.metadataDisplayFields || '（未指定，使用默认字段）') : '（当前模式不限制字段清单）' }}</span>
                <span>archive 子目录</span>
                <span>{{ previewSettings.archiveSubdir || 'archives' }}</span>
                <span>materialized 子目录</span>
                <span>{{ previewSettings.materializedSubdir || '/' }}</span>
                <span>RuleBook</span>
                <span>{{ previewSettings.ruleBookName || '-' }} @ {{ previewSettings.ruleBookVersion || '-' }}</span>
              </div>
              <div v-if="previewSettings.materializedSubdir === '/'" class="help-text preview-help-text">
                当前仓库使用“根目录兼容模式”：普通扫描以仓库根目录为主，但会显式排除 archive 子目录。
              </div>
            </div>
          </CollapsibleSection>
        </div>
      </div>

      <template #footer>
        <div class="repo-settings-footer">
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
import CollapsibleSection from './CollapsibleSection.vue'
import RuleBookSelector from './RuleBookSelector.vue'
import { DEFAULT_VISIBLE_REPO_TYPE_KEY, isRepoTypeHidden } from '../utils/repoTypeVisibility'
import { metadataDisplayModeLabel, stringifyMetadataDisplayFields } from '../utils/repoMetadataDisplay'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  }
})

const dialogVisible = ref(false)
const currentRepoName = ref('加载中...')
const inputRepoName = ref('')
const isBasicRepo = ref(false)
const loadingRepoInfo = ref(false)
const saving = ref(false)
const normalizingIncremental = ref(false)
const deletingMissingEntries = ref(false)
const generatingProposalQueue = ref(false)
const repoTypeOptions = ref([])
const originalName = ref('')
const originalSettingsSignature = ref('')
const currentEffectiveSettings = ref(defaultEffectiveSettings())
const bulkActionsCollapsed = ref(true)
const overlayCollapsed = ref(true)
const previewCollapsed = ref(true)

const typeForm = reactive(createTypeForm())

const isBusy = computed(() => saving.value || loadingRepoInfo.value || normalizingIncremental.value || deletingMissingEntries.value || generatingProposalQueue.value)
const selectedRepoTypeOption = computed(() => {
  return repoTypeOptions.value.find((item) => item.key === typeForm.repoTypeKey) || null
})
const selectableRepoTypeOptions = computed(() => {
  const selectedKey = String(typeForm.repoTypeKey || '').trim()
  return repoTypeOptions.value.filter((item) => {
    const key = String(item?.key || '').trim()
    if (isRepoTypeHidden(key) && key !== selectedKey) {
      return false
    }
    return item.enabled !== false || key === selectedKey
  })
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
        metadataDisplayMode: selectedRepoTypeOption.value.metadata_display_mode || 'hidden',
        metadataDisplayFields: selectedRepoTypeOption.value.metadata_display_fields || '',
        archiveSubdir: selectedRepoTypeOption.value.archive_subdir || 'archives',
        materializedSubdir: selectedRepoTypeOption.value.materialized_subdir || '/',
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
  if (typeForm.customizeMetadataDisplayMode) base.metadataDisplayMode = String(typeForm.metadataDisplayMode || '').trim() || 'hidden'
  if (typeForm.customizeMetadataDisplayFields) base.metadataDisplayFields = stringifyMetadataDisplayFields(typeForm.metadataDisplayFields)
  if (typeForm.customizeArchiveSubdir) base.archiveSubdir = String(typeForm.archiveSubdir || '').trim() || 'archives'
  if (typeForm.customizeMaterializedSubdir) base.materializedSubdir = String(typeForm.materializedSubdir || '').trim() || '/'
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
    repoTypeKey: DEFAULT_VISIBLE_REPO_TYPE_KEY,
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
    customizeMetadataDisplayMode: false,
    metadataDisplayMode: 'hidden',
    customizeMetadataDisplayFields: false,
    metadataDisplayFields: '',
    customizeArchiveSubdir: false,
    archiveSubdir: 'archives',
    customizeMaterializedSubdir: false,
    materializedSubdir: '/',
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
    metadataDisplayMode: 'hidden',
    metadataDisplayFields: '',
    archiveSubdir: 'archives',
    materializedSubdir: '/',
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
  if (typeForm.customizeMetadataDisplayMode) payload.metadata_display_mode = String(typeForm.metadataDisplayMode || '').trim() || 'hidden'
  if (typeForm.customizeMetadataDisplayFields) payload.metadata_display_fields = stringifyMetadataDisplayFields(typeForm.metadataDisplayFields)
  if (typeForm.customizeArchiveSubdir) payload.archive_subdir = String(typeForm.archiveSubdir || '').trim() || 'archives'
  if (typeForm.customizeMaterializedSubdir) payload.materialized_subdir = String(typeForm.materializedSubdir || '').trim() || '/'
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
  typeForm.customizeMetadataDisplayMode = false
  typeForm.customizeMetadataDisplayFields = false
  typeForm.customizeArchiveSubdir = false
  typeForm.customizeMaterializedSubdir = false
  typeForm.customizeRulebook = false
}

function applyTypeSettingsPayload(payload) {
  const override = payload?.settings_override || {}
  const effective = payload?.effective || {}

  typeForm.repoTypeKey = payload?.repo_type_key || DEFAULT_VISIBLE_REPO_TYPE_KEY

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

  typeForm.customizeMetadataDisplayMode = typeof override?.metadata_display_mode === 'string'
  typeForm.metadataDisplayMode = typeForm.customizeMetadataDisplayMode ? String(override.metadata_display_mode || 'hidden') : String(effective?.metadata_display_mode || 'hidden')

  typeForm.customizeMetadataDisplayFields = typeof override?.metadata_display_fields === 'string'
  typeForm.metadataDisplayFields = String(override?.metadata_display_fields || effective?.metadata_display_fields || '')

  typeForm.customizeArchiveSubdir = typeof override?.archive_subdir === 'string'
  typeForm.archiveSubdir = String(override?.archive_subdir || effective?.archive_subdir || 'archives')

  typeForm.customizeMaterializedSubdir = typeof override?.materialized_subdir === 'string'
  typeForm.materializedSubdir = String(override?.materialized_subdir || effective?.materialized_subdir || '/')

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
    metadataDisplayMode: effective?.metadata_display_mode || 'hidden',
    metadataDisplayFields: effective?.metadata_display_fields || '',
    archiveSubdir: effective?.archive_subdir || 'archives',
    materializedSubdir: effective?.materialized_subdir || '/',
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

async function refreshRepo() {
  if (isBasicRepo.value) {
    ElMessage.info('基础漫画仓库没有仓库根路径，所有内容需要手工添加')
    return
  }

  normalizingIncremental.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/normalize/incremental`, { method: 'POST' })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '触发刷新失败'))
    }

    await res.json()
    emitter.emit('refresh-all')
    emitter.emit('refresh-repo', { repoId: props.repoId })
    ElMessage.success('已触发刷新')
  } catch (e) {
    console.error('[RepoSettingsButton] refreshRepo failed', e)
    ElMessage.error(e.message || '触发刷新失败')
  } finally {
    normalizingIncremental.value = false
  }
}

async function deleteMissingRepoEntries() {
  if (!props.repoId) {
    ElMessage.error('缺少仓库信息，无法删除失效项')
    return
  }

  const ok = window.confirm('这会批量删除当前仓库中所有“文件失踪”的记录，且不会删除任何实际文件。是否继续？')
  if (!ok) return

  deletingMissingEntries.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos/missing`, { method: 'DELETE' })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '删除所有失效项失败'))
    }

    const data = await res.json()
    const deletedCount = Number(data?.deleted_count || 0)
    emitter.emit('refresh-repo', { repoId: props.repoId })
    emitter.emit('refresh-all')
    ElMessage.success(deletedCount > 0 ? `已删除 ${deletedCount} 条失效记录` : '当前仓库没有失效记录需要删除')
  } catch (e) {
    console.error('[RepoSettingsButton] deleteMissingRepoEntries failed', e)
    ElMessage.error(e.message || '删除所有失效项失败')
  } finally {
    deletingMissingEntries.value = false
  }
}

async function generateRefreshProposalQueue() {
  if (!props.repoId) {
    ElMessage.error('缺少仓库信息，无法生成提案队列')
    return
  }

  generatingProposalQueue.value = true
  try {
    emitter.emit('repo-refresh-proposals', { repoId: props.repoId })
  } finally {
    generatingProposalQueue.value = false
  }
}

function openRefreshProposalQueue() {
  if (!props.repoId) {
    ElMessage.error('缺少仓库信息，无法查看提案队列')
    return
  }
  emitter.emit('repo-open-refresh-proposals', { repoId: props.repoId })
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
  const res = await fetch('/api/repo-types?include_disabled=true&include_hidden=true')
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
  bulkActionsCollapsed.value = true
  overlayCollapsed.value = true
  previewCollapsed.value = true
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

.collapsible-section-stack {
  margin-top: 16px;
}

.collapsible-section-stack > * + * {
  margin-top: 18px;
  padding-top: 18px;
  border-top: 1px solid #dbe3ee;
}

.settings-action-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.settings-action-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px;
  border: 1px solid #dbe4ee;
  border-radius: 12px;
  background: #ffffff;
}

.settings-action-card-danger {
  border-color: #fecaca;
  background: #fff7f7;
}

.settings-action-copy {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.settings-action-title {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.settings-action-desc {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: #475569;
}

.settings-action-disabled-hint {
  font-size: 13px;
  color: #64748b;
  white-space: nowrap;
}

.overlay-inline-stack {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.preview-help-text {
  margin-top: 12px;
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

.overlay-row-textarea {
  align-items: flex-start;
}

.overlay-textarea-wrap {
  flex: 1;
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
  border: 1px solid #dbe3ee;
  border-radius: 12px;
  padding: 12px;
  background: #fff;
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
  justify-content: flex-end;
  gap: 8px;
}

.footer-right-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 640px) {
  .settings-action-card {
    flex-direction: column;
    align-items: flex-start;
  }

  .settings-action-disabled-hint {
    white-space: normal;
  }
}
</style>

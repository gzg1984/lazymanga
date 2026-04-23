<template>
  <el-dialog
    :model-value="modelValue"
    width="720px"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <template #header>
      <div class="dialog-title-with-tip">
        <span>信息</span>
        <el-tooltip :content="editorModeDescription" placement="top" effect="dark">
          <el-icon class="dialog-title-tip-icon"><WarningFilled /></el-icon>
        </el-tooltip>
      </div>
    </template>

    <div class="manual-edit-content">

      <div class="manual-edit-meta">
        <div><span class="meta-label">仓库 ID:</span> {{ repoId }}</div>
        <div><span class="meta-label">要素 ID:</span> {{ displayRecord?.id ?? '-' }}</div>
        <div class="break-all"><span class="meta-label">路径:</span> {{ displayRecord?.path || '-' }}</div>
        <div v-if="!isDirectoryRecord" class="break-all"><span class="meta-label">MD5:</span> {{ displayRecord?.md5 || '（待计算）' }}</div>
        <div>
          <span class="meta-label">{{ isDirectoryRecord ? '目录大小:' : '文件大小:' }}</span>
          {{ formatSizeHuman(displayRecord) }}
          <span class="meta-sub">（{{ formatSizeBytes(displayRecord) }}<template v-if="isDirectoryRecord">，递归汇总</template>）</span>
        </div>
        <div>
          <span class="meta-label">文件状态:</span>
          <span :class="displayRecord?.is_missing ? 'status-missing' : 'status-ok'">
            {{ displayRecord?.is_missing ? '文件失踪' : '正常' }}
          </span>
        </div>
      </div>

      <div class="manual-info-panel">
        <div class="panel-header">
          <div class="panel-title-row">
            <div class="form-label">{{ editMode ? editActionLabel : '当前信息' }}</div>
            <el-tooltip v-if="panelInfoTooltipLines.length" placement="top-start" effect="dark">
              <template #content>
                <div class="panel-tooltip-content">
                  <div v-for="(line, index) in panelInfoTooltipLines" :key="`panel-tip-${index}`">{{ line }}</div>
                </div>
              </template>
              <el-icon class="panel-title-tip-icon"><WarningFilled /></el-icon>
            </el-tooltip>
            <span
              v-if="editMode"
              class="panel-title-hint"
              :class="autoNormalizeEnabled ? 'is-enabled' : 'is-disabled'"
            >
              {{ autoRelocateHintText }}
            </span>
          </div>
          <el-button
            v-if="!editMode"
            size="small"
            type="primary"
            plain
            :disabled="submitting || !displayRecord?.id"
            @click="enterEditMode"
          >
            {{ editActionLabel }}
          </el-button>
        </div>

        <el-alert
          v-if="refreshProposalSummary"
          class="refresh-proposal-alert"
          type="warning"
          :closable="false"
          show-icon
        >
          <template #title>
            {{ refreshProposalSummary }}
          </template>
          <div v-if="refreshProposalPathHint" class="refresh-proposal-path">识别来源：{{ refreshProposalPathHint }}</div>
          <div class="refresh-proposal-actions">
            <el-button
              size="small"
              type="warning"
              plain
              :disabled="submitting || !displayRecord?.id"
              @click="openRefreshProposalEditor"
            >
              查看并确认提案
            </el-button>
            <span class="refresh-proposal-hint">当前只生成候选变更，关闭或取消不会自动落库。</span>
          </div>
        </el-alert>

        <template v-if="editMode">
          <div class="manual-edit-form inline-edit-form">
            <div class="keyword-helper">
              <div class="keyword-helper-header">
                <div class="form-label">关键词列表</div>
                <div class="keyword-helper-tip">
                  {{ activeInputLabel ? `当前填充目标：${activeInputLabel}` : '先点下方任一输入框，再点这里的关键词即可快速填入' }}
                </div>
              </div>
              <div v-if="keywordSuggestions.length" class="keyword-chip-list">
                <el-tag
                  v-for="keyword in keywordSuggestions"
                  :key="`keyword-${keyword}`"
                  class="keyword-chip"
                  :type="activeInputLabel ? 'primary' : 'info'"
                  effect="plain"
                  round
                  @mousedown.prevent
                  @click="applyKeywordSuggestion(keyword)"
                >
                  {{ keyword }}
                </el-tag>
              </div>
              <div v-else class="metadata-empty-hint">当前原始路径里还没有可提取的关键词。</div>
            </div>

            <div v-if="!usesMetadataEditor" class="form-row">
              <div class="form-label">类型</div>
              <el-radio-group v-model="form.targetType" :disabled="submitting || !isoRecord">
                <el-radio-button label="os">OS</el-radio-button>
                <el-radio-button label="entertainment">娱乐</el-radio-button>
                <el-radio-button label="others">Others</el-radio-button>
              </el-radio-group>
            </div>

            <div v-if="!usesMetadataEditor" class="form-row">
              <div class="form-label">修改名字</div>
              <el-radio-group v-model="form.nameMode" :disabled="submitting || !isoRecord">
                <el-radio label="auto">自动</el-radio>
                <el-radio label="manual">手动</el-radio>
              </el-radio-group>
            </div>

            <div v-if="!usesMetadataEditor && form.nameMode === 'manual'" class="form-row">
              <div class="form-label">新名字</div>
              <el-input
                v-model="form.manualName"
                :disabled="submitting || !isoRecord"
                :placeholder="manualNamePlaceholder"
                @focus="handleInputFocus('manual-name', '', '新名字', $event)"
              />
            </div>

            <div v-if="usesMetadataEditor" class="form-row">
              <div class="form-label">元数据字段</div>

              <div v-if="metadataEditorEntries.length" class="metadata-editor-grid">
                <div v-for="entry in metadataEditorEntries" :key="`${displayRecord?.id || 'draft'}-${entry.key}`" class="metadata-editor-item">
                  <div class="metadata-editor-label">{{ entry.label }}</div>
                  <div v-if="entry.currentValue" class="metadata-editor-current">当前值：{{ entry.currentValue }}</div>
                  <el-input
                    v-model="metadataDraft[entry.key]"
                    :disabled="submitting || !isoRecord"
                    :placeholder="entry.currentValue || entry.placeholder"
                    @focus="handleInputFocus('metadata', entry.key, entry.label, $event)"
                  />
                </div>
              </div>

              <div v-else class="metadata-empty-hint">当前记录还没有识别到可编辑 metadata，可先点“刷新”重新识别。</div>
            </div>
          </div>
        </template>

        <template v-else>
          <div v-if="!metadataDisplayEnabled" class="metadata-empty-hint">当前仓库配置未启用 metadata 展示；如需显示，请在仓库设置或仓库类型模板里开启。</div>
          <div v-else-if="displayMetadataEntries.length" class="metadata-display-grid">
            <div
              v-for="entry in displayMetadataEntries"
              :key="`view-${entry.key}`"
              :class="['metadata-display-item', { 'is-title-item': entry.key === 'title' }]"
            >
              <div v-if="entry.key !== 'title'" class="metadata-display-label">{{ entry.label }}</div>
              <div :class="['metadata-display-value', { 'is-title-value': entry.key === 'title' }]">{{ entry.value }}</div>
            </div>
          </div>
          <div v-else class="metadata-empty-hint">当前记录还没有识别到可展示 metadata，可先点“刷新”重新识别。</div>
        </template>

        <details class="raw-path-collapse">
          <summary class="raw-path-summary">原始数据</summary>
          <div class="raw-path-body">
            <div v-if="rawPathEntries.length" class="raw-path-grid">
              <div v-for="entry in rawPathEntries" :key="`raw-${entry.key}`" class="metadata-display-item raw-path-item">
                <div class="raw-path-item-header">
                  <div class="metadata-display-label">{{ entry.label }}</div>
                  <el-button
                    v-if="entry.key === 'source_path' && canForceRestoreOriginalPath"
                    class="raw-path-action"
                    size="small"
                    type="danger"
                    plain
                    :loading="restoringOriginalPath"
                    :disabled="submitting || refreshing"
                    @click="forceRestoreOriginalPath"
                  >
                    强制恢复到原始路径
                  </el-button>
                </div>
                <div class="metadata-display-value">{{ entry.value }}</div>
              </div>
            </div>
            <div v-else class="metadata-empty-hint">当前记录暂时没有额外的原始路径数据。</div>
          </div>
        </details>
      </div>
    </div>

    <template #footer>
      <el-button :disabled="submitting" @click="emit('update:modelValue', false)">关闭</el-button>
      <el-button v-if="editMode" :disabled="submitting" @click="exitEditMode">返回查看</el-button>
      <el-button :loading="refreshing" :disabled="submitting || !displayRecord?.id" @click="refreshRecordMetadata">刷新</el-button>
      <el-button
        v-if="!editMode"
        type="primary"
        plain
        :disabled="submitting || !displayRecord?.id"
        @click="enterEditMode"
      >
        {{ editActionLabel }}
      </el-button>
      <el-button v-if="editMode" type="primary" :loading="submitting" :disabled="!canSubmit" @click="submitManualEdit">修改</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { WarningFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import emitter from '../eventBus'
import { metadataDisplayModeLabel, resolveMetadataDisplayConfig, shouldExposeMetadataFieldByConfig } from '../utils/repoMetadataDisplay'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  repoId: {
    type: Number,
    required: true
  },
  isoRecord: {
    type: Object,
    default: null
  },
  refreshProposal: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:modelValue', 'update:refreshProposal'])

const submitting = ref(false)
const refreshing = ref(false)
const restoringOriginalPath = ref(false)
const displayRecord = ref(null)
const autoNormalizeEnabled = ref(false)
const repoInfo = ref(null)
const editMode = ref(false)
const metadataDraft = ref({})
const refreshMetadataProposal = ref(null)
const activeInputTarget = ref(null)
const activeInputElement = ref(null)
const preferredMetadataKeys = [
  'title',
  'series_name',
  'scanlator_group',
  'author_name',
  'author_alias',
  'original_work',
  'event_code',
  'comic_market',
  'year',
  'karita_id'
]

const form = reactive({
  targetType: 'os',
  nameMode: 'auto',
  manualName: ''
})

function inferFileExtension(name) {
  const match = String(name || '').trim().match(/(\.[^.\\/]+)$/)
  return match ? match[1].toLowerCase() : ''
}

function inferCurrentName(path, fallbackName) {
  const name = String(fallbackName || '').trim()
  if (name) return name
  const normalized = String(path || '').replace(/\\/g, '/').trim()
  if (!normalized) return ''
  const parts = normalized.split('/')
  return parts[parts.length - 1] || ''
}

function inferTargetTypeFromRecord(record) {
  if (record?.is_entertament && !record?.is_os) {
    return 'entertainment'
  }
  if (record?.is_os && !record?.is_entertament) {
    return 'os'
  }
  return 'others'
}

function normalizeMetadataValue(value) {
  if (Array.isArray(value)) {
    return value.map((item) => String(item || '').trim()).filter(Boolean).join(' / ')
  }
  if (value === null || value === undefined) {
    return ''
  }
  if (typeof value === 'object') {
    return ''
  }
  return String(value).trim()
}

function metadataFieldLabel(key) {
  const mapping = {
    title: '标题',
    series_name: '系列名字',
    scanlator_group: '汉化组',
    author_name: '作者',
    author_alias: '作者别名',
    original_work: '原作',
    comic_market: 'Comic Market',
    event_code: '活动编号',
    circle: '社团',
    circle_name: '社团名',
    year: '年份',
    karita_id: 'Karita ID',
    relative_path: '相对路径',
    source_path: '来源路径',
    path_parts: '路径拆分',
    original_name: '原始名称'
  }
  return mapping[key] || key
}

function shouldExposeMetadataField(key, normalizedValue) {
  return shouldExposeMetadataFieldByConfig(key, normalizedValue, metadataDisplayConfig.value)
}

function extractRowMetadata(item) {
  if (item?.metadata && typeof item.metadata === 'object' && !Array.isArray(item.metadata)) {
    return item.metadata
  }

  const raw = String(item?.metadata_json || item?.metadataJson || '').trim()
  if (!raw || raw === '{}') {
    return {}
  }

  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed
    }
  } catch (_) {
    // ignore invalid metadata json
  }
  return {}
}

function resolveManualEditorMode(info, record) {
  const rawMode = String(info?.manual_editor_mode || info?.manualEditorMode || '').trim().toLowerCase()
  if (rawMode === 'metadata-editor' || rawMode === 'metadata') {
    return 'metadata-editor'
  }
  if (rawMode === 'legacy-type-editor' || rawMode === 'legacy') {
    return 'legacy-type-editor'
  }

  const repoTypeKey = String(info?.repo_type_key || info?.repoTypeKey || '').trim().toLowerCase()
  if (repoTypeKey === 'manga' || repoTypeKey === 'karita-manga') {
    return 'metadata-editor'
  }

  const metadata = extractRowMetadata(record)
  if (Object.keys(metadata).length > 0 && !!(record?.is_directory ?? record?.isDirectory)) {
    return 'metadata-editor'
  }

  return 'legacy-type-editor'
}

function resetMetadataDraft() {
  metadataDraft.value = {}
}

function resetRefreshMetadataProposal() {
  refreshMetadataProposal.value = null
  emit('update:refreshProposal', null)
}

function resolveMetadataSourceRecord() {
  if (displayRecord.value) {
    return displayRecord.value
  }
  return props.isoRecord || null
}

function extractRefreshProposalMetadata() {
  const metadata = refreshMetadataProposal.value?.metadata
  if (metadata && typeof metadata === 'object' && !Array.isArray(metadata)) {
    return metadata
  }
  return {}
}

function resolveMetadataDraftSource() {
  const proposalMetadata = extractRefreshProposalMetadata()
  if (Object.keys(proposalMetadata).length > 0) {
    return proposalMetadata
  }
  return extractRowMetadata(resolveMetadataSourceRecord())
}

function resolveMetadataProposalBase() {
  return {
    ...extractRowMetadata(resolveMetadataSourceRecord()),
    ...extractRefreshProposalMetadata()
  }
}

function normalizeArchiveSubdirForDialog(raw) {
  const normalized = String(raw || '').replace(/\\/g, '/').trim().replace(/^\/+|\/+$/g, '')
  return normalized || 'archives'
}

function stripArchivePrefixForDialogPath(raw) {
  const normalizedPath = String(raw || '').replace(/\\/g, '/').trim().replace(/^\/+|\/+$/g, '')
  if (!normalizedPath) {
    return ''
  }
  if (!isArchiveRecord.value) {
    return normalizedPath
  }
  const prefix = normalizeArchiveSubdirForDialog(repoInfo.value?.archive_subdir)
  if (!prefix) {
    return normalizedPath
  }
  if (normalizedPath === prefix) {
    return ''
  }
  if (normalizedPath.startsWith(prefix + '/')) {
    return normalizedPath.slice(prefix.length + 1)
  }
  return normalizedPath
}

function syncMetadataDraft() {
  const nextDraft = {}
  if (!usesMetadataEditor.value) {
    metadataDraft.value = nextDraft
    return
  }

  const metadata = resolveMetadataDraftSource()
  for (const entry of metadataEditorEntries.value) {
    nextDraft[entry.key] = normalizeMetadataValue(metadata[entry.key] ?? entry.currentValue)
  }
  metadataDraft.value = nextDraft
}

function buildMetadataPayload() {
  const base = resolveMetadataProposalBase()
  const draft = metadataDraft.value || {}
  for (const entry of metadataEditorEntries.value) {
    const value = String(draft[entry.key] || '').trim()
    base[entry.key] = value
  }
  return base
}

function tokenizeKeywordText(value) {
  const extensionLikeTokens = new Set(['zip', 'rar', '7z', 'cbz', 'cbr', 'jpg', 'jpeg', 'png', 'webp', 'gif', 'json', 'db', 'txt'])
  return String(value ?? '')
    .split(/[/\\\s()[\]{}<>【】（）「」『』《》|｜,，、;；:：_+=~!@#$%^&*?'"`.-]+/u)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item) return false
      const lower = item.toLowerCase()
      if (extensionLikeTokens.has(lower)) return false
      if (!/[A-Za-z0-9\u4e00-\u9fff]/.test(item)) return false
      const hasCjk = /[\u4e00-\u9fff]/.test(item)
      if (!hasCjk && item.length < 2 && !/[A-Za-z]+\d+|\d+[A-Za-z]+|\d{4}/.test(item)) {
        return false
      }
      return item.length <= 40
    })
}

function handleInputFocus(scope, key, label, event) {
  activeInputTarget.value = { scope, key, label }
  activeInputElement.value = event?.target || null
}

function assignActiveInputValue(value) {
  const target = activeInputTarget.value
  if (!target) {
    return
  }
  if (target.scope === 'manual-name') {
    form.manualName = value
    return
  }
  if (target.scope === 'metadata' && target.key) {
    metadataDraft.value[target.key] = value
  }
}

function buildKeywordInsertedValue(currentValue, keyword, inputElement) {
  const current = String(currentValue ?? '')
  const nextKeyword = String(keyword || '').trim()
  if (!nextKeyword) {
    return { value: current, cursor: current.length }
  }

  if (!inputElement || typeof inputElement.selectionStart !== 'number' || typeof inputElement.selectionEnd !== 'number') {
    const trimmedCurrent = current.trim()
    const value = trimmedCurrent ? `${trimmedCurrent} ${nextKeyword}` : nextKeyword
    return { value, cursor: value.length }
  }

  const start = inputElement.selectionStart
  const end = inputElement.selectionEnd
  const before = current.slice(0, start)
  const after = current.slice(end)
  const needsLeadingSpace = !!before && !/[\s/|（(【]$/.test(before)
  const needsTrailingSpace = !!after && !/^[\s/|）)】]/.test(after)
  const insertion = `${needsLeadingSpace ? ' ' : ''}${nextKeyword}${needsTrailingSpace ? ' ' : ''}`
  return {
    value: `${before}${insertion}${after}`,
    cursor: before.length + insertion.length
  }
}

async function applyKeywordSuggestion(keyword) {
  if (!activeInputTarget.value) {
    ElMessage.info('请先点击一个输入框，再点关键词')
    return
  }

  const target = activeInputTarget.value
  const currentValue = target.scope === 'manual-name'
    ? form.manualName
    : metadataDraft.value?.[target.key] || ''
  const { value, cursor } = buildKeywordInsertedValue(currentValue, keyword, activeInputElement.value)
  assignActiveInputValue(value)

  await nextTick()
  if (activeInputElement.value?.focus) {
    activeInputElement.value.focus()
    if (typeof activeInputElement.value.setSelectionRange === 'function') {
      activeInputElement.value.setSelectionRange(cursor, cursor)
    }
  }
}

function formatTargetTypeLabel(value) {
  const mapping = {
    os: 'OS',
    entertainment: '娱乐',
    others: 'Others'
  }
  return mapping[value] || String(value || '（未设置）')
}

function formatRefreshDiagnosisFlag(value) {
  return value ? '是' : '否'
}

function formatRefreshDiagnosisKind() {
  if (isDirectoryRecord.value) {
    return '目录'
  }
  if (isArchiveRecord.value) {
    return 'Archive 文件'
  }
  return '普通文件'
}

function formatRefreshDiagnosisFieldList(value) {
  if (!Array.isArray(value) || value.length === 0) {
    return '（空）'
  }
  return value.map((item) => String(item || '').trim()).filter(Boolean).join('、') || '（空）'
}

function buildRefreshFallbackMessage(data) {
  const analysis = data?.metadata_analysis
  const proposal = data?.metadata_proposal
  const hasProposalObject = !!(proposal && typeof proposal === 'object')
  const proposalAvailable = !!proposal?.available
  const proposalMode = String(proposal?.editor_mode || '').trim() || '（空）'
  const analysisAttempted = !!analysis?.attempted
  const analysisStatus = String(analysis?.status || '').trim() || '（空）'
  const analysisReason = String(analysis?.reason || '').trim() || '（空）'
  const analyzedAt = String(analysis?.analyzed_at || '').trim() || '（空）'
  const detectedFields = formatRefreshDiagnosisFieldList(analysis?.detected_fields)
  const blockedFields = formatRefreshDiagnosisFieldList(analysis?.blocked_fields)
  const localMode = usesMetadataEditor.value ? 'metadata-editor' : editorMode.value

  if (isArchiveRecord.value || usesMetadataEditor.value || isMetadataEditor.value) {
    return [
      '刷新已触发，但这次没有补充 md5/大小，也没有进入 metadata 提案确认流。',
      `诊断：要素类型=${formatRefreshDiagnosisKind()}，exists=${formatRefreshDiagnosisFlag(data?.exists !== false)}，metadata_analysis.attempted=${formatRefreshDiagnosisFlag(analysisAttempted)}，analysis.status=${analysisStatus}，analysis.reason=${analysisReason}，analysis.at=${analyzedAt}，analysis.detected_fields=${detectedFields}，analysis.blocked_fields=${blockedFields}，metadata_proposal对象=${formatRefreshDiagnosisFlag(hasProposalObject)}，proposal.available=${formatRefreshDiagnosisFlag(proposalAvailable)}，proposal.editor_mode=${proposalMode}，前端当前编辑模式=${localMode}。`,
      '这通常说明前端触发已到达 /refresh，但后端这次没有返回可用的 metadata 提案。'
    ].join(' ')
  }

  return isDirectoryRecord.value ? '目录存在，已完成路径识别检查与大小刷新' : '文件存在，md5和文件大小均已存在'
}

function formatChangeValue(value) {
  const normalized = normalizeMetadataValue(value)
  return normalized || '（空）'
}

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function collectPendingChanges() {
  const changes = []
  const record = resolveMetadataSourceRecord()
  if (!record) {
    return changes
  }

  if (!usesMetadataEditor.value) {
    const currentType = inferTargetTypeFromRecord(record)
    if (form.targetType !== currentType) {
      changes.push({
        label: '类型',
        before: formatTargetTypeLabel(currentType),
        after: formatTargetTypeLabel(form.targetType)
      })
    }
  }

  if (form.nameMode === 'manual') {
    changes.push({
      label: '名字模式',
      before: '自动',
      after: '手动'
    })
    const currentName = inferCurrentName(record?.path, record?.filename)
    const nextName = form.manualName.trim()
    if (nextName !== currentName) {
      changes.push({
        label: '新名字',
        before: formatChangeValue(currentName),
        after: formatChangeValue(nextName)
      })
    }
  }

  if (usesMetadataEditor.value) {
    const originalMetadata = extractRowMetadata(record)
    const finalMetadata = buildMetadataPayload()
    const metadataKeys = new Set(metadataEditorEntries.value.map((entry) => entry.key))
    for (const key of refreshProposalChangedFields.value) {
      if (key) {
        metadataKeys.add(key)
      }
    }
    for (const key of metadataKeys) {
      const before = normalizeMetadataValue(originalMetadata[key])
      const after = normalizeMetadataValue(finalMetadata[key])
      if (before === after) {
        continue
      }
      changes.push({
        label: metadataFieldLabel(key),
        before: formatChangeValue(before),
        after: formatChangeValue(after)
      })
    }
  }

  return changes
}

async function confirmPendingChanges() {
  const changes = collectPendingChanges()
  const message = changes.length > 0
    ? [
        '<div style="line-height:1.7">',
        '<div style="margin-bottom:8px;font-weight:600;">即将提交以下修改：</div>',
        ...changes.map((change, index) => `<div>${index + 1}. <strong>${escapeHtml(change.label)}</strong>：${escapeHtml(change.before)} → ${escapeHtml(change.after)}</div>`),
        '</div>'
      ].join('')
    : '当前没有检测到明确的字段变化，仍要继续提交吗？'

  try {
    await ElMessageBox.confirm(message, '确认修改', {
      type: 'warning',
      confirmButtonText: '确认修改',
      cancelButtonText: '取消',
      dangerouslyUseHTMLString: changes.length > 0
    })
    return true
  } catch (_) {
    return false
  }
}

const isDirectoryRecord = computed(() => !!(displayRecord.value?.is_directory ?? displayRecord.value?.isDirectory))
const isArchiveRecord = computed(() => {
  const record = resolveMetadataSourceRecord()
  const direct = String(record?.item_kind || record?.itemKind || '').trim().toLowerCase()
  if (direct === 'archive') {
    return true
  }
  const metadata = extractRowMetadata(record)
  return String(metadata?.item_kind || '').trim().toLowerCase() === 'archive'
})
const currentRecordExt = computed(() => inferFileExtension(inferCurrentName(displayRecord.value?.path, displayRecord.value?.filename)))
const editorMode = computed(() => resolveManualEditorMode(repoInfo.value, displayRecord.value))
const isMetadataEditor = computed(() => editorMode.value === 'metadata-editor')
const proposalEditorMode = computed(() => String(refreshMetadataProposal.value?.editor_mode || '').trim().toLowerCase())
const usesMetadataEditor = computed(() => isMetadataEditor.value || proposalEditorMode.value === 'metadata-editor')
const metadataDisplayConfig = computed(() => resolveMetadataDisplayConfig(repoInfo.value || {}))
const metadataDisplayEnabled = computed(() => metadataDisplayConfig.value.mode !== 'hidden')

const metadataConfigSummary = computed(() => {
  const repoTypeLabel = String(repoInfo.value?.template_name || repoInfo.value?.repo_type_key || '当前仓库').trim() || '当前仓库'
  return `这些 metadata 信息是根据仓库配置显示的：${repoTypeLabel} 当前设置为“${metadataDisplayModeLabel(metadataDisplayConfig.value.mode)}”。`
})

const metadataConfigFieldSummary = computed(() => {
  if (metadataDisplayConfig.value.mode === 'hidden') {
    return '当前信息弹窗不会展示 metadata。'
  }
  if (metadataDisplayConfig.value.mode === 'selected') {
    const labels = metadataDisplayConfig.value.fields.map((key) => metadataFieldLabel(key)).filter(Boolean)
    return labels.length ? `当前允许展示/编辑的字段：${labels.join('、')}` : '当前使用默认 metadata 字段列表。'
  }
  return '当前会自动展示已识别到的 metadata 字段。'
})

const archiveMetadataStorageHint = computed(() => {
  if (!isArchiveRecord.value) {
    return ''
  }
  return 'Archive 的 metadata 当前保存在仓库数据库 repo.db 的 repo_iso.metadata_json 中，不写回 archive 目录或压缩包内部。'
})

const panelInfoTooltipLines = computed(() => {
  const lines = []
  if (metadataConfigSummary.value) {
    lines.push(metadataConfigSummary.value)
  }
  if (metadataConfigFieldSummary.value) {
    lines.push(metadataConfigFieldSummary.value)
  }
  if (archiveMetadataStorageHint.value) {
    lines.push(archiveMetadataStorageHint.value)
  }
  return lines
})

const displayMetadataEntries = computed(() => {
  if (!metadataDisplayEnabled.value) {
    return []
  }
  const metadata = extractRowMetadata(resolveMetadataSourceRecord())
  const keys = new Set(metadataDisplayConfig.value.mode === 'selected' ? metadataDisplayConfig.value.fields : preferredMetadataKeys)
  for (const key of Object.keys(metadata)) {
    const normalizedValue = normalizeMetadataValue(metadata[key])
    if (!shouldExposeMetadataField(key, normalizedValue)) {
      continue
    }
    keys.add(key)
  }

  return Array.from(keys)
    .map((key) => {
      const value = normalizeMetadataValue(metadata[key])
      return {
        key,
        label: metadataFieldLabel(key),
        value
      }
    })
    .filter((entry) => !!entry.value)
})

const rawPathEntries = computed(() => {
  const metadata = extractRowMetadata(resolveMetadataSourceRecord())
  const rawKeys = ['source_path', 'relative_path', 'path_parts', 'original_name']

  return rawKeys
    .map((key) => ({
      key,
      label: metadataFieldLabel(key),
      value: normalizeMetadataValue(metadata[key])
    }))
    .filter((entry) => !!entry.value)
})

const metadataEditorEntries = computed(() => {
  if (!usesMetadataEditor.value) {
    return []
  }

  const metadata = extractRowMetadata(resolveMetadataSourceRecord())
  const proposalMetadata = extractRefreshProposalMetadata()
  const keys = new Set(metadataDisplayConfig.value.mode === 'selected' ? metadataDisplayConfig.value.fields : preferredMetadataKeys)
  for (const key of Object.keys(metadata)) {
    const normalizedValue = normalizeMetadataValue(metadata[key])
    if (!shouldExposeMetadataField(key, normalizedValue)) {
      continue
    }
    keys.add(key)
  }
  for (const key of Object.keys(proposalMetadata)) {
    const normalizedValue = normalizeMetadataValue(proposalMetadata[key])
    if (!shouldExposeMetadataField(key, normalizedValue)) {
      continue
    }
    keys.add(key)
  }
  for (const key of refreshProposalChangedFields.value) {
    if (key) {
      keys.add(key)
    }
  }

  return Array.from(keys)
    .filter((key) => !String(key || '').startsWith('_'))
    .map((key) => ({
      key,
      label: metadataFieldLabel(key),
      placeholder: `填写${metadataFieldLabel(key)}`,
      currentValue: normalizeMetadataValue(metadata[key])
    }))
})

const storedSourcePath = computed(() => rawPathEntries.value.find((entry) => entry.key === 'source_path')?.value || '')
const canForceRestoreOriginalPath = computed(() => isDirectoryRecord.value && !!displayRecord.value?.id && !!storedSourcePath.value)
const refreshProposalChangedFields = computed(() => {
  const fields = refreshMetadataProposal.value?.changed_fields
  if (!Array.isArray(fields)) {
    return []
  }
  return fields.map((field) => String(field || '').trim()).filter(Boolean)
})
const refreshProposalSummary = computed(() => {
  if (!refreshMetadataProposal.value?.available) {
    return ''
  }
  const labels = refreshProposalChangedFields.value.map((field) => metadataFieldLabel(field))
  if (labels.length > 0) {
    return `刷新识别到了候选 metadata 变更：${labels.join('、')}`
  }
  return '刷新识别到了候选 metadata 变更，请确认后再提交。'
})
const refreshProposalPathHint = computed(() => String(refreshMetadataProposal.value?.analysis_path || '').trim())
const keywordSuggestions = computed(() => {
  const record = resolveMetadataSourceRecord()
  const metadata = resolveMetadataProposalBase()
  const sourceValues = [
    stripArchivePrefixForDialogPath(record?.path),
    record?.filename,
    stripArchivePrefixForDialogPath(metadata?.source_path),
    metadata?.relative_path,
    normalizeMetadataValue(metadata?.path_parts),
    metadata?.original_name
  ]

  const seen = new Set()
  const results = []
  for (const source of sourceValues) {
    for (const token of tokenizeKeywordText(source)) {
      const normalized = token.toLowerCase()
      if (seen.has(normalized)) {
        continue
      }
      seen.add(normalized)
      results.push(token)
      if (results.length >= 24) {
        return results
      }
    }
  }
  return results
})
const activeInputLabel = computed(() => activeInputTarget.value?.label || '')

const manualNameLooksValid = computed(() => {
  if (form.nameMode !== 'manual') return true
  const value = form.manualName.trim()
  if (value === '') return false

  const currentExt = currentRecordExt.value
  if (!currentExt) return true

  const manualExt = inferFileExtension(value)
  return manualExt === '' || manualExt === currentExt
})

const manualNamePlaceholder = computed(() => {
  if (currentRecordExt.value) {
    return `输入新的文件名（可省略后缀，将自动保留 ${currentRecordExt.value}）`
  }
  return isDirectoryRecord.value ? '输入新的目录名' : '输入新的文件名'
})

function buildMetadataDrivenManualName(titleValue) {
  let nextName = String(titleValue || '').trim()
  if (!nextName) {
    return ''
  }

  const currentExt = currentRecordExt.value
  if (!currentExt) {
    return nextName
  }

  const providedExt = inferFileExtension(nextName)
  if (providedExt && providedExt !== currentExt) {
    nextName = nextName.slice(0, -providedExt.length).trim()
  }
  if (inferFileExtension(nextName) === currentExt) {
    return nextName
  }
  return `${nextName}${currentExt}`
}

function resolveMetadataEditorRenamePayload() {
  if (!usesMetadataEditor.value) {
    return {
      nameMode: form.nameMode,
      manualName: form.manualName.trim()
    }
  }

  if (isDirectoryRecord.value) {
    return {
      nameMode: 'auto',
      manualName: ''
    }
  }

  const metadata = extractRowMetadata(resolveMetadataSourceRecord())
  const currentTitle = normalizeMetadataValue(metadata?.title)
  const nextTitle = normalizeMetadataValue(metadataDraft.value?.title)
  if (!nextTitle || nextTitle === currentTitle) {
    return {
      nameMode: 'auto',
      manualName: ''
    }
  }

  return {
    nameMode: 'manual',
    manualName: buildMetadataDrivenManualName(nextTitle)
  }
}

const canSubmit = computed(() => {
  if (!displayRecord.value?.id) return false
  if (!manualNameLooksValid.value) return false
  return true
})

const editActionLabel = computed(() => (usesMetadataEditor.value ? '修改元数据' : '修改信息'))
const autoRelocateHintText = computed(() => (
  autoNormalizeEnabled.value ? '修改后会自动更新目录路径' : '修改后不会自动更新目录路径'
))

const editorModeDescription = computed(() => {
  const configHint = metadataDisplayEnabled.value
    ? '显示字段范围由仓库类型和当前仓库 overlay 一起决定。'
    : '当前仓库配置未启用 metadata 展示，所以这里只显示基础信息与原始数据。'
  if (usesMetadataEditor.value && !editMode.value) {
    return `当前先展示识别结果；点击“修改元数据”后可直接修改标题、汉化组、作者、原作等字段，其中标题会作为名字来源。${configHint}`
  }
  if (usesMetadataEditor.value) {
    return `当前已进入“元数据编辑”模式；直接改标题就会按新标题更新目录/文件名，不改标题则保持自动结果。上方关键词也能快速填入当前聚焦的输入框。${configHint}`
  }
  return editMode.value ? `设置类型和名字策略后，点击修改会先给出变更确认，再提交到后端执行更新。${configHint}` : `当前先展示这条记录的信息；如需调整，再进入修改模式。${configHint}`
})

function parseSizeBytes(v) {
  if (v && typeof v === 'object') {
    const candidates = [v.size_bytes, v.sizeBytes, v.size]
    for (const candidate of candidates) {
      const parsed = Number(candidate)
      if (Number.isFinite(parsed)) {
        return parsed
      }
    }
    return null
  }

  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

function formatSizeHuman(v) {
  const size = parseSizeBytes(v)
  if (size === null || size === -1) return '待计算'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  if (unitIndex === 0) {
    return `${Math.round(value)} ${units[unitIndex]}`
  }
  return `${value.toFixed(2)} ${units[unitIndex]}`
}

function formatSizeBytes(v) {
  const size = parseSizeBytes(v)
  if (size === null || size === -1) return '待计算'
  return `${Math.round(size)} B`
}

function resetFormFromRecord() {
  displayRecord.value = props.isoRecord ? { ...props.isoRecord } : null
  resetRefreshMetadataProposal()
  editMode.value = false
  activeInputTarget.value = null
  activeInputElement.value = null
  form.targetType = inferTargetTypeFromRecord(displayRecord.value)
  form.nameMode = 'auto'
  form.manualName = inferCurrentName(displayRecord.value?.path, displayRecord.value?.filename)
  syncMetadataDraft()
}

function enterEditMode() {
  editMode.value = true
  activeInputTarget.value = null
  activeInputElement.value = null
  syncMetadataDraft()
}

function exitEditMode() {
  editMode.value = false
  activeInputTarget.value = null
  activeInputElement.value = null
}

function openRefreshProposalEditor() {
  if (!refreshMetadataProposal.value?.available) {
    return
  }
  enterEditMode()
}

function updateRefreshMetadataProposal(proposal) {
  refreshMetadataProposal.value = proposal && typeof proposal === 'object' ? proposal : null
  emit('update:refreshProposal', refreshMetadataProposal.value)
}

async function parseErrorMessage(res, fallback) {
  try {
    const data = await res.clone().json()
    if (data && data.error) {
      return `${fallback}: ${data.error}`
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

async function fetchRepoInfoState() {
  if (!props.repoId) {
    autoNormalizeEnabled.value = false
    repoInfo.value = null
    return
  }

  try {
    const res = await fetch(`/api/repos/${props.repoId}/type-settings`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库类型设置失败'))
    }

    const data = await res.json()
    const effective = data?.effective || {}
    repoInfo.value = {
      ...effective,
      repo_type_key: data?.repo_type_key || '',
      resolution_note: data?.resolution_note || '',
      template_name: data?.template?.name || ''
    }
    autoNormalizeEnabled.value = !!effective?.auto_normalize
    syncMetadataDraft()
  } catch (e) {
    autoNormalizeEnabled.value = false
    repoInfo.value = null
    ElMessage.error(e.message || '获取仓库类型设置失败')
  }
}

async function forceRestoreOriginalPath() {
  if (!canForceRestoreOriginalPath.value) {
    ElMessage.error('当前记录没有可恢复的原始路径')
    return
  }

  try {
    await ElMessageBox.confirm(
      `将把当前目录强制恢复到存储的原始路径：\n${storedSourcePath.value}\n\n缺少的上级目录会自动创建。`,
      '确认恢复',
      {
        type: 'warning',
        confirmButtonText: '强制恢复',
        cancelButtonText: '取消'
      }
    )
  } catch (_) {
    return
  }

  restoringOriginalPath.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos/${displayRecord.value.id}/manual-edit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ force_restore_source_path: true })
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '恢复原始路径失败'))
    }

    const data = await res.json()
    if (data?.record) {
      displayRecord.value = { ...data.record }
      syncMetadataDraft()
    }
    editMode.value = false
    emitter.emit('refresh-repo', { repoId: props.repoId })
    ElMessage.success(data?.moved === false ? '当前已在原始路径，无需恢复' : '已强制恢复到原始路径')
  } catch (e) {
    ElMessage.error(e.message || '恢复原始路径失败')
  } finally {
    restoringOriginalPath.value = false
  }
}

async function submitManualEdit() {
  if (!displayRecord.value?.id) {
    ElMessage.error('缺少记录信息，无法修改')
    return
  }
  if (!manualNameLooksValid.value) {
    if (!form.manualName.trim()) {
      ElMessage.error('手动模式下，请填写新名字')
    } else if (currentRecordExt.value) {
      ElMessage.error(`手动模式下，新名字需保留 ${currentRecordExt.value} 后缀`)
    } else {
      ElMessage.error('手动模式下，新名字格式无效')
    }
    return
  }

  const confirmed = await confirmPendingChanges()
  if (!confirmed) {
    return
  }

  submitting.value = true
  try {
    const renamePayload = resolveMetadataEditorRenamePayload()
    const payload = {
      target_type: form.targetType,
      name_mode: renamePayload.nameMode,
      manual_name: renamePayload.manualName
    }
    if (usesMetadataEditor.value) {
      payload.metadata = buildMetadataPayload()
    }

    const res = await fetch(`/api/repos/${props.repoId}/repoisos/${displayRecord.value.id}/manual-edit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '手动修改失败'))
    }

    await res.json()
    updateRefreshMetadataProposal(null)
    emitter.emit('refresh-repo', { repoId: props.repoId })
    ElMessage.success(usesMetadataEditor.value ? '元数据修改已提交' : '手动修改已提交')
    emit('update:modelValue', false)
  } catch (e) {
    ElMessage.error(e.message || '手动修改失败')
  } finally {
    submitting.value = false
  }
}

async function refreshRecordMetadata() {
  if (!displayRecord.value?.id) {
    ElMessage.error('缺少记录信息，无法刷新')
    return
  }

  refreshing.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos/${displayRecord.value.id}/refresh`, {
      method: 'POST'
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '刷新失败'))
    }

    const data = await res.json()
    if (data?.record) {
      displayRecord.value = { ...data.record }
    }
    updateRefreshMetadataProposal(data?.metadata_proposal)
    syncMetadataDraft()

    emitter.emit('refresh-repo', { repoId: props.repoId })

    if (!data?.exists) {
      ElMessage.warning('当前记录对应文件不存在，已标记为文件失踪，可直接删除记录')
      return
    }

    const parts = []
    if (data?.path_moved) parts.push(isDirectoryRecord.value ? '路径识别/重命名' : '路径重定位')
    if (data?.md5_updated) parts.push('md5')
    if (data?.size_updated) parts.push(isDirectoryRecord.value ? '目录大小' : '文件大小')

    if (refreshMetadataProposal.value?.available && usesMetadataEditor.value) {
      editMode.value = true
      syncMetadataDraft()
      const labels = refreshProposalChangedFields.value.map((field) => metadataFieldLabel(field))
      ElMessage.warning(labels.length
        ? `刷新识别到了候选 metadata：${labels.join('、')}。请确认后再提交。`
        : '刷新识别到了候选 metadata。请确认后再提交。')
      return
    }

    if (parts.length > 0) {
      ElMessage.success(`刷新完成，已补充：${parts.join('、')}`)
      return
    }

    ElMessage.warning(buildRefreshFallbackMessage(data))
  } catch (e) {
    ElMessage.error(e.message || '刷新失败')
  } finally {
    refreshing.value = false
  }
}

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      resetFormFromRecord()
      refreshMetadataProposal.value = props.refreshProposal && typeof props.refreshProposal === 'object' ? props.refreshProposal : null
      fetchRepoInfoState()
      return
    }
    repoInfo.value = null
    refreshMetadataProposal.value = null
    editMode.value = false
    activeInputTarget.value = null
    activeInputElement.value = null
    resetMetadataDraft()
  }
)

watch(
  () => props.repoId,
  () => {
    autoNormalizeEnabled.value = false
    repoInfo.value = null
    refreshMetadataProposal.value = null
    if (props.modelValue) {
      fetchRepoInfoState()
    }
  }
)

watch(
  () => props.isoRecord,
  () => {
    if (props.modelValue) {
      resetFormFromRecord()
    }
  },
  { deep: true }
)

watch(editorMode, () => {
  if (props.modelValue) {
    syncMetadataDraft()
  }
})

watch(
  () => props.refreshProposal,
  (proposal) => {
    refreshMetadataProposal.value = proposal && typeof proposal === 'object' ? proposal : null
    if (props.modelValue) {
      syncMetadataDraft()
    }
  },
  { deep: true }
)
</script>

<style scoped>
.dialog-title-with-tip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #0f172a;
  font-weight: 600;
}

.dialog-title-tip-icon {
  color: #d97706;
  font-size: 14px;
  cursor: help;
}

.manual-edit-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.manual-edit-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #f8fafc;
  font-size: 14px;
  color: #334155;
}

.meta-label {
  font-weight: 600;
}

.meta-sub {
  color: #64748b;
  margin-left: 4px;
}

.manual-edit-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 12px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
}

.inline-edit-form {
  padding: 0;
  border: 0;
  background: transparent;
}

.form-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-label {
  font-size: 14px;
  font-weight: 600;
  color: #334155;
}

.manual-info-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
}

.refresh-proposal-alert {
  margin-bottom: 4px;
}

.refresh-proposal-path {
  margin-top: 4px;
  font-size: 12px;
  color: #92400e;
  word-break: break-all;
}

.refresh-proposal-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 8px;
  flex-wrap: wrap;
}

.refresh-proposal-hint {
  font-size: 12px;
  color: #92400e;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.metadata-config-note {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  border: 1px solid #dbe3ee;
  border-radius: 8px;
  background: #f8fafc;
  color: #475569;
  font-size: 12px;
  line-height: 1.6;
}

.panel-title-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
}

.panel-title-tip-icon {
  color: #d97706;
  font-size: 14px;
  cursor: help;
}

.panel-tooltip-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-width: 360px;
  white-space: normal;
  line-height: 1.5;
}

.panel-title-hint {
  font-size: 12px;
  font-weight: 500;
  line-height: 1.4;
}

.panel-title-hint.is-enabled {
  color: #0f766e;
}

.panel-title-hint.is-disabled {
  color: #64748b;
}

.metadata-display-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
}

.metadata-display-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px;
  border-radius: 6px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
}

.metadata-display-item.is-title-item {
  grid-column: 1 / -1;
  background: #ffffff;
}

.raw-path-item-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}

.raw-path-action {
  margin-left: auto;
}

.metadata-display-label {
  font-size: 12px;
  color: #64748b;
  font-weight: 600;
}

.metadata-display-value {
  font-size: 14px;
  color: #0f172a;
  word-break: break-word;
}

.metadata-display-value.is-title-value {
  font-size: 16px;
  font-weight: 700;
}

.keyword-helper {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  border-radius: 8px;
  border: 1px dashed #cbd5e1;
  background: #f8fafc;
}

.keyword-helper-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.keyword-helper-tip {
  font-size: 12px;
  color: #64748b;
}

.keyword-chip-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.keyword-chip {
  cursor: pointer;
  user-select: none;
}

.metadata-editor-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
}

.metadata-editor-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.metadata-editor-label {
  font-size: 13px;
  color: #334155;
  font-weight: 600;
}

.metadata-editor-current {
  font-size: 12px;
  color: #64748b;
}

.raw-path-collapse {
  margin-top: 6px;
  border-top: 1px dashed #cbd5e1;
  padding-top: 10px;
}

.raw-path-summary {
  cursor: pointer;
  color: #475569;
  font-size: 13px;
  font-weight: 600;
  user-select: none;
}

.raw-path-body {
  margin-top: 10px;
}

.raw-path-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
}

.raw-path-item {
  background: #fffdf5;
}

.metadata-empty-hint {
  padding: 10px 12px;
  border-radius: 6px;
  background: #f8fafc;
  color: #64748b;
  font-size: 13px;
}

.status-ok {
  color: #166534;
  font-weight: 600;
}

.status-missing {
  color: #b91c1c;
  font-weight: 700;
}
</style>

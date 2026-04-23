<template>
  <div class="iso-table-wrap p-6 border-4 border-slate-400 rounded-lg bg-slate-100">
    <div v-if="refreshProposalQueueCount > 0" class="repoiso-toolbar">
      <div class="repoiso-toolbar-summary">
        <span class="repoiso-toolbar-title">刷新提案</span>
        <span class="repoiso-toolbar-hint">
          {{ `当前有 ${refreshProposalQueueCount} 条待确认的 metadata 提案` }}
        </span>
      </div>
      <div class="repoiso-toolbar-actions">
        <el-button
          size="small"
          type="warning"
          plain
          @click="refreshProposalQueueVisible = true"
        >
          查看提案队列<span v-if="refreshProposalQueueCount > 0">（{{ refreshProposalQueueCount }}）</span>
        </el-button>
      </div>
    </div>

    <el-table
      v-loading="loading"
      :data="filteredRepoIsoList"
      :row-class-name="resolveRowClassName"
      style="width: 100%"
      border
    >
      <el-table-column width="170" align="center">
        <template #header>
          <div class="type-column-header">
            <span class="type-column-title">类型</span>
            <el-dropdown
              v-if="showLegacyTypeFilters"
              trigger="click"
              size="small"
              @command="handleTypeFilterCommand"
            >
              <el-button class="type-filter-trigger" size="small" :type="activeTypeFilter === 'all' ? 'primary' : 'info'" plain>
                {{ typeFilterButtonLabel }}
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="all">全部</el-dropdown-item>
                  <el-dropdown-item v-if="showDirectoryFilter" command="directory">目录</el-dropdown-item>
                  <el-dropdown-item v-if="showOSFilter" command="os">OS</el-dropdown-item>
                  <el-dropdown-item v-if="showEntertainmentFilter" command="entertainment">娱乐</el-dropdown-item>
                  <el-dropdown-item v-if="showArchiveFilter" command="archive">Archive</el-dropdown-item>
                  <el-dropdown-item v-if="showOthersFilter" command="others">Others</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </template>
        <template #default="scope">
          <el-tag size="small" :type="elementTagType(scope.row)">{{ formatElementType(scope.row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="path" min-width="360">
        <template #header>
          <div class="type-filter-actions">
            <div v-if="showMetadataFilter" class="metadata-filter-actions">
              <el-select
                :model-value="activeMetadataKeyFilter"
                size="small"
                clearable
                filterable
                class="metadata-filter-select"
                placeholder="元数据字段"
                @change="handleMetadataKeyChange"
              >
                <el-option
                  v-for="option in metadataKeyOptions"
                  :key="option.value"
                  :label="`${option.label} (${option.count})`"
                  :value="option.value"
                />
              </el-select>
              <el-select
                :model-value="activeMetadataValueFilter"
                size="small"
                clearable
                filterable
                class="metadata-filter-select metadata-filter-value-select"
                :disabled="!activeMetadataKeyFilter"
                placeholder="字段值"
                @change="handleMetadataValueChange"
              >
                <el-option
                  v-for="option in metadataValueOptions"
                  :key="option.value"
                  :label="`${option.label} (${option.count})`"
                  :value="option.value"
                />
              </el-select>
            </div>
          </div>
        </template>
        <template #default="scope">
          <div v-if="isDirectoryRow(scope.row)" :class="['others-path-cell', { 'path-missing': isRowMissing(scope.row) }]">
            <span class="others-badge">目录</span>
            <div class="path-preview-stack">
	          <span class="others-primary-name">{{ resolvePrimaryDisplayLabel(scope.row) }}</span>
              <div v-if="metadataPreviewEntries(scope.row).length" class="metadata-preview-row">
                <el-tag
                  v-for="entry in metadataPreviewEntries(scope.row)"
                  :key="`${scope.row.id}-${entry.key}`"
                  class="metadata-preview-tag"
                  :title="`${entry.label}：${entry.value}`"
                  size="small"
                  type="info"
                  effect="plain"
                >
                  {{ entry.label }}：{{ entry.value }}
                </el-tag>
                <el-tag
                  v-if="hasRefreshProposal(scope.row)"
                  class="metadata-preview-tag metadata-proposal-tag"
                  size="small"
                  type="danger"
                  effect="plain"
                >
                  待确认提案
                </el-tag>
              </div>
              <div v-else-if="hasRefreshProposal(scope.row)" class="metadata-preview-row">
                <el-tag class="metadata-preview-tag metadata-proposal-tag" size="small" type="danger" effect="plain">待确认提案</el-tag>
              </div>
            </div>
          </div>
          <div v-else-if="isArchiveItem(scope.row)" :class="['others-path-cell', { 'path-missing': isRowMissing(scope.row) }]">
            <span class="archive-badge">Archive</span>
            <div class="path-preview-stack">
	          <span class="others-primary-name">{{ resolvePrimaryDisplayLabel(scope.row) }}</span>
              <div v-if="metadataPreviewEntries(scope.row).length" class="metadata-preview-row">
                <el-tag
                  v-for="entry in metadataPreviewEntries(scope.row)"
                  :key="`${scope.row.id}-${entry.key}`"
                  class="metadata-preview-tag"
                  :title="`${entry.label}：${entry.value}`"
                  size="small"
                  type="warning"
                  effect="plain"
                >
                  {{ entry.label }}：{{ entry.value }}
                </el-tag>
                <el-tag
                  v-if="hasRefreshProposal(scope.row)"
                  class="metadata-preview-tag metadata-proposal-tag"
                  size="small"
                  type="danger"
                  effect="plain"
                >
                  待确认提案
                </el-tag>
              </div>
              <div v-else-if="hasRefreshProposal(scope.row)" class="metadata-preview-row">
                <el-tag class="metadata-preview-tag metadata-proposal-tag" size="small" type="danger" effect="plain">待确认提案</el-tag>
              </div>
            </div>
          </div>
          <div v-else-if="isOSItem(scope.row)" :class="['os-path-cell', { 'path-missing': isRowMissing(scope.row) }]">
            <span class="os-badge">OS</span>
            <span class="os-file-name">{{ extractFileName(scope.row.path) }}</span>
          </div>
          <div v-else-if="isEntertainmentItem(scope.row)" :class="['entertainment-path-cell', { 'path-missing': isRowMissing(scope.row) }]">
            <span class="entertainment-badge">娱乐</span>
            <span class="entertainment-file-name">{{ extractFileName(scope.row.path) }}</span>
          </div>
          <div v-else-if="isOtherItem(scope.row)" :class="['others-path-cell', { 'path-missing': isRowMissing(scope.row) }]">
            <span class="others-badge">Others</span>
            <div class="path-preview-stack">
	          <span class="others-primary-name">{{ resolvePrimaryDisplayLabel(scope.row) }}</span>
            </div>
          </div>
          <span v-else :class="{ 'path-missing': isRowMissing(scope.row) }">{{ formatDisplayPath(scope.row) }}</span>
        </template>
      </el-table-column>
      <el-table-column v-if="showMD5Column" label="MD5" min-width="280">
        <template #default="scope">
          <span class="meta-text">{{ isDirectoryRow(scope.row) ? '目录' : (String(scope.row?.md5 || '').trim() || '待计算') }}</span>
        </template>
      </el-table-column>
      <el-table-column v-if="showSizeColumn" label="大小" width="112" align="right" class-name="size-column" header-cell-class-name="size-column-header">
        <template #default="scope">
          <span class="meta-text">{{ formatSize(scope.row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="276" align="center" class-name="action-column" header-cell-class-name="action-column-header">
        <template #default="scope">
          <div class="row-actions">
            <el-button
              v-if="!isRowMissing(scope.row) && !isDirectoryRow(scope.row)"
              class="download-action-button"
              size="small"
              type="primary"
              plain
              :loading="isRowDownloading(scope.row)"
              @click="handleDownload(scope.row)"
            >
              下载
            </el-button>
            <el-button
              circle
              size="small"
              type="primary"
              :icon="Setting"
              :disabled="isRowDownloading(scope.row)"
              @click="openManualEdit(scope.row)"
            />
            <el-button
              v-if="canShowDeleteButton(scope.row)"
              circle
              size="small"
              type="danger"
              plain
              :icon="Delete"
              :disabled="isRowDownloading(scope.row)"
              @click="openDeleteDialog(scope.row)"
            />
            <el-button
              v-if="singleMoveEnabled"
              circle
              size="small"
              type="warning"
              :icon="Right"
              :disabled="isRowDownloading(scope.row)"
              @click="openSingleMoveDialog(scope.row)"
            />
            <span v-if="hasRefreshProposal(scope.row)" class="row-proposal-hint">待确认提案</span>
            <span v-if="isRowMissing(scope.row)" class="row-missing-hint">文件失踪</span>
            <span v-if="getRowDownloadHint(scope.row)" class="row-download-hint">{{ getRowDownloadHint(scope.row) }}</span>
          </div>
        </template>
      </el-table-column>
      <template #header>
        <div class="flex justify-between items-center w-full">
          <span>管理要素信息表</span>
          <span class="text-xs text-slate-600">共 {{ filteredRepoIsoList.length }} 条</span>
        </div>
      </template>
    </el-table>

    <RepoManualEditDialog
      v-model="manualEditVisible"
      :repo-id="props.repoId"
      :iso-record="activeIsoRecord"
      :refresh-proposal="activeRefreshProposal"
      @update:refresh-proposal="handleActiveRefreshProposalUpdate"
    />

    <el-dialog
      v-model="refreshProposalQueueVisible"
      title="刷新提案队列"
      width="780px"
    >
      <div class="refresh-proposal-queue-wrap">
        <div class="refresh-proposal-queue-toolbar">
          <div class="refresh-proposal-queue-summary">
            共 {{ refreshProposalQueueItems.length }} 条待确认提案。点击“打开编辑器”后，仍需在编辑弹窗中再次确认提交。
            <span v-if="rememberedProposalStatusCount > 0">当前已记住 {{ rememberedProposalStatusCount }} 条忽略/已处理记录。</span>
          </div>
          <div class="refresh-proposal-queue-toolbar-actions">
            <el-button size="small" :disabled="refreshProposalQueueCount === 0" @click="clearRefreshProposalQueue">清空队列</el-button>
            <el-button size="small" plain :disabled="rememberedProposalStatusCount === 0" @click="clearRememberedProposalStatuses">清空忽略/已处理记录</el-button>
          </div>
        </div>

        <div v-if="refreshProposalQueueItems.length" class="refresh-proposal-queue-list">
          <div v-for="item in refreshProposalQueueItems" :key="`proposal-${item.iso_id}`" class="refresh-proposal-queue-item">
            <div class="refresh-proposal-queue-main">
              <div class="refresh-proposal-queue-path">{{ item.path }}</div>
              <div class="refresh-proposal-queue-meta">
                <el-tag size="small" type="warning" effect="plain">{{ item.is_directory ? '目录' : (item.item_kind === 'archive' ? 'Archive' : '文件') }}</el-tag>
                <el-tag
                  v-for="field in proposalChangedFieldLabels(item.metadata_proposal)"
                  :key="`${item.iso_id}-${field}`"
                  size="small"
                  type="danger"
                  effect="plain"
                >
                  {{ field }}
                </el-tag>
              </div>
            </div>
            <div class="refresh-proposal-queue-actions">
              <el-button size="small" type="warning" plain @click="openRefreshProposalItem(item)">打开编辑器</el-button>
              <el-button size="small" plain @click="ignoreRefreshProposalItem(item)">忽略</el-button>
            </div>
          </div>
        </div>
        <div v-else class="refresh-proposal-queue-empty">当前没有待确认的刷新提案。</div>
      </div>
    </el-dialog>

    <RepoDeleteDialog
      v-model="deleteDialogVisible"
      :repo-id="props.repoId"
      :iso-record="activeDeleteRecord"
    />

    <el-dialog
      v-model="singleMoveDialogVisible"
      title="单个要素迁移"
      width="640px"
    >
      <div class="single-move-content">
        <div class="single-move-row"><span class="single-move-label">源仓库ID</span><span>{{ props.repoId }}</span></div>
        <div class="single-move-row"><span class="single-move-label">要素</span><span class="break-all">{{ singleMoveRecord?.path || '-' }}</span></div>
        <div class="single-move-row">
          <span class="single-move-label">目标仓库</span>
          <el-select
            v-model="singleMoveTargetRepoId"
            placeholder="请选择目标仓库"
            filterable
            class="single-move-target-select"
            :loading="singleMoveReposLoading"
            :disabled="singleMoveSubmitting"
          >
            <el-option
              v-for="repo in singleMoveTargetOptions"
              :key="repo.id"
              :label="`${repo.name || '（未命名）'} (#${repo.id})`"
              :value="String(repo.id)"
            />
          </el-select>
        </div>
      </div>

      <template #footer>
        <el-button :disabled="singleMoveSubmitting" @click="singleMoveDialogVisible = false">取消</el-button>
        <el-button
          type="warning"
          :loading="singleMoveSubmitting"
          :disabled="!canSubmitSingleMove"
          @click="submitSingleMove"
        >
          迁移该要素
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Check, Delete, Right, Setting } from '@element-plus/icons-vue'
import emitter from '../eventBus'
import RepoManualEditDialog from './RepoManualEditDialog.vue'
import RepoDeleteDialog from './RepoDeleteDialog.vue'
import { findLatestTaskBySource, getTaskHint, isTaskBusy, startManagedDownload } from '../download/manager'
import { resolveMetadataDisplayConfig, shouldExposeMetadataFieldByConfig } from '../utils/repoMetadataDisplay'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  },
  refreshSignal: {
    type: Number,
    default: 0
  }
})

const repoIsoList = ref([])
const loading = ref(false)
const refreshProposalQueueLoading = ref(false)
const activeTypeFilter = ref('all')
const activeOSDistroFilter = ref('')
const activeMetadataKeyFilter = ref('')
const activeMetadataValueFilter = ref('')
const manualEditorMode = ref('legacy-type-editor')
const metadataDisplayMode = ref('hidden')
const metadataDisplayFields = ref('')
const archiveSubdir = ref('archives')
const manualEditVisible = ref(false)
const activeIsoRecord = ref(null)
const refreshProposalByIsoId = ref({})
const refreshProposalQueueVisible = ref(false)
const rememberedProposalStatuses = ref({})
const deleteButtonEnabled = ref(false)
const showMD5Column = ref(false)
const showSizeColumn = ref(false)
const singleMoveEnabled = ref(false)
const deleteDialogVisible = ref(false)
const activeDeleteRecord = ref(null)
const singleMoveDialogVisible = ref(false)
const singleMoveRecord = ref(null)
const singleMoveTargetRepoId = ref('')
const singleMoveTargetRepos = ref([])
const singleMoveReposLoading = ref(false)
const singleMoveSubmitting = ref(false)
const deferredRepoId = ref(null)
const delayedRefreshTimer = ref(null)
const fetchRepoIsosRequestSeq = ref(0)
const fetchRepoInfoRequestSeq = ref(0)

const OS_DISTRO_ALL_COMMAND = '__all__'
const osTopLevelTypeSegments = new Set(['linux', 'windows', 'macos', 'vmware'])

const showOSFilter = computed(() => {
  return repoIsoList.value.some((item) => isOSItem(item))
})

const showEntertainmentFilter = computed(() => {
  return repoIsoList.value.some((item) => isEntertainmentItem(item))
})

const showDirectoryFilter = computed(() => {
  return repoIsoList.value.some((item) => isDirectoryRow(item))
})

const showArchiveFilter = computed(() => {
  return repoIsoList.value.some((item) => isArchiveItem(item))
})

const showOthersFilter = computed(() => {
  return repoIsoList.value.some((item) => isOtherItem(item))
})

const showLegacyTypeFilters = computed(() => {
  return showDirectoryFilter.value || showOSFilter.value || showEntertainmentFilter.value || showArchiveFilter.value || showOthersFilter.value
})

const osTotalCount = computed(() => {
  return repoIsoList.value.filter((item) => isOSItem(item)).length
})

const osDistroOptions = computed(() => {
  const distroCounter = new Map()

  for (const item of repoIsoList.value) {
    const distro = extractOSDistro(item)
    if (!distro) {
      continue
    }
    distroCounter.set(distro, (distroCounter.get(distro) || 0) + 1)
  }

  return Array.from(distroCounter.entries())
    .map(([value, count]) => ({ value, label: value, count }))
    .sort((a, b) => a.label.localeCompare(b.label, 'en', { sensitivity: 'base' }))
})

const osFilterButtonLabel = computed(() => {
  return activeOSDistroFilter.value || 'OS'
})

const typeFilterButtonLabel = computed(() => {
  if (activeTypeFilter.value === 'directory') {
    return '目录'
  }
  if (activeTypeFilter.value === 'os') {
    return osFilterButtonLabel.value
  }
  if (activeTypeFilter.value === 'entertainment') {
    return '娱乐'
  }
  if (activeTypeFilter.value === 'archive') {
    return 'Archive'
  }
  if (activeTypeFilter.value === 'others') {
    return 'Others'
  }
  return '全部'
})

const metadataDisplayConfig = computed(() => resolveMetadataDisplayConfig({
  manual_editor_mode: manualEditorMode.value,
  metadata_display_mode: metadataDisplayMode.value,
  metadata_display_fields: metadataDisplayFields.value
}))

const metadataKeyOptions = computed(() => {
  const counter = new Map()
  for (const item of repoIsoList.value) {
    const metadata = extractRowMetadata(item)
    for (const [key, value] of Object.entries(metadata)) {
      const normalizedValue = normalizeMetadataValue(value)
      if (!shouldExposeMetadataField(key, normalizedValue)) {
        continue
      }
      counter.set(key, (counter.get(key) || 0) + 1)
    }
  }

  return Array.from(counter.entries())
    .map(([value, count]) => ({ value, label: metadataFieldLabel(value), count }))
    .sort((a, b) => a.label.localeCompare(b.label, 'zh-Hans-CN', { sensitivity: 'base' }))
})

const metadataValueOptions = computed(() => {
  const selectedKey = String(activeMetadataKeyFilter.value || '').trim()
  if (!selectedKey) {
    return []
  }

  const counter = new Map()
  for (const item of repoIsoList.value) {
    const metadata = extractRowMetadata(item)
    const normalizedValue = normalizeMetadataValue(metadata[selectedKey])
    if (!normalizedValue) {
      continue
    }
    counter.set(normalizedValue, (counter.get(normalizedValue) || 0) + 1)
  }

  return Array.from(counter.entries())
    .map(([value, count]) => ({ value, label: value, count }))
    .sort((a, b) => a.label.localeCompare(b.label, 'zh-Hans-CN', { sensitivity: 'base' }))
})

const showMetadataFilter = computed(() => {
  return metadataDisplayConfig.value.mode !== 'hidden' && metadataKeyOptions.value.length > 0
})

const singleMoveTargetOptions = computed(() => {
  return singleMoveTargetRepos.value.filter((repo) => Number(repo.id) !== Number(props.repoId))
})

const canSubmitSingleMove = computed(() => {
  return !!singleMoveRecord.value?.id && !!singleMoveTargetRepoId.value && !singleMoveSubmitting.value
})

const refreshProposalQueueItems = computed(() => {
  return Object.values(refreshProposalByIsoId.value)
    .filter((item) => item && typeof item === 'object' && item.metadata_proposal)
    .sort((left, right) => Number(right?.iso_id || 0) - Number(left?.iso_id || 0))
})

const refreshProposalQueueCount = computed(() => refreshProposalQueueItems.value.length)
const rememberedProposalStatusCount = computed(() => Object.keys(rememberedProposalStatuses.value || {}).length)

const activeRefreshProposal = computed(() => {
  const isoId = Number(activeIsoRecord.value?.id || 0)
  if (!isoId) {
    return null
  }
  return refreshProposalByIsoId.value[String(isoId)]?.metadata_proposal || null
})

const filteredRepoIsoList = computed(() => {
  let items = repoIsoList.value

  if (activeTypeFilter.value === 'directory') {
    items = items.filter((item) => isDirectoryRow(item))
  } else if (activeTypeFilter.value === 'os') {
    items = items.filter((item) => {
      if (!isOSItem(item)) {
        return false
      }
      if (!activeOSDistroFilter.value) {
        return true
      }
      return extractOSDistro(item) === activeOSDistroFilter.value
    })
  } else if (activeTypeFilter.value === 'entertainment') {
    items = items.filter((item) => isEntertainmentItem(item))
  } else if (activeTypeFilter.value === 'archive') {
    items = items.filter((item) => isArchiveItem(item))
  } else if (activeTypeFilter.value === 'others') {
    items = items.filter((item) => isOtherItem(item))
  }

  if (activeMetadataKeyFilter.value) {
    items = items.filter((item) => {
      const metadata = extractRowMetadata(item)
      const normalizedValue = normalizeMetadataValue(metadata[activeMetadataKeyFilter.value])
      if (!normalizedValue) {
        return false
      }
      if (!activeMetadataValueFilter.value) {
        return true
      }
      return normalizedValue === activeMetadataValueFilter.value
    })
  }

  return items
})

function normalizePath(path) {
  return String(path || '').replace(/\\/g, '/').trim()
}

function normalizeArchiveSubdirForDisplay(value) {
  const normalized = normalizePath(value).replace(/^\/+|\/+$/g, '')
  return normalized || 'archives'
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

function shouldExposeMetadataField(key, normalizedValue) {
  return shouldExposeMetadataFieldByConfig(key, normalizedValue, metadataDisplayConfig.value)
}

function metadataFieldLabel(key) {
  const mapping = {
    title: '标题',
    archive_format: '压缩格式',
    archive_storage_path: 'Archive 路径',
    lifecycle: '状态',
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
    source_path: '来源路径',
    original_name: '原始名称'
  }
  return mapping[key] || key
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

function metadataPreviewEntries(item) {
  if (metadataDisplayConfig.value.mode === 'hidden') {
    return []
  }
  const metadata = extractRowMetadata(item)
  const preferredKeys = metadataDisplayConfig.value.mode === 'selected'
    ? metadataDisplayConfig.value.fields.filter((key) => key !== 'title')
    : ['series_name', 'scanlator_group', 'author_name', 'author_alias', 'original_work']
  const entries = []

  for (const key of preferredKeys) {
    const value = normalizeMetadataValue(metadata[key])
    if (!value) {
      continue
    }
    entries.push({ key, label: metadataFieldLabel(key), value })
    if (entries.length >= 4) {
      break
    }
  }

  return entries
}

function refreshProposalMapKey(isoId) {
  const value = Number(isoId || 0)
  return value > 0 ? String(value) : ''
}

function proposalStorageKey(repoId) {
  const value = Number(repoId || 0)
  return value > 0 ? `lazymanga:refresh-proposal-status:${value}` : ''
}

function proposalSignature(item) {
  const proposal = item?.metadata_proposal
  if (!proposal || typeof proposal !== 'object') {
    return ''
  }
  const changedFields = Array.isArray(proposal.changed_fields) ? proposal.changed_fields.map((field) => String(field || '').trim()).filter(Boolean).sort() : []
  const changes = proposal.changes && typeof proposal.changes === 'object' ? proposal.changes : {}
  const changeParts = changedFields.map((field) => {
    const entry = changes[field] || {}
    return `${field}:${JSON.stringify(entry.from ?? null)}=>${JSON.stringify(entry.to ?? null)}`
  })
  const analysisPath = String(proposal.analysis_path || '').trim()
  return `${Number(item?.iso_id || 0)}|${analysisPath}|${changeParts.join('|')}`
}

function loadRememberedProposalStatuses() {
  const key = proposalStorageKey(props.repoId)
  if (!key || typeof window === 'undefined' || !window.localStorage) {
    rememberedProposalStatuses.value = {}
    return
  }
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) {
      rememberedProposalStatuses.value = {}
      return
    }
    const parsed = JSON.parse(raw)
    rememberedProposalStatuses.value = parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch (_) {
    rememberedProposalStatuses.value = {}
  }
}

function persistRememberedProposalStatuses() {
  const key = proposalStorageKey(props.repoId)
  if (!key || typeof window === 'undefined' || !window.localStorage) {
    return
  }
  try {
    if (Object.keys(rememberedProposalStatuses.value || {}).length === 0) {
      window.localStorage.removeItem(key)
      return
    }
    window.localStorage.setItem(key, JSON.stringify(rememberedProposalStatuses.value))
  } catch (_) {
    // ignore local storage failures
  }
}

function rememberProposalStatus(item, status) {
  const signature = proposalSignature(item)
  if (!signature) {
    return
  }
  rememberedProposalStatuses.value = {
    ...rememberedProposalStatuses.value,
    [signature]: {
      status: String(status || '').trim() || 'ignored',
      updated_at: new Date().toISOString()
    }
  }
  persistRememberedProposalStatuses()
}

function clearRememberedProposalStatuses() {
  rememberedProposalStatuses.value = {}
  persistRememberedProposalStatuses()
}

function isProposalRemembered(item) {
  const signature = proposalSignature(item)
  return !!(signature && rememberedProposalStatuses.value[signature])
}

function hasRefreshProposal(item) {
  const key = refreshProposalMapKey(item?.id)
  return !!(key && refreshProposalByIsoId.value[key]?.metadata_proposal)
}

function proposalChangedFieldLabels(proposal) {
  const fields = Array.isArray(proposal?.changed_fields) ? proposal.changed_fields : []
  return fields.map((field) => metadataFieldLabel(String(field || '').trim())).filter(Boolean)
}

function setRefreshProposalItem(item) {
  const key = refreshProposalMapKey(item?.iso_id)
  if (!key || !item?.metadata_proposal) {
    return
  }
  refreshProposalByIsoId.value = {
    ...refreshProposalByIsoId.value,
    [key]: {
      ...item,
      iso_id: Number(item.iso_id)
    }
  }
}

function removeRefreshProposalItem(isoId) {
  const key = refreshProposalMapKey(isoId)
  if (!key || !refreshProposalByIsoId.value[key]) {
    return
  }
  const next = { ...refreshProposalByIsoId.value }
  delete next[key]
  refreshProposalByIsoId.value = next
}

function clearRefreshProposalQueue() {
  refreshProposalByIsoId.value = {}
}

function syncRefreshProposalQueueWithRows(rows) {
  const validIds = new Set((Array.isArray(rows) ? rows : []).map((row) => refreshProposalMapKey(row?.id)).filter(Boolean))
  const next = {}
  for (const [key, item] of Object.entries(refreshProposalByIsoId.value)) {
    if (!validIds.has(key)) {
      continue
    }
    next[key] = item
  }
  refreshProposalByIsoId.value = next
}

function isDirectoryRow(item) {
  return !!(item?.is_directory ?? item?.isDirectory)
}

function isArchiveItem(item) {
  const direct = String(item?.item_kind || item?.itemKind || '').trim().toLowerCase()
  if (direct === 'archive') {
    return true
  }
  const metadata = extractRowMetadata(item)
  const metadataKind = String(metadata?.item_kind || '').trim().toLowerCase()
  return metadataKind === 'archive'
}

function isOSItem(item) {
  return !!item?.is_os && !item?.is_entertament
}

function isEntertainmentItem(item) {
  return !!item?.is_entertament && !item?.is_os
}

function isOtherItem(item) {
  return !isDirectoryRow(item) && !isArchiveItem(item) && !isOSItem(item) && !isEntertainmentItem(item)
}

function isRowMissing(item) {
  return !!(item?.is_missing ?? item?.isMissing)
}

function resolveRowClassName({ row }) {
  return isRowMissing(row) ? 'repoiso-missing-row' : ''
}

function canShowDeleteButton(row) {
  return deleteButtonEnabled.value || isRowMissing(row)
}

function looksLikeFileSegment(segment) {
  return /\.[a-z0-9]{2,8}$/i.test(String(segment || ''))
}

function extractOSDistro(item) {
  if (!isOSItem(item)) {
    return ''
  }

  const parts = normalizePath(item?.path).split('/').filter(Boolean)
  if (parts.length < 2) {
    return ''
  }

  let distroIndex = 1
  if (parts.length >= 3 && osTopLevelTypeSegments.has(parts[1].toLowerCase())) {
    distroIndex = 2
  }

  const distro = String(parts[distroIndex] || '').trim()
  if (!distro || looksLikeFileSegment(distro)) {
    return ''
  }

  return distro
}

function extractFileName(path) {
  const normalized = normalizePath(path)
  const parts = normalized.split('/')
  return parts[parts.length - 1] || normalized
}

function formatDisplayPath(item) {
  const path = normalizePath(item?.path)
  if (!isArchiveItem(item)) {
    return path
  }
  const prefix = normalizeArchiveSubdirForDisplay(archiveSubdir.value)
  if (!prefix) {
    return path
  }
  if (path === prefix) {
    return ''
  }
  if (path.startsWith(prefix + '/')) {
    return path.slice(prefix.length + 1)
  }
  return path
}

function resolvePrimaryDisplayLabel(item) {
  const metadata = extractRowMetadata(item)
  const metadataTitle = normalizeMetadataValue(metadata?.title)
  if (metadataTitle) {
    return metadataTitle
  }
  const fileName = String(item?.filename || item?.fileName || '').trim()
  if (fileName) {
    return fileName
  }
  const displayPath = formatDisplayPath(item)
  if (displayPath) {
    return extractFileName(displayPath)
  }
  return '-'
}

function extractElementSuffix(item) {
  if (isDirectoryRow(item)) return ''
  const name = String(item?.filename || item?.fileName || extractFileName(item?.path || '') || '').trim()
  const match = name.match(/(\.[a-z0-9]{1,12})$/i)
  return match ? match[1].toLowerCase() : ''
}

function formatElementType(item) {
  if (isDirectoryRow(item)) return '目录'
  if (isArchiveItem(item)) {
    const suffix = extractElementSuffix(item)
    return suffix ? `Archive ${suffix}` : 'Archive'
  }
  const suffix = extractElementSuffix(item)
  if (suffix) return `${suffix} 文件`
  return '文件'
}

function elementTagType(item) {
  if (isDirectoryRow(item)) return 'success'
  if (isArchiveItem(item)) return 'warning'
  const suffix = extractElementSuffix(item)
  if (suffix === '.iso') return 'warning'
  if (suffix) return 'primary'
  return 'info'
}

function setTypeFilter(type) {
  activeTypeFilter.value = type
}

function handleTypeFilterCommand(command) {
  const type = String(command || '').trim()
  if (!type || type === 'all') {
    activeOSDistroFilter.value = ''
    setTypeFilter('all')
    return
  }

  if (type === 'os') {
    activateOSFilter()
    return
  }

  activeOSDistroFilter.value = ''
  setTypeFilter(type)
}

function handleMetadataKeyChange(value) {
  activeMetadataKeyFilter.value = String(value || '').trim()
  activeMetadataValueFilter.value = ''
}

function handleMetadataValueChange(value) {
  activeMetadataValueFilter.value = String(value || '').trim()
}

function activateOSFilter() {
  activeTypeFilter.value = 'os'
  activeOSDistroFilter.value = ''
}

function handleOSDistroCommand(command) {
  activeTypeFilter.value = 'os'

  const value = String(command || '').trim()
  if (!value || value === OS_DISTRO_ALL_COMMAND) {
    activeOSDistroFilter.value = ''
    return
  }

  activeOSDistroFilter.value = value
}

function openManualEdit(row) {
  activeIsoRecord.value = row ? { ...row } : null
  manualEditVisible.value = true
}

function openRefreshProposalItem(item) {
  const row = repoIsoList.value.find((candidate) => Number(candidate?.id) === Number(item?.iso_id))
  if (!row) {
    ElMessage.error('当前提案对应的记录已不存在，请先刷新列表')
    removeRefreshProposalItem(item?.iso_id)
    return
  }
  setRefreshProposalItem(item)
  activeIsoRecord.value = { ...row }
  manualEditVisible.value = true
  refreshProposalQueueVisible.value = false
}

function ignoreRefreshProposalItem(item) {
  rememberProposalStatus(item, 'ignored')
  removeRefreshProposalItem(item?.iso_id)
}

function handleActiveRefreshProposalUpdate(proposal) {
  const isoId = Number(activeIsoRecord.value?.id || 0)
  if (!isoId) {
    return
  }
  if (proposal && typeof proposal === 'object') {
    const existing = refreshProposalByIsoId.value[String(isoId)] || {}
    setRefreshProposalItem({
      ...existing,
      iso_id: isoId,
      path: activeIsoRecord.value?.path || existing.path || '',
      file_name: activeIsoRecord.value?.filename || activeIsoRecord.value?.fileName || existing.file_name || '',
      is_directory: !!(activeIsoRecord.value?.is_directory ?? activeIsoRecord.value?.isDirectory ?? existing.is_directory),
      item_kind: String(activeIsoRecord.value?.item_kind || activeIsoRecord.value?.itemKind || existing.item_kind || '').trim(),
      metadata_proposal: proposal
    })
    return
  }
  const existing = refreshProposalByIsoId.value[String(isoId)]
  if (existing) {
    rememberProposalStatus(existing, 'processed')
  }
  removeRefreshProposalItem(isoId)
}

function openDeleteDialog(row) {
  activeDeleteRecord.value = row ? { ...row } : null
  deleteDialogVisible.value = true
}

async function fetchSingleMoveTargetRepos() {
  singleMoveReposLoading.value = true
  try {
    const res = await fetch('/api/repos')
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库列表失败'))
    }

    const data = await res.json()
    singleMoveTargetRepos.value = Array.isArray(data) ? data : []
  } catch (e) {
    singleMoveTargetRepos.value = []
    ElMessage.error(e.message || '获取仓库列表失败')
  } finally {
    singleMoveReposLoading.value = false
  }
}

async function openSingleMoveDialog(row) {
  singleMoveRecord.value = row ? { ...row } : null
  singleMoveTargetRepoId.value = ''
  singleMoveDialogVisible.value = true
  await fetchSingleMoveTargetRepos()
}

async function submitSingleMove() {
  if (!singleMoveRecord.value?.id || !singleMoveTargetRepoId.value) {
    ElMessage.warning('请选择目标仓库')
    return
  }

  singleMoveSubmitting.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos/${singleMoveRecord.value.id}/move`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target_repo_id: Number(singleMoveTargetRepoId.value) })
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '单个要素迁移失败'))
    }

    const data = await res.json()
    singleMoveDialogVisible.value = false
    emitter.emit('refresh-repo', { repoId: props.repoId })
    emitter.emit('refresh-repo', { repoId: Number(singleMoveTargetRepoId.value) })
    emitter.emit('refresh-all')
    ElMessage.success(data?.message || '单个要素迁移成功')
  } catch (e) {
    ElMessage.error(e.message || '单个要素迁移失败')
  } finally {
    singleMoveSubmitting.value = false
  }
}

function resolveRecordSizeBytes(value) {
  if (value && typeof value === 'object') {
    const candidates = [value.size_bytes, value.sizeBytes, value.size]
    for (const candidate of candidates) {
      const parsed = Number(candidate)
      if (Number.isFinite(parsed)) {
        return parsed
      }
    }
    return null
  }

  const parsed = Number(value)
  if (!Number.isFinite(parsed)) {
    return null
  }
  return parsed
}

function formatSize(v) {
  const size = resolveRecordSizeBytes(v)
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

function ensureActiveTypeFilterValid() {
  if (!showLegacyTypeFilters.value) {
    activeOSDistroFilter.value = ''
    activeTypeFilter.value = 'all'
  }

  if (activeTypeFilter.value === 'os' && !showOSFilter.value) {
    activeOSDistroFilter.value = ''
    activeTypeFilter.value = 'all'
    return
  }

  if (activeOSDistroFilter.value) {
    const exists = osDistroOptions.value.some((option) => option.value === activeOSDistroFilter.value)
    if (!exists) {
      activeOSDistroFilter.value = ''
    }
  }

  if (activeTypeFilter.value === 'entertainment' && !showEntertainmentFilter.value) {
    activeTypeFilter.value = 'all'
    return
  }
  if (activeTypeFilter.value === 'directory' && !showDirectoryFilter.value) {
    activeTypeFilter.value = 'all'
    return
  }
  if (activeTypeFilter.value === 'archive' && !showArchiveFilter.value) {
    activeTypeFilter.value = 'all'
    return
  }
  if (activeTypeFilter.value === 'others' && !showOthersFilter.value) {
    activeTypeFilter.value = 'all'
  }

  if (activeMetadataKeyFilter.value) {
    const keyExists = metadataKeyOptions.value.some((option) => option.value === activeMetadataKeyFilter.value)
    if (!keyExists) {
      activeMetadataKeyFilter.value = ''
      activeMetadataValueFilter.value = ''
      return
    }
  }

  if (activeMetadataValueFilter.value) {
    const valueExists = metadataValueOptions.value.some((option) => option.value === activeMetadataValueFilter.value)
    if (!valueExists) {
      activeMetadataValueFilter.value = ''
    }
  }
}

function clearDelayedRefreshTimer() {
  if (delayedRefreshTimer.value) {
    clearTimeout(delayedRefreshTimer.value)
    delayedRefreshTimer.value = null
  }
}

function scheduleInitialRefresh(delayMs = 0) {
  clearDelayedRefreshTimer()

  if (delayMs > 0) {
    const targetRepoId = Number(props.repoId)
    delayedRefreshTimer.value = setTimeout(() => {
      delayedRefreshTimer.value = null
      if (Number(props.repoId) !== targetRepoId) {
        return
      }
      deferredRepoId.value = null
      fetchRepoIsos()
      fetchRepoInfo()
    }, delayMs)
    return
  }

  deferredRepoId.value = null
  fetchRepoIsos()
  fetchRepoInfo()
}

function handleRefreshRepo(payload) {
  const repoId = Number(payload?.repoId)
  if (!repoId || repoId !== props.repoId) {
    return
  }

  if (Number(deferredRepoId.value) === Number(props.repoId)) {
    return
  }
  fetchRepoIsos()
}

function handleRefreshAll() {
  if (Number(deferredRepoId.value) === Number(props.repoId)) {
    return
  }

  fetchRepoIsos()
  fetchRepoInfo()
}

function handleRepoCreatedActivated(payload) {
  const repoId = Number(payload?.repoId)
  deferredRepoId.value = repoId > 0 ? repoId : null
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

function buildRowDownloadSourceKey(row) {
  return `repo:${props.repoId}:${row?.id || ''}`
}

function getRowDownloadTask(row) {
  return findLatestTaskBySource(buildRowDownloadSourceKey(row))
}

function isRowDownloading(row) {
  return isTaskBusy(getRowDownloadTask(row))
}

function getRowDownloadHint(row) {
  return getTaskHint(getRowDownloadTask(row))
}

async function handleDownload(row) {
  if (!row?.id) {
    ElMessage.error('未获取到要素记录ID，无法下载')
    return
  }

  if (isDirectoryRow(row)) {
    ElMessage.error('目录条目不支持单文件下载')
    return
  }

  if (isRowMissing(row)) {
    ElMessage.error('当前文件已失踪，无法下载')
    return
  }

  // Preflight existence check so missing files never start download tasks.
  try {
    const refreshRes = await fetch(`/api/repos/${props.repoId}/repoisos/${row.id}/refresh`, {
      method: 'POST'
    })
    if (!refreshRes.ok) {
      throw new Error(await parseErrorMessage(refreshRes, '下载前检查文件状态失败'))
    }

    const refreshData = await refreshRes.json()
    if (!refreshData?.exists) {
      emitter.emit('refresh-repo', { repoId: props.repoId })
      ElMessage.error('当前文件已失踪，无法下载')
      return
    }
  } catch (e) {
    ElMessage.error(e.message || '下载前检查文件状态失败')
    return
  }

  try {
    const fallbackFileName = row.filename || extractFileName(row.path) || 'download.bin'
    const result = await startManagedDownload({
      sourceKey: buildRowDownloadSourceKey(row),
      sourceLabel: `仓库 #${props.repoId}`,
      url: `/api/repos/${props.repoId}/repoisos/${row.id}/download`,
      fallbackFileName
    })

    if (result.ok) {
      if (result.task.status === 'delegated') {
        ElMessage.success('已交给浏览器下载管理：' + (result.task.fileName || fallbackFileName))
      } else {
        ElMessage.success('下载已开始：' + (result.task.fileName || fallbackFileName))
      }
      return
    }

    ElMessage.error(result.task.errorMessage || '下载失败')
  } catch (e) {
    ElMessage.error(e.message || '下载失败')
  }
}

async function fetchRefreshProposalQueue() {
  if (!props.repoId) {
    clearRefreshProposalQueue()
    return
  }

  refreshProposalQueueLoading.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos/refresh-proposals`, {
      method: 'POST'
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '批量生成刷新提案失败'))
    }

    const data = await res.json()
    const items = Array.isArray(data?.items) ? data.items : []
    const next = {}
    for (const item of items) {
      const key = refreshProposalMapKey(item?.iso_id)
      if (!key || !item?.metadata_proposal) {
        continue
      }
      if (isProposalRemembered(item)) {
        continue
      }
      next[key] = item
    }
    refreshProposalByIsoId.value = next
    if (items.length > 0) {
      refreshProposalQueueVisible.value = true
      ElMessage.success(`已生成 ${items.length} 条待确认提案`)
    } else {
      ElMessage.info('当前没有识别到需要确认的 metadata 提案')
    }
  } catch (e) {
    ElMessage.error(e.message || '批量生成刷新提案失败')
  } finally {
    refreshProposalQueueLoading.value = false
  }
}

async function fetchRepoIsos() {
  if (!props.repoId) {
    repoIsoList.value = []
    return
  }

  const requestSeq = ++fetchRepoIsosRequestSeq.value
  loading.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取管理要素信息表失败'))
    }

    const data = await res.json()
    if (requestSeq !== fetchRepoIsosRequestSeq.value) {
      return
    }
    repoIsoList.value = Array.isArray(data) ? data : []
    syncRefreshProposalQueueWithRows(repoIsoList.value)
    ensureActiveTypeFilterValid()
  } catch (e) {
    console.error('[RepoIsoTable] fetchRepoIsos failed', e)
    if (requestSeq !== fetchRepoIsosRequestSeq.value) {
      return
    }
    repoIsoList.value = []
    clearRefreshProposalQueue()
    activeTypeFilter.value = 'all'
    activeOSDistroFilter.value = ''
    activeMetadataKeyFilter.value = ''
    activeMetadataValueFilter.value = ''
    ElMessage.error(e.message || '获取管理要素信息表失败')
  } finally {
    if (requestSeq === fetchRepoIsosRequestSeq.value) {
      loading.value = false
    }
  }
}

async function fetchRepoInfo() {
  if (!props.repoId) {
    deleteButtonEnabled.value = false
    showMD5Column.value = false
    showSizeColumn.value = false
    singleMoveEnabled.value = false
    manualEditorMode.value = 'legacy-type-editor'
    metadataDisplayMode.value = 'hidden'
    metadataDisplayFields.value = ''
    archiveSubdir.value = 'archives'
    return
  }

  const requestSeq = ++fetchRepoInfoRequestSeq.value
  try {
    const res = await fetch(`/api/repos/${props.repoId}/type-settings`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库类型设置失败'))
    }

    const data = await res.json()
    if (requestSeq !== fetchRepoInfoRequestSeq.value) {
      return
    }
    const effective = data?.effective || {}
    deleteButtonEnabled.value = !!effective?.delete_button
    showMD5Column.value = !!effective?.show_md5
    showSizeColumn.value = !!effective?.show_size
    singleMoveEnabled.value = !!effective?.single_move
    manualEditorMode.value = String(effective?.manual_editor_mode || effective?.manualEditorMode || 'legacy-type-editor')
    metadataDisplayMode.value = String(effective?.metadata_display_mode || 'hidden')
    metadataDisplayFields.value = String(effective?.metadata_display_fields || '')
    archiveSubdir.value = normalizeArchiveSubdirForDisplay(effective?.archive_subdir || 'archives')
    ensureActiveTypeFilterValid()
  } catch (e) {
    console.error('[RepoIsoTable] fetchRepoInfo failed', e)
    if (requestSeq !== fetchRepoInfoRequestSeq.value) {
      return
    }
    deleteButtonEnabled.value = false
    showMD5Column.value = false
    showSizeColumn.value = false
    singleMoveEnabled.value = false
    manualEditorMode.value = 'legacy-type-editor'
    metadataDisplayMode.value = 'hidden'
    metadataDisplayFields.value = ''
    archiveSubdir.value = 'archives'
  }
}

onMounted(() => {
  emitter.on('refresh-all', handleRefreshAll)
  emitter.on('refresh-repo', handleRefreshRepo)
  emitter.on('repo-refresh-proposals', handleRefreshProposalRequest)
  emitter.on('repo-open-refresh-proposals', handleOpenRefreshProposalQueue)
  emitter.on('repo-created-activated', handleRepoCreatedActivated)
  loadRememberedProposalStatuses()
})

onUnmounted(() => {
  clearDelayedRefreshTimer()
  emitter.off('refresh-all', handleRefreshAll)
  emitter.off('refresh-repo', handleRefreshRepo)
  emitter.off('repo-refresh-proposals', handleRefreshProposalRequest)
  emitter.off('repo-open-refresh-proposals', handleOpenRefreshProposalQueue)
  emitter.off('repo-created-activated', handleRepoCreatedActivated)
})

function handleRefreshProposalRequest(payload) {
  const repoId = Number(payload?.repoId)
  if (!repoId || repoId !== props.repoId) {
    return
  }
  fetchRefreshProposalQueue()
}

function handleOpenRefreshProposalQueue(payload) {
  const repoId = Number(payload?.repoId)
  if (!repoId || repoId !== props.repoId) {
    return
  }
  if (refreshProposalQueueCount.value > 0) {
    refreshProposalQueueVisible.value = true
    return
  }
  ElMessage.info('当前没有待确认的提案队列')
}

watch(
  () => props.repoId,
  () => {
    deleteDialogVisible.value = false
    activeDeleteRecord.value = null
    singleMoveDialogVisible.value = false
    singleMoveRecord.value = null
    singleMoveTargetRepoId.value = ''
    singleMoveTargetRepos.value = []
    repoIsoList.value = []
    refreshProposalQueueVisible.value = false
    clearRefreshProposalQueue()
    loadRememberedProposalStatuses()
    activeTypeFilter.value = 'all'
    activeOSDistroFilter.value = ''
    activeMetadataKeyFilter.value = ''
    activeMetadataValueFilter.value = ''
    deleteButtonEnabled.value = false
    showMD5Column.value = false
    showSizeColumn.value = false
    singleMoveEnabled.value = false
    manualEditorMode.value = 'legacy-type-editor'
    metadataDisplayMode.value = 'hidden'
    metadataDisplayFields.value = ''

    if (Number(deferredRepoId.value) === Number(props.repoId)) {
      scheduleInitialRefresh(500)
      return
    }

    scheduleInitialRefresh(0)
  },
  { immediate: true }
)

watch(
  () => props.refreshSignal,
  () => {
    if (!props.repoId) {
      return
    }

    fetchRepoIsos()
    fetchRepoInfo()
  }
)
</script>

<style scoped>
.repoiso-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.repoiso-toolbar-summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.repoiso-toolbar-title {
  font-size: 14px;
  font-weight: 700;
  color: #7c2d12;
}

.repoiso-toolbar-hint {
  font-size: 12px;
  color: #92400e;
}

.repoiso-toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.el-table {
  --el-table-border-color: #94a3b8;
  --el-table-border: 2px solid #94a3b8;
  border-radius: 8px;
  font-size: 16px;
}

.el-table th,
.el-table td {
  border-right: 2px solid #94a3b8 !important;
  border-bottom: 2px solid #94a3b8 !important;
}

.el-table th:last-child,
.el-table td:last-child {
  border-right: none !important;
}

.el-table tr:last-child td {
  border-bottom: none !important;
}

:deep(.size-column-header .cell),
:deep(.size-column .cell) {
  padding-left: 6px !important;
  padding-right: 10px !important;
}

:deep(.action-column-header .cell),
:deep(.action-column .cell) {
  padding-left: 6px !important;
  padding-right: 6px !important;
}

.type-filter-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.type-column-header {
  display: inline-flex;
  flex-direction: row;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.type-column-title {
  font-weight: 700;
  color: #334155;
  line-height: 1.2;
  flex-shrink: 0;
}

.type-filter-trigger {
  min-width: 44px;
  padding-left: 6px;
  padding-right: 6px;
  font-size: 12px;
}

.type-filter-trigger :deep(.el-icon) {
  margin-left: 4px;
}

.metadata-filter-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.metadata-filter-select {
  width: 150px;
}

.metadata-filter-value-select {
  width: 210px;
}

.row-actions {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  flex-wrap: wrap;
}

.download-action-button {
  padding-left: 8px;
  padding-right: 8px;
}

.row-download-hint {
  color: #475569;
  font-size: 12px;
  line-height: 1.3;
  max-width: 220px;
}

.row-missing-hint {
  color: #b91c1c;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.3;
}

.row-proposal-hint {
  color: #c2410c;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.3;
}

:deep(.el-table__body tr.repoiso-missing-row > td) {
  background: #fff1f2;
}

.path-missing {
  color: #b91c1c;
  text-decoration: line-through;
}

.metadata-preview-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  width: 100%;
  min-width: 0;
}

.metadata-preview-tag {
  max-width: min(100%, 240px);
  min-width: 0;
}

.metadata-proposal-tag {
  border-color: #fca5a5;
}

.metadata-preview-tag :deep(.el-tag__content) {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.single-move-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.refresh-proposal-queue-wrap {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.refresh-proposal-queue-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.refresh-proposal-queue-toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.refresh-proposal-queue-summary {
  font-size: 13px;
  color: #78350f;
  line-height: 1.6;
}

.refresh-proposal-queue-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 60vh;
  overflow: auto;
}

.refresh-proposal-queue-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border-radius: 10px;
  border: 1px solid #fed7aa;
  background: #fff7ed;
}

.refresh-proposal-queue-main {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  flex: 1 1 auto;
}

.refresh-proposal-queue-path {
  font-size: 14px;
  line-height: 1.6;
  color: #431407;
  word-break: break-all;
}

.refresh-proposal-queue-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.refresh-proposal-queue-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex-shrink: 0;
}

.refresh-proposal-queue-empty {
  padding: 28px 16px;
  text-align: center;
  color: #92400e;
  border: 1px dashed #fdba74;
  border-radius: 10px;
  background: #fff7ed;
}

.single-move-row {
  display: grid;
  grid-template-columns: 92px minmax(0, 1fr);
  gap: 8px;
  align-items: center;
}

.single-move-label {
  color: #64748b;
  font-weight: 600;
  font-size: 13px;
}

.single-move-target-select {
  width: 100%;
}

.os-filter-dropdown :deep(.el-button-group .el-button:not(.el-dropdown__caret-button)) {
  min-width: 52px;
}

.os-filter-dropdown :deep(.el-button-group .el-dropdown__caret-button) {
  min-width: 24px;
  width: 24px;
  padding-left: 6px;
  padding-right: 6px;
}

.os-distro-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  min-width: 180px;
}

.os-distro-option-meta {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #475569;
}

.os-distro-count {
  min-width: 1.5em;
  text-align: right;
}

.os-path-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.os-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  color: #ffffff;
  background-color: #2563eb;
  line-height: 1.3;
  flex-shrink: 0;
}

.os-file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.others-path-cell {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.path-preview-stack {
  display: flex;
  flex: 1 1 0;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.others-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  color: #ffffff;
  background-color: #6b7280;
  line-height: 1.3;
  flex-shrink: 0;
}

.archive-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  color: #9a3412;
  background-color: #ffedd5;
  line-height: 1.3;
  flex-shrink: 0;
}

.others-primary-name {
  display: block;
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 600;
}

.entertainment-path-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.entertainment-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  color: #111827;
  background-color: #facc15;
  line-height: 1.3;
  flex-shrink: 0;
}

.entertainment-file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

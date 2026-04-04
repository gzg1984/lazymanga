<template>
  <div class="iso-table-wrap p-6 border-4 border-slate-400 rounded-lg bg-slate-100">
    <el-table
      v-loading="loading"
      :data="filteredRepoIsoList"
      :row-class-name="resolveRowClassName"
      style="width: 100%"
      border
    >
      <el-table-column prop="path" min-width="360">
        <template #header>
          <div class="type-filter-actions">
            <el-button
              size="small"
              :type="activeTypeFilter === 'all' ? 'primary' : 'info'"
              :plain="activeTypeFilter !== 'all'"
              @click.stop="setTypeFilter('all')"
            >
              全部
            </el-button>
            <el-dropdown
              v-if="showOSFilter"
              class="os-filter-dropdown"
              split-button
              trigger="click"
              size="small"
              :type="activeTypeFilter === 'os' ? 'primary' : 'info'"
              @click.stop="activateOSFilter"
              @command="handleOSDistroCommand"
            >
              {{ osFilterButtonLabel }}
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item :command="OS_DISTRO_ALL_COMMAND">
                    <div class="os-distro-option">
                      <span>全部发行版 ({{ osTotalCount }})</span>
                      <span class="os-distro-option-meta">
                        <el-icon v-if="activeOSDistroFilter === ''"><Check /></el-icon>
                      </span>
                    </div>
                  </el-dropdown-item>
                  <el-dropdown-item
                    v-for="option in osDistroOptions"
                    :key="option.value"
                    :command="option.value"
                  >
                    <div class="os-distro-option">
                      <span>{{ option.label }}</span>
                      <span class="os-distro-option-meta">
                        <span class="os-distro-count">{{ option.count }}</span>
                        <el-icon v-if="activeOSDistroFilter === option.value"><Check /></el-icon>
                      </span>
                    </div>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button
              v-if="showEntertainmentFilter"
              size="small"
              :type="activeTypeFilter === 'entertainment' ? 'primary' : 'info'"
              :plain="activeTypeFilter !== 'entertainment'"
              @click.stop="setTypeFilter('entertainment')"
            >
              娱乐
            </el-button>
            <el-button
              v-if="showOthersFilter"
              size="small"
              :type="activeTypeFilter === 'others' ? 'primary' : 'info'"
              :plain="activeTypeFilter !== 'others'"
              @click.stop="setTypeFilter('others')"
            >
              Others
            </el-button>
          </div>
        </template>
        <template #default="scope">
          <div v-if="isOSItem(scope.row)" :class="['os-path-cell', { 'path-missing': isRowMissing(scope.row) }]">
            <span class="os-badge">OS</span>
            <span class="os-file-name">{{ extractFileName(scope.row.path) }}</span>
          </div>
          <div v-else-if="isEntertainmentItem(scope.row)" :class="['entertainment-path-cell', { 'path-missing': isRowMissing(scope.row) }]">
            <span class="entertainment-badge">娱乐</span>
            <span class="entertainment-file-name">{{ extractFileName(scope.row.path) }}</span>
          </div>
          <div v-else-if="isOtherItem(scope.row)" :class="['others-path-cell', { 'path-missing': isRowMissing(scope.row) }]">
            <span class="others-badge">Others</span>
            <span class="others-full-path">{{ scope.row.path }}</span>
          </div>
          <span v-else :class="{ 'path-missing': isRowMissing(scope.row) }">{{ scope.row.path }}</span>
        </template>
      </el-table-column>
      <el-table-column v-if="showMD5Column" label="MD5" min-width="280">
        <template #default="scope">
          <span class="meta-text">{{ String(scope.row?.md5 || '').trim() || '待计算' }}</span>
        </template>
      </el-table-column>
      <el-table-column v-if="showSizeColumn" label="大小" width="140" align="right">
        <template #default="scope">
          <span class="meta-text">{{ formatSize(scope.row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="320" align="center">
        <template #default="scope">
          <div class="row-actions">
            <el-button
              v-if="!isRowMissing(scope.row)"
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
            <span v-if="isRowMissing(scope.row)" class="row-missing-hint">文件失踪</span>
            <span v-if="getRowDownloadHint(scope.row)" class="row-download-hint">{{ getRowDownloadHint(scope.row) }}</span>
          </div>
        </template>
      </el-table-column>
      <template #header>
        <div class="flex justify-between items-center w-full">
          <span>仓库ISO列表</span>
          <span class="text-xs text-slate-600">共 {{ filteredRepoIsoList.length }} 条</span>
        </div>
      </template>
    </el-table>

    <RepoManualEditDialog
      v-model="manualEditVisible"
      :repo-id="props.repoId"
      :iso-record="activeIsoRecord"
    />

    <RepoDeleteDialog
      v-model="deleteDialogVisible"
      :repo-id="props.repoId"
      :iso-record="activeDeleteRecord"
    />

    <el-dialog
      v-model="singleMoveDialogVisible"
      title="单个ISO迁移"
      width="640px"
    >
      <div class="single-move-content">
        <div class="single-move-row"><span class="single-move-label">源仓库ID</span><span>{{ props.repoId }}</span></div>
        <div class="single-move-row"><span class="single-move-label">ISO</span><span class="break-all">{{ singleMoveRecord?.path || '-' }}</span></div>
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
          迁移该 ISO
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
const activeTypeFilter = ref('all')
const activeOSDistroFilter = ref('')
const manualEditVisible = ref(false)
const activeIsoRecord = ref(null)
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

const showOthersFilter = computed(() => {
  return repoIsoList.value.some((item) => isOtherItem(item))
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

const singleMoveTargetOptions = computed(() => {
  return singleMoveTargetRepos.value.filter((repo) => Number(repo.id) !== Number(props.repoId))
})

const canSubmitSingleMove = computed(() => {
  return !!singleMoveRecord.value?.id && !!singleMoveTargetRepoId.value && !singleMoveSubmitting.value
})

const filteredRepoIsoList = computed(() => {
  if (activeTypeFilter.value === 'all') {
    return repoIsoList.value
  }

  if (activeTypeFilter.value === 'os') {
    return repoIsoList.value.filter((item) => {
      if (!isOSItem(item)) {
        return false
      }
      if (!activeOSDistroFilter.value) {
        return true
      }
      return extractOSDistro(item) === activeOSDistroFilter.value
    })
  }

  if (activeTypeFilter.value === 'entertainment') {
    return repoIsoList.value.filter((item) => isEntertainmentItem(item))
  }

  if (activeTypeFilter.value === 'others') {
    return repoIsoList.value.filter((item) => isOtherItem(item))
  }

  return repoIsoList.value
})

function normalizePath(path) {
  return String(path || '').replace(/\\/g, '/').trim()
}

function isOSItem(item) {
  return !!item?.is_os && !item?.is_entertament
}

function isEntertainmentItem(item) {
  return !!item?.is_entertament && !item?.is_os
}

function isOtherItem(item) {
  return !isOSItem(item) && !isEntertainmentItem(item)
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

function setTypeFilter(type) {
  activeTypeFilter.value = type
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
      throw new Error(await parseErrorMessage(res, '单个ISO迁移失败'))
    }

    const data = await res.json()
    singleMoveDialogVisible.value = false
    emitter.emit('refresh-repo', { repoId: props.repoId })
    emitter.emit('refresh-repo', { repoId: Number(singleMoveTargetRepoId.value) })
    emitter.emit('refresh-all')
    ElMessage.success(data?.message || '单个ISO迁移成功')
  } catch (e) {
    ElMessage.error(e.message || '单个ISO迁移失败')
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
  if (activeTypeFilter.value === 'others' && !showOthersFilter.value) {
    activeTypeFilter.value = 'all'
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
    ElMessage.error('未获取到ISO记录ID，无法下载')
    return
  }

  if (isRowMissing(row)) {
    ElMessage.error('当前ISO文件已失踪，无法下载')
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
      ElMessage.error('当前ISO文件已失踪，无法下载')
      return
    }
  } catch (e) {
    ElMessage.error(e.message || '下载前检查文件状态失败')
    return
  }

  try {
    const fallbackFileName = row.filename || extractFileName(row.path) || 'download.iso'
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
      throw new Error(await parseErrorMessage(res, '获取仓库ISO列表失败'))
    }

    const data = await res.json()
    if (requestSeq !== fetchRepoIsosRequestSeq.value) {
      return
    }
    repoIsoList.value = Array.isArray(data) ? data : []
    ensureActiveTypeFilterValid()
  } catch (e) {
    console.error('[RepoIsoTable] fetchRepoIsos failed', e)
    if (requestSeq !== fetchRepoIsosRequestSeq.value) {
      return
    }
    repoIsoList.value = []
    activeTypeFilter.value = 'all'
    activeOSDistroFilter.value = ''
    ElMessage.error(e.message || '获取仓库ISO列表失败')
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
    return
  }

  const requestSeq = ++fetchRepoInfoRequestSeq.value
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repo-info`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取 repo info 失败'))
    }

    const data = await res.json()
    if (requestSeq !== fetchRepoInfoRequestSeq.value) {
      return
    }
    deleteButtonEnabled.value = !!data?.delete_button
    showMD5Column.value = !!data?.show_md5
    showSizeColumn.value = !!data?.show_size
    singleMoveEnabled.value = !!data?.single_move
  } catch (e) {
    console.error('[RepoIsoTable] fetchRepoInfo failed', e)
    if (requestSeq !== fetchRepoInfoRequestSeq.value) {
      return
    }
    deleteButtonEnabled.value = false
    showMD5Column.value = false
    showSizeColumn.value = false
    singleMoveEnabled.value = false
  }
}

onMounted(() => {
  emitter.on('refresh-all', handleRefreshAll)
  emitter.on('refresh-repo', handleRefreshRepo)
  emitter.on('repo-created-activated', handleRepoCreatedActivated)
})

onUnmounted(() => {
  clearDelayedRefreshTimer()
  emitter.off('refresh-all', handleRefreshAll)
  emitter.off('refresh-repo', handleRefreshRepo)
  emitter.off('repo-created-activated', handleRepoCreatedActivated)
})

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
    activeTypeFilter.value = 'all'
    activeOSDistroFilter.value = ''
    deleteButtonEnabled.value = false
    showMD5Column.value = false
    showSizeColumn.value = false
    singleMoveEnabled.value = false

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

.type-filter-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.row-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
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

:deep(.el-table__body tr.repoiso-missing-row > td) {
  background: #fff1f2;
}

.path-missing {
  color: #b91c1c;
  text-decoration: line-through;
}

.single-move-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
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
  align-items: center;
  gap: 8px;
  width: 100%;
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

.others-full-path {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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

<template>
  <div>
    <el-button type="primary" size="small" @click="openDialog">迁移与合并</el-button>

    <el-dialog v-model="dialogVisible" title="迁移与合并" width="980px">
      <div class="merge-dialog-content" v-loading="loadingRepos">
        <div class="repo-panel source-panel">
          <div class="panel-title">源仓库（当前）</div>
          <div v-if="sourceRepo" class="panel-body">
            <div class="repo-name">{{ sourceRepo.name || '（未命名）' }}</div>
            <div class="repo-meta-row"><span class="meta-label">基础仓库</span>{{ sourceRepo.basic ? '是' : '否' }}</div>
            <div class="repo-meta-row"><span class="meta-label">ID</span>{{ sourceRepo.id }}</div>
            <div class="repo-meta-row"><span class="meta-label">路径</span>{{ sourceRepo.root_path || '（未设置）' }}</div>
            <div class="repo-meta-row"><span class="meta-label">DB</span>{{ sourceRepo.db_filename || 'repo.db' }}</div>
            <div class="repo-meta-row"><span class="meta-label">类型</span>{{ sourceRepo.is_internal ? '内部' : `外部(${sourceRepo.external_device_name || '-'})` }}</div>
            <div class="repo-meta-row">
              <span class="meta-label">仓库总体积</span>
              <span v-if="sourceSummaryLoading">加载中...</span>
              <span v-else>
                {{ formatBytesHuman(sourceSummary?.total_size_bytes || 0) }}
                <span v-if="sourceHasIncompleteSize" class="meta-warning">（数据不全，需刷新）</span>
              </span>
            </div>
          </div>
          <div v-else class="panel-empty">未找到当前仓库详情</div>
        </div>

        <div class="arrow-panel" aria-hidden="true">
          <el-icon class="merge-arrow"><Right /></el-icon>
        </div>

        <div class="repo-panel target-panel">
          <div class="panel-title">目标仓库</div>
          <el-select
            v-model="targetRepoId"
            placeholder="请选择目标仓库"
            filterable
            class="target-select"
            :disabled="submitting || loadingRepos || targetRepoOptions.length === 0 || mergeTaskRunning"
          >
            <el-option
              v-for="repo in targetRepoOptions"
              :key="repo.id"
              :label="`${repo.name || '（未命名）'} (#${repo.id})`"
              :value="String(repo.id)"
            />
          </el-select>

          <div v-if="targetRepo" class="panel-body">
            <div class="repo-name">{{ targetRepo.name || '（未命名）' }}</div>
            <div class="repo-meta-row"><span class="meta-label">ID</span>{{ targetRepo.id }}</div>
            <div class="repo-meta-row"><span class="meta-label">路径</span>{{ targetRepo.root_path || '（未设置）' }}</div>
            <div class="repo-meta-row"><span class="meta-label">DB</span>{{ targetRepo.db_filename || 'repo.db' }}</div>
            <div class="repo-meta-row"><span class="meta-label">类型</span>{{ targetRepo.is_internal ? '内部' : `外部(${targetRepo.external_device_name || '-'})` }}</div>
            <div class="repo-meta-row">
              <span class="meta-label">可用空间</span>
              <span v-if="targetSummaryLoading">加载中...</span>
              <span v-else>{{ formatBytesHuman(targetSummary?.available_bytes || 0) }}</span>
            </div>
          </div>
          <div v-else class="panel-empty">
            {{ targetRepoOptions.length === 0 ? '没有可选目标仓库（当前仅有一个仓库）' : '请选择一个目标仓库查看详情' }}
          </div>
        </div>
      </div>

      <div v-if="sourceRepo" class="merge-policy-tip" :class="sourceRepo.basic ? 'merge-policy-tip-safe' : 'merge-policy-tip-danger'">
        {{ sourceRepo.basic ? '当前源仓库带有 basic 标记。迁移成功后会保留该仓库，不会自动删除仓库信息。' : '当前源仓库不是 basic 仓库。迁移成功且源仓库已清空后，会自动删除该仓库信息。' }}
      </div>

      <div v-if="mergeTask" class="merge-task-progress">
        <div class="progress-title">
          任务状态：{{ mergeTask.status }}
          <span v-if="mergeTask.task_id" class="progress-task-id">#{{ mergeTask.task_id }}</span>
        </div>
        <el-progress :percentage="Math.round(mergeTaskProgressPercent)" :stroke-width="16" />
        <div class="progress-meta-row" v-if="mergeTask.current_file">
          <span class="progress-label">当前文件：</span>
          <span class="progress-value">{{ mergeTask.current_file }}</span>
        </div>
        <div class="progress-meta-row" v-if="mergeTask.current_step">
          <span class="progress-label">当前步骤：</span>
          <span class="progress-value">{{ mergeTask.current_step }}</span>
        </div>
        <div class="progress-meta-row">
          <span class="progress-label">进度：</span>
          <span class="progress-value">{{ mergeTask.processed || 0 }} / {{ mergeTask.total || 0 }}</span>
        </div>
        <div class="progress-meta-row" v-if="mergeTask.cleanup?.message">
          <span class="progress-label">清理结果：</span>
          <span class="progress-value">{{ mergeTask.cleanup.message }}</span>
        </div>
      </div>

      <template #footer>
        <div class="merge-dialog-footer">
          <el-button :disabled="submitting" @click="closeDialog">关闭</el-button>
          <el-button
            v-if="!mergeTaskCompleted"
            type="danger"
            :loading="submitting"
            :disabled="!canSubmit"
            @click="submitMergeRequest"
          >
            {{ mergeButtonLabel }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Right } from '@element-plus/icons-vue'
import emitter from '../eventBus'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  }
})

const dialogVisible = ref(false)
const loadingRepos = ref(false)
const submitting = ref(false)
const repos = ref([])
const targetRepoId = ref('')
const summaryByRepoId = ref({})
const summaryLoadingByRepoId = ref({})
const mergeTask = ref(null)
const mergeTaskPollTimer = ref(null)

const sourceRepo = computed(() => repos.value.find((repo) => Number(repo.id) === Number(props.repoId)) || null)
const targetRepoOptions = computed(() => repos.value.filter((repo) => Number(repo.id) !== Number(props.repoId)))
const targetRepo = computed(() => targetRepoOptions.value.find((repo) => String(repo.id) === targetRepoId.value) || null)
const sourceSummary = computed(() => getRepoSummary(props.repoId))
const targetSummary = computed(() => getRepoSummary(targetRepo.value?.id))
const sourceSummaryLoading = computed(() => isRepoSummaryLoading(props.repoId))
const targetSummaryLoading = computed(() => isRepoSummaryLoading(targetRepo.value?.id))
const sourceTotalBytes = computed(() => Number(sourceSummary.value?.total_size_bytes || 0))
const sourceHasIncompleteSize = computed(() => !!sourceSummary.value?.has_incomplete_size)
const targetAvailableBytes = computed(() => Number(targetSummary.value?.available_bytes || 0))
const mergeTaskRunning = computed(() => mergeTask.value?.status === 'running')
const mergeTaskCompleted = computed(() => mergeTask.value?.status === 'completed')
const mergeTaskProgressPercent = computed(() => Number(mergeTask.value?.progress_percent || 0))
const spaceCheckReady = computed(() => {
  if (!sourceRepo.value || !targetRepo.value) return false
  if (sourceSummaryLoading.value || targetSummaryLoading.value) return false
  return !!sourceSummary.value && !!targetSummary.value
})
const hasEnoughSpace = computed(() => {
  if (!spaceCheckReady.value) return false
  if (sourceHasIncompleteSize.value) return false
  return sourceTotalBytes.value < targetAvailableBytes.value
})
const canSubmit = computed(() => {
  return !!sourceRepo.value && !!targetRepo.value && hasEnoughSpace.value && !submitting.value && !mergeTaskRunning.value && !mergeTaskCompleted.value
})
const mergeButtonLabel = computed(() => {
  if (mergeTaskRunning.value) {
    return '迁移中...'
  }
  if (!targetRepo.value) {
    return '迁移与合并'
  }
  return hasEnoughSpace.value ? '迁移与合并' : '迁移与合并（空间不足）'
})

function stopMergeTaskPolling() {
  if (mergeTaskPollTimer.value) {
    clearInterval(mergeTaskPollTimer.value)
    mergeTaskPollTimer.value = null
  }
}

async function fetchMergeTask(taskId, silent = false) {
  const currentTaskId = String(taskId || '').trim()
  if (!currentTaskId) {
    return
  }

  try {
    const res = await fetch(`/api/repos/merge-transfer/tasks/${encodeURIComponent(currentTaskId)}`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '查询迁移进度失败'))
    }

    const previousStatus = mergeTask.value?.status
    const data = await res.json()
    mergeTask.value = data

    if (data?.status === 'completed' && previousStatus !== 'completed') {
      stopMergeTaskPolling()
      emitter.emit('refresh-all')
      ElMessage.success(data?.cleanup?.message || data?.message || '迁移与合并已完成')
      return
    }

    if (data?.status === 'failed' && previousStatus !== 'failed') {
      stopMergeTaskPolling()
      ElMessage.error(data?.error || data?.message || '迁移与合并失败')
      return
    }
  } catch (e) {
    if (!silent) {
      ElMessage.error(e.message || '查询迁移进度失败')
    }
  }
}

function startMergeTaskPolling(taskId) {
  stopMergeTaskPolling()
  fetchMergeTask(taskId)
  mergeTaskPollTimer.value = setInterval(() => {
    fetchMergeTask(taskId, true)
  }, 1000)
}

function parseBytes(v) {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

function formatBytesHuman(v) {
  const bytes = parseBytes(v)
  if (bytes <= 0) return '0 B'

  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let value = bytes
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

function repoKey(repoId) {
  if (!repoId) return ''
  return String(repoId)
}

function getRepoSummary(repoId) {
  const key = repoKey(repoId)
  if (!key) return null
  return summaryByRepoId.value[key] || null
}

function isRepoSummaryLoading(repoId) {
  const key = repoKey(repoId)
  if (!key) return false
  return !!summaryLoadingByRepoId.value[key]
}

async function fetchRepoStorageSummary(repoId) {
  const key = repoKey(repoId)
  if (!key) return

  summaryLoadingByRepoId.value = { ...summaryLoadingByRepoId.value, [key]: true }
  try {
    const res = await fetch(`/api/repos/${key}/storage-summary`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库存储信息失败'))
    }

    const data = await res.json()
    summaryByRepoId.value = { ...summaryByRepoId.value, [key]: data }
  } catch (e) {
    ElMessage.error(e.message || '获取仓库存储信息失败')
  } finally {
    summaryLoadingByRepoId.value = { ...summaryLoadingByRepoId.value, [key]: false }
  }
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

async function fetchRepos() {
  loadingRepos.value = true
  try {
    const res = await fetch('/api/repos')
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库列表失败'))
    }

    const list = await res.json()
    repos.value = Array.isArray(list) ? list : []
  } catch (e) {
    repos.value = []
    ElMessage.error(e.message || '获取仓库列表失败')
  } finally {
    loadingRepos.value = false
  }
}

async function openDialog() {
  targetRepoId.value = ''
  summaryByRepoId.value = {}
  summaryLoadingByRepoId.value = {}
  mergeTask.value = null
  stopMergeTaskPolling()
  dialogVisible.value = true
  await fetchRepos()

  if (sourceRepo.value?.id) {
    await fetchRepoStorageSummary(sourceRepo.value.id)
  }
}

function closeDialog() {
  stopMergeTaskPolling()
  dialogVisible.value = false
}

async function submitMergeRequest() {
  if (!sourceRepo.value || !targetRepo.value) {
    ElMessage.warning('请先选择目标仓库')
    return
  }

  submitting.value = true
  try {
    const payload = {
      source_repo_id: Number(sourceRepo.value.id),
      target_repo_id: Number(targetRepo.value.id)
    }

    const res = await fetch('/api/repos/merge-transfer', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })

    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '迁移与合并请求失败'))
    }

    const data = await res.json()
    if (!data?.task_id) {
      throw new Error('后端未返回任务ID')
    }

    mergeTask.value = data
    startMergeTaskPolling(data.task_id)
    ElMessage.success('迁移任务已启动')
  } catch (e) {
    ElMessage.error(e.message || '迁移与合并请求失败')
  } finally {
    submitting.value = false
  }
}

watch(
  () => targetRepoId.value,
  (next) => {
    if (!dialogVisible.value) return
    if (!next) return
    fetchRepoStorageSummary(Number(next))
  }
)

watch(
  () => dialogVisible.value,
  (visible) => {
    if (!visible) {
      stopMergeTaskPolling()
    }
  }
)

onUnmounted(() => {
  stopMergeTaskPolling()
})
</script>

<style scoped>
.merge-dialog-content {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 88px minmax(0, 1fr);
  gap: 16px;
  align-items: stretch;
  min-height: 280px;
}

.repo-panel {
  border: 1px solid #cbd5e1;
  border-radius: 10px;
  background: #f8fafc;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.panel-title {
  font-size: 14px;
  font-weight: 700;
  color: #1e293b;
}

.panel-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: #334155;
  font-size: 14px;
  word-break: break-all;
}

.repo-name {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.repo-meta-row {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr);
  gap: 8px;
}

.meta-label {
  color: #64748b;
  font-weight: 600;
}

.meta-warning {
  color: #b45309;
  margin-left: 6px;
}

.panel-empty {
  color: #64748b;
  font-size: 14px;
  padding: 12px 0;
}

.arrow-panel {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed #93c5fd;
  border-radius: 10px;
  background: #eff6ff;
}

.merge-arrow {
  font-size: 40px;
  color: #2563eb;
}

.target-select {
  width: 100%;
}

.merge-dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.merge-policy-tip {
  margin-top: 14px;
  padding: 12px 14px;
  border-radius: 10px;
  font-size: 14px;
  line-height: 1.6;
}

.merge-policy-tip-safe {
  border: 1px solid #86efac;
  background: #f0fdf4;
  color: #166534;
}

.merge-policy-tip-danger {
  border: 1px solid #fdba74;
  background: #fff7ed;
  color: #9a3412;
}

.merge-task-progress {
  margin-top: 14px;
  padding: 12px;
  border: 1px solid #cbd5e1;
  border-radius: 10px;
  background: #ffffff;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.progress-title {
  font-size: 14px;
  font-weight: 700;
  color: #0f172a;
}

.progress-task-id {
  margin-left: 8px;
  color: #64748b;
  font-weight: 500;
}

.progress-meta-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
}

.progress-label {
  color: #64748b;
  min-width: 60px;
}

.progress-value {
  color: #334155;
  word-break: break-all;
}

@media (max-width: 900px) {
  .merge-dialog-content {
    grid-template-columns: minmax(0, 1fr);
  }

  .arrow-panel {
    min-height: 72px;
  }

  .merge-arrow {
    transform: rotate(90deg);
  }
}
</style>

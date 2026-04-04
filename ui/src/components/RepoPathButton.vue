<template>
  <div>
    <el-button type="success" size="small" @click="openDialog">
      {{ pathButtonText }}
    </el-button>

    <el-dialog v-model="dialogVisible" title="仓库路径与扫描" width="760px">
      <div class="flex flex-col gap-4">
        <div v-if="isBasicRepo" class="basic-repo-tip">
          <div class="basic-repo-tip-line">
            <el-icon class="basic-repo-tip-icon"><WarningFilled /></el-icon>
            <span>基础仓库的仓库根路径为系统存储根目录，所以</span>
            <span class="basic-repo-tip-danger">不会自动扫描</span>
            <span>。</span>
          </div>
          <div class="basic-repo-tip-line">
            <el-icon class="basic-repo-tip-icon"><WarningFilled /></el-icon>
            <span>所有镜像需要</span>
            <span class="basic-repo-tip-success">手工添加</span>
            <span>。</span>
          </div>
        </div>
        <div class="repo-info-grid">
          <div class="repo-info-item">
            <span class="repo-info-label">仓库位置类型</span>
            <span class="repo-info-value">{{ repoTypeLabel }}</span>
          </div>
          <div class="repo-info-item">
            <span class="repo-info-label">外部存储</span>
            <span class="repo-info-value">{{ isInternal ? '（内部仓库）' : (externalDeviceName || '（未绑定）') }}</span>
          </div>
          <div class="repo-info-item repo-info-item-wide">
            <span class="repo-info-label">仓库路径</span>
            <span class="repo-info-value">{{ displayRepoPath }}</span>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button v-if="!isBasicRepo" class="footer-refresh-btn" type="info" plain :loading="normalizingIncremental" :disabled="isBusy" @click="refreshRepo">
            刷新
          </el-button>
          <el-button :disabled="isBusy" @click="closeDialog">不做任何事</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { WarningFilled } from '@element-plus/icons-vue'
import emitter from '../eventBus'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  }
})

const emit = defineEmits(['normalized'])

const dialogVisible = ref(false)
const currentRepoPath = ref('加载中...')
const isInternal = ref(true)
const externalDeviceName = ref('')
const isBasicRepo = ref(false)
const loadingRepoInfo = ref(false)
const normalizingIncremental = ref(false)

const isBusy = computed(() => normalizingIncremental.value || loadingRepoInfo.value)
const repoTypeLabel = computed(() => (isInternal.value ? '内部' : '外部'))
const displayRepoPath = computed(() => {
  if (isBasicRepo.value) {
    return '/'
  }
  return currentRepoPath.value || '（未设置）'
})
const pathButtonText = computed(() => {
  if (isBasicRepo.value) {
    return '仓库路径：基础仓库'
  }
  return `仓库路径：${currentRepoPath.value || '（未设置）'} （${repoTypeLabel.value}）`
})

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

async function fetchRepoInfo() {
  if (!props.repoId) {
    currentRepoPath.value = ''
    isInternal.value = true
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
      currentRepoPath.value = '（仓库不存在）'
      isInternal.value = true
      externalDeviceName.value = ''
      isBasicRepo.value = false
      return
    }

    currentRepoPath.value = repo.root_path || ''
    isInternal.value = repo.is_internal !== false
    externalDeviceName.value = repo.external_device_name || ''
    isBasicRepo.value = !!repo.basic
  } catch (e) {
    console.error('[RepoPathButton] fetchRepoInfo failed', e)
    ElMessage.error(e.message || '获取仓库信息失败')
  }
}

async function openDialog() {
  loadingRepoInfo.value = true
  await fetchRepoInfo()
  loadingRepoInfo.value = false
  dialogVisible.value = true
}

function closeDialog() {
  dialogVisible.value = false
}

async function refreshRepo() {
  if (isBasicRepo.value) {
    ElMessage.info('基础仓库没有仓库根路径，所有镜像需要手工添加')
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
    emit('normalized')
    ElMessage.success('已触发刷新')
    dialogVisible.value = false
  } catch (e) {
    console.error('[RepoPathButton] refreshRepo failed', e)
    ElMessage.error(e.message || '触发刷新失败')
  } finally {
    normalizingIncremental.value = false
  }
}

onMounted(() => {
  emitter.on('refresh-all', fetchRepoInfo)
})

onUnmounted(() => {
  emitter.off('refresh-all', fetchRepoInfo)
})

watch(
  () => props.repoId,
  () => {
    dialogVisible.value = false
    fetchRepoInfo()
  },
  { immediate: true }
)
</script>

<style scoped>
.repo-info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.repo-info-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.repo-info-item-wide {
  grid-column: 1 / -1;
}

.repo-info-label {
  font-size: 13px;
  color: #64748b;
}

.repo-info-value {
  font-size: 14px;
  line-height: 1.6;
  color: #0f172a;
  word-break: break-all;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  width: 100%;
}

.footer-refresh-btn {
  margin-right: auto;
}

.basic-repo-tip {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #eff6ff;
}

.basic-repo-tip-line {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  line-height: 1.7;
  color: #334155;
}

.basic-repo-tip-icon {
  color: #f59e0b;
  font-size: 14px;
  flex-shrink: 0;
}

.basic-repo-tip-danger {
  color: #dc2626;
  font-weight: 700;
}

.basic-repo-tip-success {
  color: #16a34a;
  font-weight: 700;
}
</style>

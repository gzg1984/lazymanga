<template>
  <div>
    <el-button type="success" size="small" @click="openDialog">
      {{ pathButtonText }}
    </el-button>

    <el-dialog v-model="dialogVisible" title="挂载路径" width="760px">
      <div class="repo-path-layout">
        <section class="repo-path-panel repo-path-info-panel">
          <div class="repo-panel-header">
            <div>
              <div class="repo-panel-eyebrow">固定信息</div>
              <h3 class="repo-panel-title">仓库配置</h3>
            </div>
          </div>

          <div v-if="isBasicRepo" class="basic-repo-tip">
            <div class="basic-repo-tip-line">
              <el-icon class="basic-repo-tip-icon"><WarningFilled /></el-icon>
              <span>基础漫画仓库的仓库根路径为系统存储根目录，所以</span>
              <span class="basic-repo-tip-danger">不会自动扫描</span>
              <span>。</span>
            </div>
            <div class="basic-repo-tip-line">
              <el-icon class="basic-repo-tip-icon"><WarningFilled /></el-icon>
              <span>所有元素需要</span>
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
        </section>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button
            type="danger"
            :loading="deletingRepo"
            :disabled="isBusy || isBasicRepo"
            @click="deleteRepo"
          >
            卸载仓库（不会删除任何实际内容）
          </el-button>
          <el-button :disabled="isBusy" @click="closeDialog">不做任何事</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { WarningFilled } from '@element-plus/icons-vue'
import emitter from '../eventBus'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  }
})

const emit = defineEmits(['deleted'])

const dialogVisible = ref(false)
const currentRepoPath = ref('加载中...')
const isInternal = ref(true)
const externalDeviceName = ref('')
const isBasicRepo = ref(false)
const loadingRepoInfo = ref(false)
const deletingRepo = ref(false)

const isBusy = computed(() => loadingRepoInfo.value || deletingRepo.value)
const repoTypeLabel = computed(() => (isInternal.value ? '内部' : '外部'))
const displayRepoPath = computed(() => {
  if (isBasicRepo.value) {
    return '/'
  }
  return currentRepoPath.value || '（未设置）'
})
const pathButtonText = computed(() => {
  if (isBasicRepo.value) {
    return '挂载：基础漫画仓库'
  }
  return `挂载：${currentRepoPath.value || '（未设置）'} （${repoTypeLabel.value}）`
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

async function deleteRepo() {
  if (isBasicRepo.value) {
    ElMessage.warning('基础漫画仓库不允许卸载')
    return
  }

  try {
    await ElMessageBox.confirm(
      '确认卸载这个仓库记录吗？不会删除任何实际内容。',
      '确认卸载仓库',
      {
        type: 'warning',
        confirmButtonText: '卸载仓库',
        cancelButtonText: '取消'
      }
    )
  } catch (_) {
    return
  }

  deletingRepo.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}`, { method: 'DELETE' })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '卸载仓库失败'))
    }

    dialogVisible.value = false
    emitter.emit('refresh-all')
    emit('deleted')
    ElMessage.success('仓库已卸载')
  } catch (e) {
    console.error('[RepoPathButton] deleteRepo failed', e)
    ElMessage.error(e.message || '卸载仓库失败')
  } finally {
    deletingRepo.value = false
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
.repo-path-layout {
  display: flex;
}

.repo-path-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
  padding: 18px;
  border: 1px solid #dbe4ee;
  border-radius: 14px;
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
}

.repo-path-info-panel {
  flex: 1 1 auto;
}

.repo-panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.repo-panel-eyebrow {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #64748b;
}

.repo-panel-title {
  margin: 4px 0 0;
  font-size: 20px;
  line-height: 1.2;
  color: #0f172a;
}

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
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  width: 100%;
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

@media (max-width: 640px) {
  .repo-info-grid {
    grid-template-columns: 1fr;
  }
}
</style>

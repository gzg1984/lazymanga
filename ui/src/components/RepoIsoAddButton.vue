<template>
  <div v-if="showAddButton" class="repo-iso-add-wrap">
    <div class="repo-iso-add">
      <el-button v-if="addFileButtonEnabled" type="primary" size="small" @click="openDialog('file')" :disabled="loadingRepoInfo">添加文件</el-button>
      <el-button v-if="addDirectoryButtonEnabled" type="success" size="small" @click="openDialog('directory')" :disabled="loadingRepoInfo">添加目录</el-button>
    </div>

    <div v-if="directoryAddCautionVisible" class="directory-add-caution">
      ⚠️ {{ directoryAddCautionMessage }}
    </div>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="640px">
      <div class="dialog-hint">当前浏览：{{ currentDir || '/' }}</div>
      <div v-if="scanSpecSummaryVisible" class="dialog-scan-summary">
        <div class="dialog-scan-summary-line">
          <span class="dialog-scan-summary-label">允许文件后缀</span>
          <span class="dialog-scan-summary-value">{{ allowedFileExtensionsLabel }}</span>
        </div>
        <div v-if="directoryRuleSummaryLabel" class="dialog-scan-summary-line">
          <span class="dialog-scan-summary-label">目录识别规则</span>
          <span class="dialog-scan-summary-value">{{ directoryRuleSummaryLabel }}</span>
        </div>
      </div>
      <div v-if="addMode === 'directory'" class="dialog-top-actions">
        <el-button type="primary" :disabled="!canAddCurrentDirectory" @click="submitCurrentDirectory">添加当前目录</el-button>
      </div>
      <el-table v-if="fileList.length" :data="fileList" style="width: 100%" @row-click="handleRowClick">
        <el-table-column prop="name" label="名称">
          <template #default="scope">
            <span v-if="scope.row.isDir" class="dir-entry">📁 {{ scope.row.name }}</span>
            <span v-else>{{ scope.row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="size" label="大小" />
      </el-table>
      <div v-else>暂无{{ addMode === 'directory' ? '目录' : '文件' }}</div>
      <template #footer>
        <div class="dialog-footer-row">
          <div class="dialog-footer-tip">点击目录可继续进入浏览；点击 `..` 可返回上级。</div>
          <div class="dialog-footer-actions">
            <el-button v-if="addMode === 'directory'" type="primary" :disabled="!canAddCurrentDirectory" @click="submitCurrentDirectory">添加当前目录</el-button>
            <el-button @click="dialogVisible = false">关闭</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import emitter from '../eventBus'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  }
})

const loadingRepoInfo = ref(false)
const addFileButtonEnabled = ref(false)
const addDirectoryButtonEnabled = ref(false)
const directoryAddCautionVisible = ref(false)
const directoryAddCautionMessage = ref('本仓库已设定根目录，直接添加目录可能导致数据迁移出错，请谨慎使用。')
const lastWarnedRepoId = ref(0)
const dialogVisible = ref(false)
const fileList = ref([])
const currentDir = ref('')
const addMode = ref('file')
const scanSpec = ref({
  extensions: [],
  include_files_without_ext: false,
  directory_rules: []
})

const showAddButton = computed(() => !!props.repoId && (addFileButtonEnabled.value || addDirectoryButtonEnabled.value))
const dialogTitle = computed(() => (addMode.value === 'directory' ? '选择目录' : '选择文件'))
const canAddCurrentDirectory = computed(() => addMode.value === 'directory' && String(currentDir.value || '').trim() !== '')
const normalizedAllowedExtensions = computed(() => {
  const values = Array.isArray(scanSpec.value?.extensions) ? scanSpec.value.extensions : []
  const normalized = values
    .map((item) => String(item || '').trim())
    .filter((item) => item !== '')
  if (scanSpec.value?.include_files_without_ext) {
    normalized.push('无后缀文件')
  }
  return normalized
})
const allowedFileExtensionsLabel = computed(() => {
  if (normalizedAllowedExtensions.value.length === 0) {
    return '.iso'
  }
  if (normalizedAllowedExtensions.value.includes('*')) {
    return '任意文件'
  }
  return normalizedAllowedExtensions.value.join(' / ')
})
const directoryRuleSummaryLabel = computed(() => {
  const rules = Array.isArray(scanSpec.value?.directory_rules) ? scanSpec.value.directory_rules : []
  const labels = rules
    .map((rule) => {
      const exts = Array.isArray(rule?.extensions) ? rule.extensions.filter(Boolean).join(' / ') : ''
      const count = Number(rule?.min_file_count || 0)
      if (!exts) {
        return ''
      }
      return count > 0 ? `${exts}，至少 ${count} 个文件` : exts
    })
    .filter((item) => item !== '')
  return labels.join('；')
})
const scanSpecSummaryVisible = computed(() => {
  return normalizedAllowedExtensions.value.length > 0 || directoryRuleSummaryLabel.value !== ''
})

async function parseErrorMessage(res, fallback) {
  try {
    const data = await res.clone().json()
    if (data && data.error) {
      return `${fallback}: ${data.error}`
    }
  } catch (_) {
    // not json
  }

  try {
    const text = await res.text()
    if (text) {
      return `${fallback}: ${text}`
    }
  } catch (_) {
    // ignore
  }

  return `${fallback} (HTTP ${res.status})`
}

async function fetchRepoInfo() {
  if (!props.repoId) {
    addFileButtonEnabled.value = false
    addDirectoryButtonEnabled.value = false
    directoryAddCautionVisible.value = false
    scanSpec.value = { extensions: [], include_files_without_ext: false, directory_rules: [] }
    lastWarnedRepoId.value = 0
    return
  }

  loadingRepoInfo.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/type-settings`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库类型设置失败'))
    }

    const data = await res.json()
    const effective = data?.effective || {}
    addFileButtonEnabled.value = !!effective?.add_button
    addDirectoryButtonEnabled.value = !!effective?.add_directory_button
    scanSpec.value = data?.scan_spec || { extensions: [], include_files_without_ext: false, directory_rules: [] }

    const cautionMessage = String(data?.directory_add_caution_message || '').trim()
    directoryAddCautionVisible.value = !!data?.directory_add_caution && !!effective?.add_directory_button
    directoryAddCautionMessage.value = cautionMessage || '本仓库已设定根目录，直接添加目录可能导致数据迁移出错，请谨慎使用。'

    if (directoryAddCautionVisible.value && lastWarnedRepoId.value !== Number(props.repoId)) {
      lastWarnedRepoId.value = Number(props.repoId)
      ElMessage.warning(directoryAddCautionMessage.value)
    }
    if (!directoryAddCautionVisible.value) {
      lastWarnedRepoId.value = 0
    }
  } catch (e) {
    addFileButtonEnabled.value = false
    addDirectoryButtonEnabled.value = false
    directoryAddCautionVisible.value = false
    scanSpec.value = { extensions: [], include_files_without_ext: false, directory_rules: [] }
    console.error('[RepoIsoAddButton] fetchRepoInfo failed', e)
  } finally {
    loadingRepoInfo.value = false
  }
}

function openDialog(mode = 'file') {
  addMode.value = mode === 'directory' ? 'directory' : 'file'
  if (addMode.value === 'directory' && directoryAddCautionVisible.value) {
    ElMessage.warning(directoryAddCautionMessage.value)
  }
  dialogVisible.value = true
  currentDir.value = ''
  fetchFiles('')
}

function parentDirOf(dir) {
  const normalized = String(dir || '').split('/').filter(Boolean)
  normalized.pop()
  return normalized.join('/')
}

async function fetchFiles(dir = '') {
  let url = '/api/files'
  const params = new URLSearchParams()
  if (props.repoId) {
    params.set('repo_id', String(props.repoId))
  }
  if (dir) {
    params.set('dir', dir)
  }
  const query = params.toString()
  if (query) {
    url += `?${query}`
  }

  try {
    const res = await fetch(url)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取文件列表失败'))
    }

    const data = await res.json()
    let entries = Array.isArray(data) ? data : []
    if (addMode.value === 'directory') {
      entries = entries.filter((item) => !!item?.isDir)
    }
    if (dir) {
      entries.unshift({ name: '..', size: 0, isDir: true, isParentDir: true })
    }
    fileList.value = entries
    if (fileList.value.length === 0) {
      ElMessage.info(`没有可用的${addMode.value === 'directory' ? '目录' : '文件'}`)
    }
  } catch (e) {
    fileList.value = []
    ElMessage.error(e.message || '获取文件列表失败')
  }
}

async function submitAddPath(fullPath, pathKind = 'file') {
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: fullPath, path_kind: pathKind })
    })

    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '添加失败'))
    }

    ElMessage.success(pathKind === 'directory' ? '已添加目录并完成扫描' : '已添加到仓库列表')
    dialogVisible.value = false
    emitter.emit('refresh-repo', { repoId: props.repoId })
  } catch (e) {
    ElMessage.error(e.message || '添加失败')
  }
}

function submitCurrentDirectory() {
  if (!canAddCurrentDirectory.value) return
  submitAddPath(currentDir.value, 'directory')
}

async function handleRowClick(row) {
  if (row.isParentDir) {
    currentDir.value = parentDirOf(currentDir.value)
    fetchFiles(currentDir.value)
    return
  }

  if (row.isDir) {
    currentDir.value = currentDir.value ? `${currentDir.value}/${row.name}` : row.name
    fetchFiles(currentDir.value)
    return
  }

  if (addMode.value === 'directory') {
    return
  }

  const fullPath = currentDir.value ? `${currentDir.value}/${row.name}` : row.name
  submitAddPath(fullPath, 'file')
}

onMounted(() => {
  fetchRepoInfo()
  emitter.on('refresh-all', fetchRepoInfo)
})

onUnmounted(() => {
  emitter.off('refresh-all', fetchRepoInfo)
})

watch(
  () => props.repoId,
  () => {
    dialogVisible.value = false
    fileList.value = []
    currentDir.value = ''
    fetchRepoInfo()
  },
  { immediate: true }
)
</script>

<style scoped>
.repo-iso-add-wrap {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.repo-iso-add {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.directory-add-caution {
  max-width: 560px;
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid #fed7aa;
  background: #fff7ed;
  color: #b45309;
  font-size: 12px;
  line-height: 1.5;
}

.dir-entry {
  color: #409eff;
  cursor: pointer;
}

.dialog-hint {
  margin-bottom: 10px;
  color: #64748b;
  font-size: 12px;
}

.dialog-scan-summary {
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid #dbe4ee;
  background: #f8fbff;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dialog-scan-summary-line {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  font-size: 12px;
  line-height: 1.5;
}

.dialog-scan-summary-label {
  color: #475569;
  font-weight: 700;
  white-space: nowrap;
}

.dialog-scan-summary-value {
  color: #0f172a;
  word-break: break-word;
}

.dialog-top-actions {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 10px;
}

.dialog-footer-row {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dialog-footer-tip {
  color: #64748b;
  font-size: 12px;
}

.dialog-footer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>

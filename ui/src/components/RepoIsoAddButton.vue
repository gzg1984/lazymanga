<template>
  <div v-if="showAddButton" class="repo-iso-add">
    <el-button v-if="addFileButtonEnabled" type="primary" size="small" @click="openDialog('file')" :disabled="loadingRepoInfo">添加文件</el-button>
    <el-button v-if="addDirectoryButtonEnabled" type="success" size="small" @click="openDialog('directory')" :disabled="loadingRepoInfo">添加目录</el-button>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="640px">
      <div class="dialog-hint">当前浏览：{{ currentDir || '/' }}</div>
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
const dialogVisible = ref(false)
const fileList = ref([])
const currentDir = ref('')
const addMode = ref('file')

const showAddButton = computed(() => !!props.repoId && (addFileButtonEnabled.value || addDirectoryButtonEnabled.value))
const dialogTitle = computed(() => (addMode.value === 'directory' ? '选择目录' : '选择 ISO 文件'))
const canAddCurrentDirectory = computed(() => addMode.value === 'directory' && String(currentDir.value || '').trim() !== '')

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
  } catch (e) {
    addFileButtonEnabled.value = false
    addDirectoryButtonEnabled.value = false
    console.error('[RepoIsoAddButton] fetchRepoInfo failed', e)
  } finally {
    loadingRepoInfo.value = false
  }
}

function openDialog(mode = 'file') {
  addMode.value = mode === 'directory' ? 'directory' : 'file'
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
  if (dir) {
    url += `?dir=${encodeURIComponent(dir)}`
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

    ElMessage.success(pathKind === 'directory' ? '已添加目录到仓库列表' : '已添加到仓库列表')
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
.repo-iso-add {
  display: inline-flex;
  align-items: center;
  gap: 8px;
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

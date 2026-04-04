<template>
  <div v-if="showAddButton" class="repo-iso-add">
    <el-button type="primary" size="small" @click="openDialog" :disabled="loadingRepoInfo">添加</el-button>
    <el-dialog v-model="dialogVisible" title="选择 ISO 文件" width="600px">
      <el-table v-if="fileList.length" :data="fileList" style="width: 100%" @row-click="handleRowClick">
        <el-table-column prop="name" label="文件名">
          <template #default="scope">
            <span v-if="scope.row.isDir" style="color: #409EFF; cursor: pointer;">📁 {{ scope.row.name }}</span>
            <span v-else>{{ scope.row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="size" label="大小" />
      </el-table>
      <div v-else>暂无文件</div>
      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
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
const addButtonEnabled = ref(false)
const dialogVisible = ref(false)
const fileList = ref([])
const currentDir = ref('')

const showAddButton = computed(() => !!props.repoId && addButtonEnabled.value)

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
    addButtonEnabled.value = false
    return
  }

  loadingRepoInfo.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repo-info`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取 repo info 失败'))
    }

    const data = await res.json()
    addButtonEnabled.value = !!data?.add_button
  } catch (e) {
    addButtonEnabled.value = false
    console.error('[RepoIsoAddButton] fetchRepoInfo failed', e)
  } finally {
    loadingRepoInfo.value = false
  }
}

function openDialog() {
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
    const entries = Array.isArray(data) ? data : []
    if (dir) {
      entries.unshift({ name: '..', size: 0, isDir: true, isParentDir: true })
    }
    fileList.value = entries
    if (fileList.value.length === 0) {
      ElMessage.info('没有可用的文件')
    }
  } catch (e) {
    fileList.value = []
    ElMessage.error(e.message || '获取文件列表失败')
  }
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

  const fullPath = currentDir.value ? `${currentDir.value}/${row.name}` : row.name
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: fullPath })
    })

    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '添加失败'))
    }

    ElMessage.success('已添加到仓库ISO列表')
    dialogVisible.value = false
    emitter.emit('refresh-repo', { repoId: props.repoId })
  } catch (e) {
    ElMessage.error(e.message || '添加失败')
  }
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
}
</style>

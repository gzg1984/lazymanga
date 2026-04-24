<template>
  <div class="repo-tabs flex items-center gap-2">
    <el-tabs v-model="activeId" @tab-click="handleTabClick">
      <el-tab-pane v-for="r in repos" :key="r.id" :label="r.name" :name="String(r.id)"></el-tab-pane>
    </el-tabs>
    <el-button class="repo-add-btn" icon="el-icon-plus" @click="openAdd">+ 仓库</el-button>

    <el-dialog v-model="addDialogVisible" title="新增仓库" width="820px">
      <div class="add-repo-content">
        <div class="form-row">
          <span class="form-label">仓库名字</span>
          <div class="form-main-col name-row">
            <el-input
              v-model="createForm.name"
              placeholder="请输入仓库名字"
              :disabled="addRepoBusy"
              @input="onNameInputChanged"
            />
            <el-button :disabled="addRepoBusy" @click="resetNameToSuggested">重新生成名称</el-button>
          </div>
        </div>

        <div class="form-row type-row">
          <span class="form-label">仓库类型</span>
          <div class="form-main-col">
            <div v-if="loadingRepoTypes" class="muted">加载仓库类型中...</div>
            <template v-else-if="visibleRepoTypeOptions.length">
              <el-radio-group v-model="createForm.repoType" :disabled="addRepoBusy">
                <el-radio-button v-for="item in visibleRepoTypeOptions" :key="item.key" :label="item.key">{{ item.name }}</el-radio-button>
              </el-radio-group>
              <div v-if="selectedRepoTypeDescription" class="type-help">{{ selectedRepoTypeDescription }}</div>
            </template>
            <div v-else class="muted">暂无可用仓库类型，请先通过顶部入口创建模板。</div>
          </div>
        </div>

        <div class="form-row type-row">
          <span class="form-label">仓库来源</span>
          <el-switch
            v-model="createForm.isInternal"
            inline-prompt
            active-text="内部"
            inactive-text="外部"
            :disabled="addRepoBusy"
            @change="onCreateTypeChanged"
          />
        </div>

        <div v-if="!createForm.isInternal" class="form-row external-row">
          <span class="form-label">外部存储</span>
          <div class="form-main-col">
            <div v-if="loadingExternalDevices" class="muted">加载外部存储中...</div>
            <el-radio-group
              v-else-if="externalDevices.length"
              v-model="createForm.externalDeviceName"
              :disabled="addRepoBusy"
              @change="onCreateExternalDeviceChanged"
            >
              <el-radio v-for="device in externalDevices" :key="device.name" :label="device.name">{{ device.name }}</el-radio>
            </el-radio-group>
            <div v-else class="muted">暂无可用外部存储目录</div>
          </div>
        </div>

        <div class="path-browser-wrap">
          <div class="path-actions">
            <el-button type="primary" plain :disabled="addRepoBusy" @click="selectCurrentPath">选择当前浏览目录</el-button>
            <el-button :disabled="addRepoBusy || browseDir === ''" @click="goParentDir">返回上级</el-button>
            <span class="path-current">当前浏览：{{ browseDir || '/' }}</span>
          </div>

          <div class="folder-table-wrap">
            <div class="folder-table-title">可选文件夹</div>
            <el-table
              v-if="folderList.length"
              :data="folderList"
              style="width: 100%"
              height="220"
              v-loading="loadingFolders"
              @row-click="handleFolderClick"
            >
              <el-table-column prop="name" label="文件夹名">
                <template #default="scope">
                  <span class="folder-cell">📁 {{ scope.row.name }}</span>
                </template>
              </el-table-column>
            </el-table>
            <div v-else class="muted empty-box">{{ loadingFolders ? '加载中...' : '暂无可选文件夹' }}</div>
          </div>

          <div class="path-input-row">
            <label class="path-input-label" for="create-repo-path">仓库相对路径</label>
            <el-input
              id="create-repo-path"
              v-model="createForm.rootPath"
              placeholder="输入相对路径；如需存储根目录请手动输入 /"
              :disabled="addRepoBusy"
              @input="onPathInputChanged"
            />
          </div>
        </div>

      </div>

      <template #footer>
        <el-button :disabled="addRepoBusy" @click="closeAddDialog">取消</el-button>
        <el-button type="primary" :loading="creatingRepo" :disabled="!canCreateRepo" @click="submitCreateRepo">创建仓库</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import emitter from '../eventBus'
import { filterVisibleRepoTypes } from '../utils/repoTypeVisibility'

const props = defineProps({
  modelValue: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:modelValue', 'tab-reselect'])
const repos = ref([])
const activeId = ref(props.modelValue || '')
const addDialogVisible = ref(false)
const loadingExternalDevices = ref(false)
const loadingFolders = ref(false)
const loadingRepoTypes = ref(false)
const creatingRepo = ref(false)
const externalDevices = ref([])
const folderList = ref([])
const repoTypeOptions = ref([])
const browseDir = ref('')
const createForm = ref({
  repoType: '',
  isInternal: true,
  externalDeviceName: '',
  rootPath: '',
  name: ''
})
const nameManuallyEdited = ref(false)

const browseRepoId = computed(() => {
  const first = Array.isArray(repos.value) && repos.value.length > 0 ? repos.value[0] : null
  return first?.id ? Number(first.id) : 0
})

const addRepoBusy = computed(() => {
  return creatingRepo.value || loadingExternalDevices.value || loadingFolders.value || loadingRepoTypes.value
})

const selectedRepoTypeOption = computed(() => {
  return repoTypeOptions.value.find((item) => item.key === createForm.value.repoType) || null
})

const visibleRepoTypeOptions = computed(() => {
  return filterVisibleRepoTypes(repoTypeOptions.value).filter((item) => item.enabled !== false)
})

const selectedRepoTypeDescription = computed(() => {
  return String(selectedRepoTypeOption.value?.description || '').trim()
})

const canCreateRepo = computed(() => {
  const name = String(createForm.value.name || '').trim()
  const rootPath = String(createForm.value.rootPath || '').trim()
  if (!name || !rootPath) return false
  if (!createForm.value.isInternal && !String(createForm.value.externalDeviceName || '').trim()) return false
  return true
})

watch(
  () => props.modelValue,
  (v) => {
    const nextValue = v || ''
    if (nextValue !== activeId.value) {
      activeId.value = nextValue
    }
  }
)

watch(activeId, (v) => {
  emit('update:modelValue', v)
})

function handleTabClick(tabPane) {
  const tabName = String(tabPane?.props?.name || '')
  if (!tabName) {
    return
  }

  if (tabName === String(activeId.value || '')) {
    emit('tab-reselect', { repoId: Number(tabName) || 0 })
  }
}

watch(
  () => createForm.value.repoType,
  () => {
    syncNameIfAuto()
  }
)

async function parseErrorMessage(res, fallback) {
  try {
    const data = await res.clone().json()
    if (data && data.error) {
      return `${fallback}: ${data.error}`
    }
  } catch (_) {
    // not json, fallback to plain text below
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
  try {
    console.log('[RepoTabs] fetchRepos: start')
    const res = await fetch('/api/repos')
    console.log('[RepoTabs] fetchRepos: response', { status: res.status, ok: res.ok, url: res.url })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库列表失败'))
    }
    const data = await res.json()
    console.log('[RepoTabs] fetchRepos: data', data)
    repos.value = Array.isArray(data) ? data : []

    const hasCurrent = repos.value.some((r) => String(r.id) === activeId.value)
    if (!hasCurrent) {
      activeId.value = repos.value.length > 0 ? String(repos.value[0].id) : ''
    }
  } catch (e) {
    console.error('[RepoTabs] fetchRepos: failed', e)
    ElMessage.error(e.message || '获取仓库列表失败')
  }
}

function getDefaultRepoTypeKey(items = visibleRepoTypeOptions.value) {
  if (!Array.isArray(items) || items.length === 0) {
    return ''
  }
  const fallback = items.find((item) => item.enabled !== false)
  return fallback?.key || items[0]?.key || ''
}

async function fetchRepoTypes() {
  loadingRepoTypes.value = true
  try {
    const res = await fetch('/api/repo-types')
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库类型失败'))
    }

    const data = await res.json()
    repoTypeOptions.value = Array.isArray(data?.items) ? data.items : []
    const hasSelected = visibleRepoTypeOptions.value.some((item) => item.key === createForm.value.repoType)
    if (!hasSelected) {
      createForm.value.repoType = getDefaultRepoTypeKey(visibleRepoTypeOptions.value)
    }
    syncNameIfAuto()
  } catch (e) {
    repoTypeOptions.value = []
    ElMessage.error(e.message || '获取仓库类型失败')
  } finally {
    loadingRepoTypes.value = false
  }
}

function normalizePathInput(v) {
  return String(v || '').replace(/\\/g, '/').trim()
}

function getSuggestedPathLeaf(rawPath) {
  if (!rawPath || rawPath === '/') return ''
  const parts = rawPath.replace(/^\/+/, '').replace(/\/+$/, '').split('/').filter(Boolean)
  return parts.length ? parts[parts.length - 1] : ''
}

function deriveSuggestedName() {
  const rawPath = normalizePathInput(createForm.value.rootPath)
  const pathLeaf = getSuggestedPathLeaf(rawPath)
  if (pathLeaf) {
    return pathLeaf
  }
  if (rawPath === '/') {
    return 'root'
  }
  return selectedRepoTypeOption.value?.name || String(createForm.value.repoType || '新仓库').trim() || '新仓库'
}

function syncNameIfAuto() {
  if (nameManuallyEdited.value) return
  createForm.value.name = deriveSuggestedName()
}

function onNameInputChanged() {
  nameManuallyEdited.value = true
}

function resetNameToSuggested() {
  nameManuallyEdited.value = false
  createForm.value.name = deriveSuggestedName()
}

function onPathInputChanged() {
  syncNameIfAuto()
}

async function fetchExternalDevicesForCreate() {
  if (!browseRepoId.value) {
    externalDevices.value = []
    return
  }

  loadingExternalDevices.value = true
  try {
    const res = await fetch(`/api/repos/${browseRepoId.value}/path/external-devices`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取外部存储列表失败'))
    }
    const data = await res.json()
    externalDevices.value = Array.isArray(data?.devices) ? data.devices : []
    if (!externalDevices.value.some((item) => item.name === createForm.value.externalDeviceName)) {
      createForm.value.externalDeviceName = ''
    }
  } catch (e) {
    externalDevices.value = []
    ElMessage.error(e.message || '获取外部存储列表失败')
  } finally {
    loadingExternalDevices.value = false
  }
}

async function fetchFoldersForCreate(dir = '') {
  if (!browseRepoId.value) {
    folderList.value = []
    return
  }
  if (!createForm.value.isInternal && !createForm.value.externalDeviceName) {
    folderList.value = []
    return
  }

  loadingFolders.value = true
  try {
    const params = new URLSearchParams()
    params.set('internal', String(createForm.value.isInternal))
    if (!createForm.value.isInternal) {
      params.set('external_device_name', String(createForm.value.externalDeviceName || '').trim())
    }
    if (dir) {
      params.set('dir', dir)
    }

    const res = await fetch(`/api/repos/${browseRepoId.value}/path/options?${params.toString()}`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取可选文件夹失败'))
    }

    const data = await res.json()
    browseDir.value = data?.dir || ''
    createForm.value.rootPath = browseDir.value || '/'
    syncNameIfAuto()
    folderList.value = Array.isArray(data?.entries) ? data.entries.filter((item) => item?.isDir) : []
  } catch (e) {
    folderList.value = []
    ElMessage.error(e.message || '获取可选文件夹失败')
  } finally {
    loadingFolders.value = false
  }
}

function selectCurrentPath() {
  const selected = String(browseDir.value || '').trim()
  if (selected === '') {
    ElMessage.info('当前选择为空路径。若要使用存储根目录，请手动输入 "/"。')
    return
  }
  createForm.value.rootPath = selected
  syncNameIfAuto()
}

function goParentDir() {
  if (!browseDir.value) return
  const parts = browseDir.value.split('/').filter(Boolean)
  parts.pop()
  fetchFoldersForCreate(parts.join('/'))
}

function handleFolderClick(row) {
  if (!row?.isDir) return
  const next = browseDir.value ? `${browseDir.value}/${row.name}` : row.name
  fetchFoldersForCreate(next)
}

function onCreateExternalDeviceChanged() {
  browseDir.value = ''
  createForm.value.rootPath = ''
  syncNameIfAuto()
  fetchFoldersForCreate('')
}

async function onCreateTypeChanged() {
  browseDir.value = ''
  createForm.value.rootPath = ''
  folderList.value = []
  if (createForm.value.isInternal) {
    createForm.value.externalDeviceName = ''
    await fetchFoldersForCreate('')
  } else {
    await fetchExternalDevicesForCreate()
    if (createForm.value.externalDeviceName) {
      await fetchFoldersForCreate('')
    }
  }
  syncNameIfAuto()
}

async function openAdd() {
  addDialogVisible.value = true
  createForm.value = {
    repoType: getDefaultRepoTypeKey(visibleRepoTypeOptions.value),
    isInternal: true,
    externalDeviceName: '',
    rootPath: '',
    name: ''
  }
  nameManuallyEdited.value = false
  browseDir.value = ''
  externalDevices.value = []
  folderList.value = []
  syncNameIfAuto()

  const blockingTasks = []
  if (repos.value.length === 0) {
    blockingTasks.push(fetchRepos())
  } else {
    void fetchRepos()
  }

  if (repoTypeOptions.value.length === 0) {
    blockingTasks.push(fetchRepoTypes())
  } else {
    void fetchRepoTypes()
  }

  if (blockingTasks.length > 0) {
    await Promise.all(blockingTasks)
    const hasSelected = visibleRepoTypeOptions.value.some((item) => item.key === createForm.value.repoType)
    if (!hasSelected) {
      createForm.value.repoType = getDefaultRepoTypeKey(visibleRepoTypeOptions.value)
    }
    syncNameIfAuto()
  }

  void fetchFoldersForCreate('')
}

function closeAddDialog() {
  addDialogVisible.value = false
}

async function submitCreateRepo() {
  if (!canCreateRepo.value) {
    ElMessage.warning('请先填写完整信息')
    return
  }

  creatingRepo.value = true
  try {
    const body = {
      name: String(createForm.value.name || '').trim(),
      root_path: String(createForm.value.rootPath || '').trim(),
      db_filename: 'repo.db',
      is_internal: !!createForm.value.isInternal,
      external_device_name: createForm.value.isInternal ? '' : String(createForm.value.externalDeviceName || '').trim(),
      repo_type: createForm.value.repoType
    }

    const res = await fetch('/api/repos', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '新增仓库失败'))
    }

    const created = await res.json()
    await fetchRepos()
    const createdRepoId = Number(created?.id || 0)
    if (createdRepoId > 0) {
      activeId.value = String(createdRepoId)
      emitter.emit('repo-created-activated', { repoId: createdRepoId })
    }
    addDialogVisible.value = false
    emitter.emit('refresh-all')
    ElMessage.success('仓库已创建，正在自动扫描仓库内容...')
  } catch (e) {
    ElMessage.error(e.message || '新增仓库失败')
  } finally {
    creatingRepo.value = false
  }
}

onMounted(() => {
  fetchRepos()
  fetchRepoTypes()
  emitter.on('refresh-all', fetchRepos)
  emitter.on('repo-types-updated', fetchRepoTypes)
})

onUnmounted(() => {
  emitter.off('refresh-all', fetchRepos)
  emitter.off('repo-types-updated', fetchRepoTypes)
})
</script>

<style scoped>
.repo-tabs {
  margin-left: 12px;
  --el-tabs-header-height: 44px;
}

.repo-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.repo-tabs :deep(.el-tabs__item) {
  font-size: 16px;
  font-weight: 600;
}

.repo-add-btn {
  height: 30px;
  padding: 0 10px;
  border-radius: 9px;
  border: 1px solid #d9e2ec;
  background: rgba(255, 255, 255, 0.55);
  color: #475569;
  font-size: 14px;
  font-weight: 500;
  transition: border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;
}

.repo-add-btn:hover {
  border-color: #b8c4d3;
  background: rgba(255, 255, 255, 0.75);
  color: #334155;
}

.repo-add-btn:focus-visible {
  border-color: #64748b;
}

.repo-add-btn :deep(.el-icon) {
  font-size: 13px;
}

.add-repo-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-row {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
}

.type-row {
  align-items: center;
}

.external-row {
  align-items: flex-start;
}

.type-help {
  margin-top: 6px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.form-label {
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.form-main-col {
  min-width: 0;
}

.path-browser-wrap {
  border: 1px solid #cbd5e1;
  border-radius: 10px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.path-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.path-current {
  font-size: 12px;
  color: #64748b;
}

.folder-table-wrap {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 8px;
}

.folder-table-title {
  font-size: 13px;
  color: #334155;
  margin-bottom: 8px;
}

.folder-cell {
  color: #409eff;
  cursor: pointer;
}

.empty-box {
  padding: 18px 8px;
}

.path-input-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.path-input-label {
  font-size: 13px;
  color: #334155;
  font-weight: 600;
}

.name-row {
  display: flex;
  gap: 8px;
}

.muted {
  color: #64748b;
  font-size: 13px;
}
</style>

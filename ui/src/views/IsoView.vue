<template>
  <div class="mb-4 border-b-4 border-slate-400 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <RepoTabs v-model="activeTab" @tab-reselect="handleTabReselect" />
    </div>
    <button type="button" class="mr-4 text-sm font-semibold text-sky-700 hover:text-sky-900 hover:underline" @click="usageDialogVisible = true">
      Welcome to Lazy Bird ISO manager
    </button>
  </div>
  <div
    v-if="rulebookWarningVisible"
    class="mb-4 p-3 rounded bg-amber-100 border border-amber-500 text-amber-900"
  >
    <div class="font-semibold">规则书告警：当前使用容灾回退规则</div>
    <div class="text-sm mt-1">{{ rulebookWarningMessage }}</div>
  </div>
  <DownloadProgressPanel />
  <el-dialog v-model="usageDialogVisible" title="欢迎使用懒鸟 ISO 管理器" width="560px">
    <p class="mb-3 text-slate-700 leading-7">
       早起的鸟儿有虫吃。 <br>
       Early Bird gets the bug.  <br>
       Lazy Bird gets No Bug. <br>
    </p>
    <div class="rounded border border-slate-300 bg-slate-50 p-3 text-sm text-slate-600">
      基础仓库不会进行自动扫描和自动归类。<br>
      新建仓库，只要设置为无类型，也不会自动归类。<br>
      新建仓库，设置为 <span style="color: red;">操作系统镜像库</span> 时，会自动扫描并归类为对应的操作系统类型。<br>
    </div>
    <template #footer>
      <el-button @click="usageDialogVisible = false">关闭</el-button>
    </template>
  </el-dialog>
  <el-dialog
    v-model="upgradeMigrationDialogVisible"
    title="版本升级数据迁移说明"
    width="560px"
    :close-on-click-modal="false"
  >
    <div class="text-slate-700 leading-7 mb-3">
      检测到你从旧版本升级后，系统已自动将旧版基础 ISO 数据迁移到新版基础仓库。
    </div>
    <div class="rounded border border-sky-200 bg-sky-50 p-3 text-sm text-slate-700">
      <div>迁移成功条数: <span class="font-semibold text-sky-700">{{ migrationNotice.migratedCount }}</span></div>
      <div>跳过条数: <span class="font-semibold text-sky-700">{{ migrationNotice.skippedCount }}</span></div>
      <div>旧数据总条数: <span class="font-semibold text-sky-700">{{ migrationNotice.totalCount }}</span></div>
      <div class="mt-1 break-all">迁移时间: {{ migrationNotice.migratedAt || '-' }}</div>
    </div>
    <template #footer>
      <el-button type="primary" @click="upgradeMigrationDialogVisible = false">我知道了</el-button>
    </template>
  </el-dialog>
  <template v-if="activeRepoId !== null">
    <div class="repo-actions-wrap mb-4 p-3 rounded bg-blue-100 border border-blue-400 text-blue-800 flex flex-wrap gap-2">
      <RepoIsoAddButton :repo-id="activeRepoId" />
      <RepoMergeButton :repo-id="activeRepoId" />
      <RepoPathButton :repo-id="activeRepoId" />
      <RepoSettingsButton :repo-id="activeRepoId" @deleted="onActiveRepoDeleted" />
      <div class="flex-1"></div>
      <RepoInfoButton :repo-id="activeRepoId" />
    </div>
    <RepoIsoTable :repo-id="activeRepoId" :refresh-signal="repoRefreshSignal" />
  </template>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import emitter from '../eventBus'
import RepoTabs from '../components/RepoTabs.vue'
import RepoPathButton from '../components/RepoPathButton.vue'
import RepoSettingsButton from '../components/RepoSettingsButton.vue'
import RepoMergeButton from '../components/RepoMergeButton.vue'
import RepoIsoTable from '../components/RepoIsoTable.vue'
import RepoInfoButton from '../components/RepoInfoButton.vue'
import RepoIsoAddButton from '../components/RepoIsoAddButton.vue'
import DownloadProgressPanel from '../components/DownloadProgressPanel.vue'

const route = useRoute()
const router = useRouter()
const activeTab = ref('')
const repoRefreshSignal = ref(0)
const usageDialogVisible = ref(false)
const upgradeMigrationDialogVisible = ref(false)
const migrationNotice = ref({
  migratedCount: 0,
  skippedCount: 0,
  totalCount: 0,
  migratedAt: ''
})
const rulebookStatus = ref(null)
const activeRepoId = computed(() => {
  const value = Number(activeTab.value)
  return Number.isFinite(value) && value > 0 ? value : null
})

const rulebookWarningVisible = computed(() => !!rulebookStatus.value?.using_fallback)
const rulebookWarningMessage = computed(() => {
  const status = rulebookStatus.value || {}
  const filePath = status.file_path || '-'
  const err = status.last_error || 'unknown'
  const updatedAt = status.updated_at ? new Date(status.updated_at).toLocaleString() : '-'
  return `规则文件: ${filePath}；回退原因: ${err}；状态更新时间: ${updatedAt}`
})

async function fetchRuleBookStatus() {
  try {
    const res = await fetch('/api/rulebook/status')
    if (!res.ok) {
      return
    }
    const data = await res.json()
    rulebookStatus.value = data || null
  } catch (_) {
    // keep silent for non-critical status polling
  }
}

async function fetchUpgradeMigrationNotice() {
  try {
    const res = await fetch('/api/upgrade/migration-notice')
    if (!res.ok) {
      return
    }

    const data = await res.json()
    if (!data?.show) {
      return
    }

    migrationNotice.value = {
      migratedCount: Number(data.migrated_count || 0),
      skippedCount: Number(data.skipped_count || 0),
      totalCount: Number(data.total_legacy_count || 0),
      migratedAt: data.migrated_at || ''
    }
    upgradeMigrationDialogVisible.value = true
  } catch (_) {
    // keep silent for non-critical migration notice fetching
  }
}

function onActiveRepoDeleted() {
  // Immediately clear active tab to unmount repo-bound components,
  // avoiding transient fetches against a deleted repo id.
  activeTab.value = ''
  switchToBasicRepoTab()
}

async function switchToBasicRepoTab() {
  const basicRepoId = await resolveBasicRepoId()
  activeTab.value = basicRepoId ? String(basicRepoId) : ''
}

async function parseErrorMessage(res, fallback) {
  try {
    const data = await res.clone().json()
    if (data && data.error) {
      return `${fallback}: ${data.error}`
    }
  } catch (_) {
    // ignore json parse failures
  }

  try {
    const text = await res.text()
    if (text) {
      return `${fallback}: ${text}`
    }
  } catch (_) {
    // ignore text parse failures
  }

  return `${fallback} (HTTP ${res.status})`
}

async function resolveBasicRepoId() {
  try {
    const res = await fetch('/api/repos')
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库列表失败'))
    }

    const repos = await res.json()
    if (!Array.isArray(repos) || repos.length === 0) {
      return null
    }

    const basicRepo = repos.find((repo) => !!repo?.basic)
    return Number(basicRepo?.id || repos[0]?.id || 0) || null
  } catch (e) {
    ElMessage.error(e.message || '获取基础仓库失败')
    return null
  }
}

function triggerRepoRefresh(repoId) {
  repoRefreshSignal.value += 1
  emitter.emit('refresh-all')
  if (repoId) {
    emitter.emit('refresh-repo', { repoId })
  }
}

function handleTabReselect(payload) {
  const repoId = Number(payload?.repoId || 0)
  if (!repoId) {
    return
  }
  triggerRepoRefresh(repoId)
}

function clearOpenRefreshQuery() {
  if (!route.query || typeof route.query.open_refresh === 'undefined') {
    return
  }

  const nextQuery = { ...route.query }
  delete nextQuery.open_refresh
  delete nextQuery.open_repo_id
  router.replace({ path: route.path, query: nextQuery }).catch(() => {})
}

watch(
  () => route.query.open_refresh,
  async (tick) => {
    if (typeof tick !== 'string' || tick.trim() === '') {
      return
    }

    let repoId = Number(route.query.open_repo_id || 0)
    if (!repoId) {
      repoId = await resolveBasicRepoId()
    }

    if (repoId) {
      activeTab.value = String(repoId)
      triggerRepoRefresh(repoId)
    } else {
      triggerRepoRefresh(null)
    }

    clearOpenRefreshQuery()
  },
  { immediate: true }
)

onMounted(() => {
  fetchRuleBookStatus()
  fetchUpgradeMigrationNotice()
})
</script>

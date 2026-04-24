<template>
  <div class="open-import-page">
    <el-result :icon="resultIcon" :title="resultTitle" :sub-title="resultMessage">
      <template #extra>
        <el-button type="primary" @click="goHome">返回仓库主页</el-button>
      </template>
    </el-result>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const status = ref('processing')
const resultMessage = ref('正在处理打开请求...')

function openFlowLog(message, extra = null) {
  const prefix = `[OPEN_FLOW][ui][${new Date().toISOString()}] ${message}`
  if (extra === null) {
    console.log(prefix)
    return
  }
  console.log(prefix, extra)
}

const resultTitle = computed(() => {
  if (status.value === 'success') return '已导入基础漫画仓库'
  if (status.value === 'warning') return '文件已存在'
  if (status.value === 'error') return '导入失败'
  return '处理中'
})

const resultIcon = computed(() => {
  if (status.value === 'success') return 'success'
  if (status.value === 'warning') return 'warning'
  if (status.value === 'error') return 'error'
  return 'info'
})

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
  openFlowLog('resolve basic repo id start')
  const res = await fetch('/api/repos')
  if (!res.ok) {
    openFlowLog('resolve basic repo id failed', { status: res.status })
    throw new Error(await parseErrorMessage(res, '获取仓库列表失败'))
  }

  const repos = await res.json()
  if (!Array.isArray(repos) || repos.length === 0) {
    openFlowLog('resolve basic repo id empty list')
    return null
  }

  const basicRepo = repos.find((repo) => !!repo?.basic)
  const id = Number(basicRepo?.id || repos[0]?.id || 0) || null
  openFlowLog('resolve basic repo id success', { repoId: id })
  return id
}

function sanitizeFileInput(v) {
  return String(v || '').trim()
}

async function redirectToHome(repoId) {
  const query = { open_refresh: String(Date.now()) }
  if (repoId) {
    query.open_repo_id = String(repoId)
  }
  openFlowLog('redirect to home', { repoId, query })
  await router.replace({ path: '/', query })
}

async function processOpenRequest() {
  const file = sanitizeFileInput(route.query.file)
  openFlowLog('process open request start', {
    fullPath: route.fullPath,
    file,
    rawQuery: route.query
  })
  if (!file) {
    status.value = 'error'
    resultMessage.value = '缺少 file 参数'
    await redirectToHome(null)
    return
  }

  let basicRepoId = null
  try {
    basicRepoId = await resolveBasicRepoId()
  } catch (e) {
    openFlowLog('resolve basic repo id ignored error', { error: e?.message || String(e) })
    // ignore repo id resolve errors here and let open API decide
  }

  const params = new URLSearchParams()
  params.set('file', file)
  openFlowLog('request /api/open start', { url: `/api/open?${params.toString()}` })
  const res = await fetch(`/api/open?${params.toString()}`)
  openFlowLog('request /api/open response', { status: res.status, ok: res.ok })

  if (res.status === 409) {
    status.value = 'warning'
    resultMessage.value = '该文件已在基础漫画仓库中，已触发刷新。'
    await redirectToHome(basicRepoId)
    return
  }

  if (!res.ok) {
    throw new Error(await parseErrorMessage(res, 'open 导入失败'))
  }

  status.value = 'success'
  resultMessage.value = '文件已导入，正在返回仓库主页并刷新。'
  await redirectToHome(basicRepoId)
}

function goHome() {
  openFlowLog('manual go home')
  router.replace({ path: '/' }).catch(() => {})
}

onMounted(async () => {
  openFlowLog('OpenView mounted')
  try {
    await processOpenRequest()
  } catch (e) {
    openFlowLog('process open request failed', { error: e?.message || String(e) })
    status.value = 'error'
    resultMessage.value = e.message || 'open 导入失败'
    await redirectToHome(null)
  }
})
</script>

<style scoped>
.open-import-page {
  min-height: 60vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
</style>

<template>
  <div>
    <el-button type="warning" size="small" @click="openDialog">
      仓库名字：{{ currentRepoName || '（未命名）' }}
    </el-button>

    <el-dialog v-model="dialogVisible" title="仓库设置" width="560px">
      <div class="flex flex-col gap-3">
        <label class="text-sm text-slate-700" for="repo-name-input">仓库名字</label>
        <el-input
          id="repo-name-input"
          v-model="inputRepoName"
          placeholder="请输入仓库名字"
          clearable
          :disabled="isBusy || isBasicRepo"
        />
      </div>

      <template #footer>
        <div class="repo-settings-footer flex items-center justify-between w-full gap-2">
          <el-button type="danger" :loading="deleting" :disabled="saving || isBasicRepo" @click="deleteRepo">
            {{ deleteButtonText }}
          </el-button>
          <div class="flex items-center gap-2">
            <el-button :disabled="isBusy" @click="closeDialog">不做任何事</el-button>
            <el-button type="success" :disabled="!canModifyName || deleting" :loading="saving" @click="updateRepoName">
              {{ modifyButtonText }}
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import emitter from '../eventBus'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  }
})

const emit = defineEmits(['deleted'])

const dialogVisible = ref(false)
const currentRepoName = ref('加载中...')
const inputRepoName = ref('')
const isBasicRepo = ref(false)
const loadingRepoInfo = ref(false)
const saving = ref(false)
const deleting = ref(false)

const isBusy = computed(() => saving.value || deleting.value || loadingRepoInfo.value)
const canModifyName = computed(() => {
  const nextName = inputRepoName.value.trim()
  return !isBasicRepo.value && nextName !== '' && nextName !== currentRepoName.value
})
const modifyButtonText = computed(() => (isBasicRepo.value ? '不能修改基础仓库名字' : '修改名字'))
const deleteButtonText = computed(() => (isBasicRepo.value ? '不能删除基础仓库' : '删除仓库，不会删除任何实际镜像'))

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

async function fetchCurrentRepoName() {
  if (!props.repoId) {
    currentRepoName.value = ''
    inputRepoName.value = ''
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
      currentRepoName.value = '（仓库不存在）'
      inputRepoName.value = ''
      isBasicRepo.value = false
      return
    }

    currentRepoName.value = repo.name || ''
    isBasicRepo.value = !!repo.basic
    if (!dialogVisible.value) {
      inputRepoName.value = currentRepoName.value
    }
  } catch (e) {
    console.error('[RepoSettingsButton] fetchCurrentRepoName failed', e)
    ElMessage.error(e.message || '获取仓库信息失败')
  }
}

async function refreshRepoInfoState(showErrorToast = false) {
  if (!props.repoId) {
    isBasicRepo.value = false
    return
  }

  loadingRepoInfo.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repo-info`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取 repo info 失败'))
    }

    const data = await res.json()
    isBasicRepo.value = !!data?.basic
    if (typeof data?.name === 'string' && data.name !== '') {
      currentRepoName.value = data.name
    }
    inputRepoName.value = currentRepoName.value
  } catch (e) {
    console.error('[RepoSettingsButton] refreshRepoInfoState failed', e)
    if (showErrorToast) {
      ElMessage.error(e.message || '获取 repo info 失败')
    }
  } finally {
    loadingRepoInfo.value = false
  }
}

async function openDialog() {
  inputRepoName.value = currentRepoName.value
  dialogVisible.value = true
  await refreshRepoInfoState(true)
}

function closeDialog() {
  dialogVisible.value = false
}

async function updateRepoName() {
  if (isBasicRepo.value) {
    ElMessage.warning('基础仓库不允许修改名称')
    return
  }
  if (!canModifyName.value) return

  const nextName = inputRepoName.value.trim()
  saving.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: nextName })
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '修改仓库名字失败'))
    }

    const data = await res.json()
    currentRepoName.value = data?.name || nextName
    inputRepoName.value = currentRepoName.value
    dialogVisible.value = false
    emitter.emit('refresh-all')
    ElMessage.success('仓库名字已修改')
  } catch (e) {
    console.error('[RepoSettingsButton] updateRepoName failed', e)
    ElMessage.error(e.message || '修改仓库名字失败')
  } finally {
    saving.value = false
  }
}

async function deleteRepo() {
  if (isBasicRepo.value) {
    ElMessage.warning('基础仓库不允许删除')
    return
  }

  const ok = window.confirm('确认删除这个仓库记录吗？不会删除任何实际镜像文件。')
  if (!ok) return

  deleting.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}`, { method: 'DELETE' })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '删除仓库失败'))
    }

    dialogVisible.value = false
    emit('deleted')
    emitter.emit('refresh-all')
    ElMessage.success('仓库已删除')
  } catch (e) {
    console.error('[RepoSettingsButton] deleteRepo failed', e)
    ElMessage.error(e.message || '删除仓库失败')
  } finally {
    deleting.value = false
  }
}

watch(
  () => props.repoId,
  () => {
    dialogVisible.value = false
    fetchCurrentRepoName()
  },
  { immediate: true }
)
</script>

<template>
  <div>
    <el-button circle size="small" :icon="Setting" @click="openDialog" :disabled="loading">
    </el-button>

    <el-dialog v-model="dialogVisible" title="Repo Info" width="620px">
      <div v-loading="loading" class="repo-info-content">
        <div v-if="errorMessage" class="text-sm text-red-600 mb-3">{{ errorMessage }}</div>

        <template v-if="repoInfo">
          <div class="info-grid">
            <div class="info-row"><span class="info-label">ID</span><span>{{ repoInfo.id }}</span></div>
            <div class="info-row"><span class="info-label">Repo UUID</span><span>{{ repoInfo.repo_uuid || '-' }}</span></div>
            <div class="info-row"><span class="info-label">Name</span><span>{{ repoInfo.name || '-' }}</span></div>
            <div class="info-row"><span class="info-label">Schema Version</span><span>{{ repoInfo.schema_version }}</span></div>
            <div class="info-row"><span class="info-label">Created At</span><span>{{ repoInfo.created_at || '-' }}</span></div>
            <div class="info-row"><span class="info-label">Updated At</span><span>{{ repoInfo.updated_at || '-' }}</span></div>
            <div class="info-row"><span class="info-label">RuleBook Name</span><span>{{ effectiveRuleBookName }}</span></div>
            <div class="info-row"><span class="info-label">RuleBook Version</span><span>{{ effectiveRuleBookVersion }}</span></div>
            <div class="info-row"><span class="info-label">RuleBook Source</span><span>{{ ruleBookSource }}</span></div>
            <div v-if="ruleBookBindingError" class="info-row">
              <span class="info-label">RuleBook Error</span>
              <span class="text-red-600">{{ ruleBookBindingError }}</span>
            </div>
          </div>

          <div class="mt-4">
            <div class="text-sm font-semibold text-slate-700 mb-2">Flags</div>
            <div class="flags-grid">
              <div class="flag-row">
                <span class="flag-key">basic</span>
                <el-checkbox :model-value="!!repoInfo.basic" disabled />
              </div>
              <div class="flag-row">
                <span class="flag-key">add_button</span>
                <el-checkbox :model-value="!!repoInfo.add_button" disabled />
              </div>
              <div class="flag-row">
                <span class="flag-key">delete_button</span>
                <el-checkbox :model-value="!!repoInfo.delete_button" disabled />
              </div>
              <div class="flag-row">
                <span class="flag-key">auto_normalize</span>
                <el-checkbox :model-value="!!repoInfo.auto_normalize" disabled />
              </div>
              <div class="flag-row">
                <span class="flag-key">show_md5</span>
                <el-checkbox :model-value="!!repoInfo.show_md5" disabled />
              </div>
              <div class="flag-row">
                <span class="flag-key">show_size</span>
                <el-checkbox :model-value="!!repoInfo.show_size" disabled />
              </div>
              <div class="flag-row">
                <span class="flag-key">single_move</span>
                <el-checkbox :model-value="!!repoInfo.single_move" disabled />
              </div>
              <div v-for="entry in extraFlagEntries" :key="entry.key" class="flag-row">
                <span class="flag-key">{{ entry.key }}</span>
                <template v-if="entry.isBoolean">
                  <el-checkbox :model-value="entry.boolValue" disabled />
                </template>
                <template v-else>
                  <span class="flag-value">{{ entry.displayValue }}</span>
                </template>
              </div>
            </div>
          </div>
        </template>
      </div>

      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Setting } from '@element-plus/icons-vue'

const props = defineProps({
  repoId: {
    type: Number,
    required: true
  }
})

const dialogVisible = ref(false)
const loading = ref(false)
const errorMessage = ref('')
const repoInfo = ref(null)
const repoRuleBookBinding = ref(null)
const ruleBookBindingError = ref('')

const parsedFlags = computed(() => {
  const raw = repoInfo.value?.flags_json
  if (!raw) return {}

  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed
    }
  } catch (_) {
    // Fall through to empty object.
  }

  return {}
})

const extraFlagEntries = computed(() => {
  return Object.keys(parsedFlags.value)
    .sort((a, b) => a.localeCompare(b, 'en', { sensitivity: 'base' }))
    .map((key) => {
      const value = parsedFlags.value[key]
      const isBoolean = typeof value === 'boolean'
      return {
        key,
        isBoolean,
        boolValue: isBoolean ? value : false,
        displayValue: isBoolean ? '' : JSON.stringify(value)
      }
    })
})

const effectiveRuleBookName = computed(() => {
  if (repoRuleBookBinding.value?.rulebook_name) {
    return repoRuleBookBinding.value.rulebook_name
  }
  const fallback = parsedFlags.value?.rulebook_name
  return typeof fallback === 'string' && fallback.trim() ? fallback : 'noop'
})

const effectiveRuleBookVersion = computed(() => {
  if (repoRuleBookBinding.value?.rulebook_version) {
    return repoRuleBookBinding.value.rulebook_version
  }
  const fallback = parsedFlags.value?.rulebook_version
  return typeof fallback === 'string' && fallback.trim() ? fallback : 'v1'
})

const ruleBookSource = computed(() => {
  return repoRuleBookBinding.value?.binding_source || (repoRuleBookBinding.value ? 'repo binding api' : 'flags_json fallback')
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

async function fetchRepoInfo(showErrorToast = false) {
  if (!props.repoId) {
    repoInfo.value = null
    repoRuleBookBinding.value = null
    ruleBookBindingError.value = ''
    errorMessage.value = ''
    return
  }

  loading.value = true
  errorMessage.value = ''
  ruleBookBindingError.value = ''
  try {
    const [repoInfoRes, ruleBookBindingRes] = await Promise.all([
      fetch(`/api/repos/${props.repoId}/repo-info`),
      fetch(`/api/repos/${props.repoId}/rulebook/binding`)
    ])

    if (!repoInfoRes.ok) {
      throw new Error(await parseErrorMessage(repoInfoRes, '获取 repo info 失败'))
    }

    const data = await repoInfoRes.json()
    repoInfo.value = data || null

    if (ruleBookBindingRes.ok) {
      repoRuleBookBinding.value = await ruleBookBindingRes.json()
    } else {
      repoRuleBookBinding.value = null
      ruleBookBindingError.value = await parseErrorMessage(ruleBookBindingRes, '获取规则书绑定失败')
    }
  } catch (e) {
    repoInfo.value = null
    repoRuleBookBinding.value = null
    errorMessage.value = e.message || '获取 repo info 失败'
    if (showErrorToast) {
      ElMessage.error(errorMessage.value)
    }
  } finally {
    loading.value = false
  }
}

function openDialog() {
  dialogVisible.value = true
  fetchRepoInfo(true)
}

watch(
  () => props.repoId,
  () => {
    dialogVisible.value = false
    fetchRepoInfo(false)
  },
  { immediate: true }
)
</script>

<style scoped>
.repo-info-content {
  min-height: 180px;
}

.info-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  gap: 12px;
  align-items: center;
  word-break: break-all;
}

.info-label {
  width: 120px;
  flex-shrink: 0;
  color: #475569;
  font-weight: 600;
}

.flags-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.flag-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.flag-key {
  width: 220px;
  flex-shrink: 0;
  color: #334155;
  font-size: 13px;
}

.flag-value {
  color: #475569;
  font-size: 13px;
  word-break: break-all;
}
</style>

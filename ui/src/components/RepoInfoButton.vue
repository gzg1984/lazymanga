<template>
  <div>
    <el-button circle size="small" :icon="Setting" @click="openDialog" :disabled="loading">
    </el-button>

    <el-dialog v-model="dialogVisible" title="Repo Info" width="600px">
      <div v-loading="loading" class="repo-info-content">
        <div v-if="errorMessage" class="text-sm text-red-600 mb-3">{{ errorMessage }}</div>

        <template v-if="repoInfo">
          <div class="info-grid">
            <div class="info-row"><span class="info-label">ID</span><span>{{ repoInfo.id }}</span></div>
            <div class="info-row"><span class="info-label">Repo UUID</span><span>{{ repoInfo.repo_uuid || '-' }}</span></div>
            <div class="info-row"><span class="info-label">Name</span><span>{{ repoInfo.name || '-' }}</span></div>
            <div class="info-row"><span class="info-label">Repo Type</span><span>{{ repoInfo.repo_type_key || '-' }}</span></div>
            <div class="info-row"><span class="info-label">Schema Version</span><span>{{ repoInfo.schema_version }}</span></div>
          </div>

          <div class="mt-4">
            <el-tabs v-model="activeTab" class="repo-info-tabs">
              <el-tab-pane label="Flag" name="flags">
                <div class="tab-section-list">
                  <div v-for="entry in boolEntries" :key="entry.key" class="status-row">
                    <span class="flag-key">{{ entry.label }}</span>
                    <el-tag :type="entry.value ? 'success' : 'info'" size="small">{{ entry.value ? '已启用' : '已关闭' }}</el-tag>
                  </div>
                </div>
              </el-tab-pane>

              <el-tab-pane label="时间" name="times">
                <div v-if="timeEntries.length" class="tab-section-list">
                  <div v-for="entry in timeEntries" :key="entry.key" class="detail-row">
                    <div class="detail-title">{{ entry.label }}</div>
                    <div class="detail-value">{{ entry.display }}</div>
                    <div class="detail-hint">{{ entry.hint }}</div>
                  </div>
                </div>
                <div v-else class="empty-tip">暂无额外时间记录。</div>
              </el-tab-pane>

              <el-tab-pane label="规则书" name="rulebook">
                <div v-if="ruleBookEntries.length" class="tab-section-list">
                  <div v-for="entry in ruleBookEntries" :key="entry.key" class="detail-row">
                    <div class="detail-title">{{ entry.label }}</div>
                    <template v-if="entry.multiline">
                      <pre class="other-code">{{ entry.display }}</pre>
                    </template>
                    <template v-else>
                      <div class="detail-value" :class="{ 'detail-error': entry.error }">{{ entry.display }}</div>
                    </template>
                    <div v-if="entry.hint" class="detail-hint">{{ entry.hint }}</div>
                  </div>
                </div>
                <div v-else class="empty-tip">暂无规则书信息。</div>
              </el-tab-pane>

              <el-tab-pane label="其他" name="others">
                <div v-if="otherEntries.length" class="tab-section-list">
                  <div v-for="entry in otherEntries" :key="entry.key" class="detail-row">
                    <div class="detail-title">{{ entry.label }}</div>
                    <template v-if="entry.multiline">
                      <pre class="other-code">{{ entry.display }}</pre>
                    </template>
                    <template v-else>
                      <div class="detail-value">{{ entry.display }}</div>
                    </template>
                    <div v-if="entry.hint" class="detail-hint">{{ entry.hint }}</div>
                  </div>
                </div>
                <div v-else class="empty-tip">暂无其他扩展信息。</div>
              </el-tab-pane>
            </el-tabs>
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
const activeTab = ref('flags')

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

const ruleBookEntries = computed(() => {
  const entries = [
    {
      key: 'rulebook_name_effective',
      label: '当前 RuleBook 名称',
      display: effectiveRuleBookName.value,
      hint: '优先显示仓库绑定结果，缺失时回退到 flags_json。'
    },
    {
      key: 'rulebook_version_effective',
      label: '当前 RuleBook 版本',
      display: effectiveRuleBookVersion.value,
      hint: '这里展示当前实际生效的规则书版本。'
    },
    {
      key: 'rulebook_source_effective',
      label: 'RuleBook 来源',
      display: ruleBookSource.value,
      hint: '用于说明当前规则书配置来自绑定接口还是 flags 回退。'
    }
  ]

  Object.keys(parsedFlags.value)
    .sort((a, b) => a.localeCompare(b, 'en', { sensitivity: 'base' }))
    .forEach((key) => {
      if (!isRuleBookKey(key)) {
        return
      }

      const value = parsedFlags.value[key]
      entries.push({
        key: `flag_${key}`,
        label: humanizeRuleBookLabel(key),
        display: formatOtherValue(value),
        multiline: typeof value === 'object' && value !== null,
        hint: '这是 repo_info.flags_json 中保存的规则书相关字段。'
      })
    })

  if (repoRuleBookBinding.value && Object.keys(repoRuleBookBinding.value).length) {
    entries.push({
      key: 'rulebook_binding_payload',
      label: '绑定详情',
      display: JSON.stringify(repoRuleBookBinding.value, null, 2),
      multiline: true,
      hint: '这是 `/rulebook/binding` 接口返回的完整信息。'
    })
  }

  if (ruleBookBindingError.value) {
    entries.push({
      key: 'rulebook_binding_error',
      label: '绑定接口错误',
      display: ruleBookBindingError.value,
      hint: '接口不可用时，会继续使用已有回退配置。',
      error: true
    })
  }

  return entries
})

const boolEntries = computed(() => {
  const info = repoInfo.value || {}
  return [
    { key: 'basic', label: '基础漫画仓库', value: !!info.basic },
    { key: 'add_button', label: '允许添加文件', value: !!info.add_button },
    { key: 'add_directory_button', label: '允许添加目录', value: !!info.add_directory_button },
    { key: 'delete_button', label: '允许删除内容', value: !!info.delete_button },
    { key: 'auto_normalize', label: '自动归类', value: !!info.auto_normalize },
    { key: 'show_md5', label: '显示 MD5', value: !!info.show_md5 },
    { key: 'show_size', label: '显示大小', value: !!info.show_size },
    { key: 'single_move', label: '允许单条移动', value: !!info.single_move }
  ]
})

const timeEntries = computed(() => {
  const info = repoInfo.value || {}
  const entries = []

  if (info.created_at) {
    entries.push({
      key: 'created_at',
      label: '仓库信息创建时间',
      display: formatDateTime(info.created_at),
      hint: '这条仓库说明信息首次写入 repo.db 的时间。'
    })
  }
  if (info.updated_at) {
    entries.push({
      key: 'updated_at',
      label: '仓库信息最后更新时间',
      display: formatDateTime(info.updated_at),
      hint: '最近一次修改该仓库配置或元信息的时间。'
    })
  }

  Object.entries(parsedFlags.value).forEach(([key, value]) => {
    if (isRuleBookKey(key) || !looksLikeTimeEntry(key, value)) return
    entries.push({
      key,
      label: humanizeTimeLabel(key),
      display: formatDateTime(value),
      hint: humanizeTimeHint(key)
    })
  })

  return entries
})

const otherEntries = computed(() => {
  const entries = []
  const overrideRaw = String(repoInfo.value?.settings_override_json || '').trim()

  entries.push({
    key: 'settings_override_json',
    label: '当前 Overlay 配置',
    display: formatStructuredJSON(overrideRaw, '（当前全部继承模板，无额外覆盖）'),
    multiline: true,
    hint: '这里显示这个仓库自己覆盖模板的差异项。'
  })

  Object.keys(parsedFlags.value)
    .sort((a, b) => a.localeCompare(b, 'en', { sensitivity: 'base' }))
    .forEach((key) => {
      const value = parsedFlags.value[key]
      if (typeof value === 'boolean' || looksLikeTimeEntry(key, value) || isRuleBookKey(key)) {
        return
      }

      entries.push({
        key,
        label: humanizeOtherLabel(key),
        display: formatOtherValue(value),
        multiline: typeof value === 'object' && value !== null,
        hint: '这是 repo_info.flags_json 中的扩展数据。'
      })
    })

  return entries
})

function isRuleBookKey(key) {
  return String(key || '').toLowerCase().includes('rulebook')
}

function looksLikeTimeEntry(key, value) {
  if (typeof value !== 'string') return false
  const lowerKey = String(key || '').toLowerCase()
  if (lowerKey.endsWith('_at') || lowerKey.includes('time')) {
    return true
  }
  return !Number.isNaN(Date.parse(value)) && value.includes('T')
}

function formatDateTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value || '-')
  }
  return date.toLocaleString('zh-CN', { hour12: false })
}

function humanizeTimeLabel(key) {
  const mapping = {
    legacy_base_iso_to_repoisos_migrated_at: '旧版基础数据迁移时间',
    legacy_base_iso_to_repoisos_notice_shown_at: '升级提示已读时间',
    legacy_base_iso_repoisos_metadata_backfill_at: '基础仓库元数据补齐时间'
  }
  return mapping[key] || `时间记录：${key}`
}

function humanizeTimeHint(key) {
  const mapping = {
    legacy_base_iso_to_repoisos_migrated_at: '记录旧版基础 ISO 数据迁移到仓库模式的执行时间。',
    legacy_base_iso_to_repoisos_notice_shown_at: '记录用户已阅读升级迁移提示的时间。',
    legacy_base_iso_repoisos_metadata_backfill_at: '记录基础仓库补齐文件元数据的时间。'
  }
  return mapping[key] || '这是系统记录的一次状态时间点。'
}

function humanizeRuleBookLabel(key) {
  const mapping = {
    rulebook_name: 'flags_json 中的 RuleBook 名称',
    rulebook_version: 'flags_json 中的 RuleBook 版本'
  }
  return mapping[key] || `RuleBook 字段：${key}`
}

function humanizeOtherLabel(key) {
  const mapping = {
    legacy_base_iso_to_repoisos_migrated_count: '旧版迁移成功条数',
    legacy_base_iso_to_repoisos_skipped_count: '旧版迁移跳过条数',
    legacy_base_iso_to_repoisos_source: '迁移来源',
    legacy_base_iso_repoisos_metadata_backfill_count: '元数据补齐条数'
  }
  return mapping[key] || key
}

function formatStructuredJSON(raw, emptyText = '-') {
  const trimmed = String(raw || '').trim()
  if (!trimmed || trimmed === '{}' || trimmed === 'null') {
    return emptyText
  }

  try {
    const parsed = JSON.parse(trimmed)
    return JSON.stringify(parsed, null, 2)
  } catch (_) {
    return trimmed
  }
}

function formatOtherValue(value) {
  if (value === null || typeof value === 'undefined') {
    return '-'
  }
  if (typeof value === 'object') {
    return JSON.stringify(value, null, 2)
  }
  return String(value)
}

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
  activeTab.value = 'flags'
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

.repo-info-tabs :deep(.el-tabs__header) {
  margin-bottom: 10px;
}

.tab-section-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #f8fafc;
}

.flag-key {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.detail-row {
  padding: 10px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #f8fafc;
}

.detail-title {
  color: #334155;
  font-size: 13px;
  font-weight: 700;
}

.detail-value {
  margin-top: 4px;
  color: #475569;
  font-size: 13px;
  word-break: break-all;
}

.detail-hint {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.detail-error {
  color: #dc2626;
}

.other-code {
  margin-top: 6px;
  padding: 8px;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

.empty-tip {
  color: #64748b;
  font-size: 13px;
}
</style>

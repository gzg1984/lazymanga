<template>
  <el-dialog
    :model-value="modelValue"
    title="手动修改"
    width="640px"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="manual-edit-content">
      <p class="manual-edit-desc">设置类型和名字策略后，点击修改会提交到后端执行文件移动与记录更新。</p>

      <div class="manual-edit-meta">
        <div><span class="meta-label">repoId:</span> {{ repoId }}</div>
        <div><span class="meta-label">isoId:</span> {{ displayRecord?.id ?? '-' }}</div>
        <div class="break-all"><span class="meta-label">path:</span> {{ displayRecord?.path || '-' }}</div>
        <div class="break-all"><span class="meta-label">md5:</span> {{ displayRecord?.md5 || '（待计算）' }}</div>
        <div>
          <span class="meta-label">文件大小:</span>
          {{ formatSizeHuman(displayRecord) }}
          <span class="meta-sub">（{{ formatSizeBytes(displayRecord) }}）</span>
        </div>
        <div>
          <span class="meta-label">文件状态:</span>
          <span :class="displayRecord?.is_missing ? 'status-missing' : 'status-ok'">
            {{ displayRecord?.is_missing ? '文件失踪' : '正常' }}
          </span>
        </div>
      </div>

      <div class="manual-edit-form">
        <div class="form-row">
          <el-checkbox :model-value="autoNormalizeEnabled" disabled>自动迁移路径（由仓库设置决定）</el-checkbox>
        </div>

        <div class="form-row">
          <div class="form-label">类型</div>
          <el-radio-group v-model="form.targetType" :disabled="submitting || !isoRecord">
            <el-radio-button label="os">OS</el-radio-button>
            <el-radio-button label="entertainment">娱乐</el-radio-button>
            <el-radio-button label="others">Others</el-radio-button>
          </el-radio-group>
        </div>

        <div class="form-row">
          <div class="form-label">修改名字</div>
          <el-radio-group v-model="form.nameMode" :disabled="submitting || !isoRecord">
            <el-radio label="auto">自动</el-radio>
            <el-radio label="manual">手动</el-radio>
          </el-radio-group>
        </div>

        <div class="form-row" v-if="form.nameMode === 'manual'">
          <div class="form-label">新名字</div>
          <el-input
            v-model="form.manualName"
            :disabled="submitting || !isoRecord"
            placeholder="输入新的文件名（必须以 .iso 结尾）"
          />
        </div>
      </div>
    </div>

    <template #footer>
      <el-button :disabled="submitting" @click="emit('update:modelValue', false)">关闭</el-button>
      <el-button :loading="refreshing" :disabled="submitting || !displayRecord?.id" @click="refreshRecordMetadata">刷新</el-button>
      <el-button type="primary" :loading="submitting" :disabled="!canSubmit" @click="submitManualEdit">修改</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import emitter from '../eventBus'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  repoId: {
    type: Number,
    required: true
  },
  isoRecord: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:modelValue'])

const submitting = ref(false)
const refreshing = ref(false)
const displayRecord = ref(null)
const autoNormalizeEnabled = ref(false)
const form = reactive({
  targetType: 'os',
  nameMode: 'auto',
  manualName: ''
})

const manualNameLooksValid = computed(() => {
  if (form.nameMode !== 'manual') return true
  const value = form.manualName.trim()
  return value !== '' && /\.iso$/i.test(value)
})

const canSubmit = computed(() => {
  if (!displayRecord.value?.id) return false
  if (!manualNameLooksValid.value) return false
  return true
})

function parseSizeBytes(v) {
  if (v && typeof v === 'object') {
    const candidates = [v.size_bytes, v.sizeBytes, v.size]
    for (const candidate of candidates) {
      const parsed = Number(candidate)
      if (Number.isFinite(parsed)) {
        return parsed
      }
    }
    return null
  }

  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

function formatSizeHuman(v) {
  const size = parseSizeBytes(v)
  if (size === null || size === -1) return '待计算'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  if (unitIndex === 0) {
    return `${Math.round(value)} ${units[unitIndex]}`
  }
  return `${value.toFixed(2)} ${units[unitIndex]}`
}

function formatSizeBytes(v) {
  const size = parseSizeBytes(v)
  if (size === null || size === -1) return '待计算'
  return `${Math.round(size)} B`
}

function inferTargetTypeFromRecord(record) {
  if (record?.is_entertament && !record?.is_os) {
    return 'entertainment'
  }
  if (record?.is_os && !record?.is_entertament) {
    return 'os'
  }
  return 'others'
}

function inferCurrentName(path, fallbackName) {
  const name = String(fallbackName || '').trim()
  if (name) return name
  const normalized = String(path || '').replace(/\\/g, '/').trim()
  if (!normalized) return ''
  const parts = normalized.split('/')
  return parts[parts.length - 1] || ''
}

function resetFormFromRecord() {
  displayRecord.value = props.isoRecord ? { ...props.isoRecord } : null
  form.targetType = inferTargetTypeFromRecord(displayRecord.value)
  form.nameMode = 'auto'
  form.manualName = inferCurrentName(displayRecord.value?.path, displayRecord.value?.filename)
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

async function fetchRepoInfoFlag() {
  if (!props.repoId) {
    autoNormalizeEnabled.value = false
    return
  }

  try {
    const res = await fetch(`/api/repos/${props.repoId}/repo-info`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取 repo info 失败'))
    }

    const data = await res.json()
    autoNormalizeEnabled.value = !!data?.auto_normalize
  } catch (e) {
    autoNormalizeEnabled.value = false
    ElMessage.error(e.message || '获取 repo info 失败')
  }
}

async function submitManualEdit() {
  if (!displayRecord.value?.id) {
    ElMessage.error('缺少ISO记录信息，无法修改')
    return
  }
  if (!manualNameLooksValid.value) {
    ElMessage.error('手动模式下，新名字必须以 .iso 结尾')
    return
  }

  submitting.value = true
  try {
    const payload = {
      target_type: form.targetType,
      name_mode: form.nameMode,
      manual_name: form.manualName.trim()
    }

    const res = await fetch(`/api/repos/${props.repoId}/repoisos/${displayRecord.value.id}/manual-edit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '手动修改失败'))
    }

    await res.json()
    emitter.emit('refresh-repo', { repoId: props.repoId })
    ElMessage.success('手动修改已提交')
    emit('update:modelValue', false)
  } catch (e) {
    ElMessage.error(e.message || '手动修改失败')
  } finally {
    submitting.value = false
  }
}

async function refreshRecordMetadata() {
  if (!displayRecord.value?.id) {
    ElMessage.error('缺少ISO记录信息，无法刷新')
    return
  }

  refreshing.value = true
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos/${displayRecord.value.id}/refresh`, {
      method: 'POST'
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '刷新失败'))
    }

    const data = await res.json()
    if (data?.record) {
      displayRecord.value = { ...data.record }
    }

    emitter.emit('refresh-repo', { repoId: props.repoId })

    if (!data?.exists) {
      ElMessage.warning('当前记录对应文件不存在，已标记为文件失踪，可直接删除记录')
      return
    }

    const parts = []
    if (data?.path_moved) parts.push('路径重定位')
    if (data?.md5_updated) parts.push('md5')
    if (data?.size_updated) parts.push('文件大小')

    if (parts.length > 0) {
      ElMessage.success(`刷新完成，已补充：${parts.join('、')}`)
      return
    }

    ElMessage.info('文件存在，md5和文件大小均已存在')
  } catch (e) {
    ElMessage.error(e.message || '刷新失败')
  } finally {
    refreshing.value = false
  }
}

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      resetFormFromRecord()
      fetchRepoInfoFlag()
    }
  }
)

watch(
  () => props.repoId,
  () => {
    autoNormalizeEnabled.value = false
    if (props.modelValue) {
      fetchRepoInfoFlag()
    }
  }
)
</script>

<style scoped>
.manual-edit-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.manual-edit-desc {
  color: #475569;
  font-size: 14px;
}

.manual-edit-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #f8fafc;
  font-size: 14px;
  color: #334155;
}

.meta-label {
  font-weight: 600;
}

.meta-sub {
  color: #64748b;
  margin-left: 4px;
}

.manual-edit-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 12px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
}

.form-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-label {
  font-size: 14px;
  font-weight: 600;
  color: #334155;
}

.status-ok {
  color: #166534;
  font-weight: 600;
}

.status-missing {
  color: #b91c1c;
  font-weight: 700;
}
</style>

<template>
  <el-dialog
    :model-value="modelValue"
    :title="dialogTitle"
    width="640px"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="delete-content">
      <p class="delete-desc">请选择删除方式。你可以只删除管理要素记录，也可以同时删除记录和实际{{ deleteTargetLabel }}。</p>

      <div class="delete-meta">
        <div><span class="meta-label">仓库 ID:</span> {{ repoId }}</div>
        <div><span class="meta-label">要素 ID:</span> {{ displayRecord?.id ?? '-' }}</div>
        <div><span class="meta-label">要素类型:</span> {{ elementTypeLabel }}</div>
        <div class="break-all"><span class="meta-label">路径:</span> {{ displayRecord?.path || '-' }}</div>
        <div class="break-all"><span class="meta-label">MD5:</span> {{ isDirectoryRecord ? '不适用' : (displayRecord?.md5 || '（待计算）') }}</div>
        <div>
          <span class="meta-label">文件大小:</span>
          {{ formatSizeHuman(displayRecord) }}
          <span class="meta-sub">（{{ formatSizeBytes(displayRecord) }}）</span>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button :disabled="submitting" @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="warning" :loading="submittingMode === 'record'" :disabled="!displayRecord?.id || submitting" @click="submitDelete(false)">
        删除记录但不删除{{ deleteTargetLabel }}
      </el-button>
      <el-button type="danger" :loading="submittingMode === 'record-and-file'" :disabled="!displayRecord?.id || submitting" @click="submitDelete(true)">
        删除记录和{{ deleteTargetLabel }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
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

const displayRecord = ref(null)
const submittingMode = ref('')
const submitting = computed(() => submittingMode.value !== '')
const isDirectoryRecord = computed(() => !!(displayRecord.value?.is_directory ?? displayRecord.value?.isDirectory))
const recordSuffix = computed(() => {
  const fileName = String(displayRecord.value?.filename || displayRecord.value?.fileName || displayRecord.value?.path || '').trim()
  const normalized = fileName.replace(/\\/g, '/')
  const lastSegment = normalized.split('/').filter(Boolean).pop() || ''
  const match = lastSegment.match(/(\.[a-z0-9]{1,12})$/i)
  return match ? match[1].toLowerCase() : ''
})
const elementTypeLabel = computed(() => {
  if (isDirectoryRecord.value) return '目录'
  if (recordSuffix.value) return `${recordSuffix.value} 文件`
  return '文件'
})
const deleteTargetLabel = computed(() => (isDirectoryRecord.value ? '目录' : elementTypeLabel.value))
const dialogTitle = computed(() => (isDirectoryRecord.value ? '删除目录' : `删除 ${elementTypeLabel.value}`))

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
  if (isDirectoryRecord.value) return '目录'

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
  if (isDirectoryRecord.value) return '不适用'

  const size = parseSizeBytes(v)
  if (size === null || size === -1) return '待计算'
  return `${Math.round(size)} B`
}

function resetRecord() {
  displayRecord.value = props.isoRecord ? { ...props.isoRecord } : null
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

async function submitDelete(deleteFile) {
  if (!displayRecord.value?.id) {
    ElMessage.error('缺少要素记录信息，无法删除')
    return
  }

  submittingMode.value = deleteFile ? 'record-and-file' : 'record'
  try {
    const res = await fetch(`/api/repos/${props.repoId}/repoisos/${displayRecord.value.id}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ delete_file: deleteFile })
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, deleteFile ? `删除记录和${deleteTargetLabel.value}失败` : '删除记录失败'))
    }

    const data = await res.json()
    emitter.emit('refresh-repo', { repoId: props.repoId })

    if (deleteFile) {
      if (data?.file_deleted) {
        ElMessage.success(`已删除记录和${deleteTargetLabel.value}`)
      } else if (data?.file_missing) {
        ElMessage.warning(`已删除记录，${deleteTargetLabel.value}原本不存在`)
      } else {
        ElMessage.success('已删除记录')
      }
    } else {
      ElMessage.success(`已删除记录，保留${deleteTargetLabel.value}`)
    }

    emit('update:modelValue', false)
  } catch (e) {
    ElMessage.error(e.message || (deleteFile ? `删除记录和${deleteTargetLabel.value}失败` : '删除记录失败'))
  } finally {
    submittingMode.value = ''
  }
}

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      resetRecord()
    }
  }
)
</script>

<style scoped>
.delete-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.delete-desc {
  color: #475569;
  font-size: 14px;
}

.delete-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid #fecaca;
  border-radius: 8px;
  background: #fff7f7;
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
</style>

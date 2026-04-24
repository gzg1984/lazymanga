<template>
  <div>
    <el-button type="info" size="small" @click="dialogVisible = true">Detail</el-button>
    <el-dialog 
      v-model="dialogVisible" 
      title="ISO详情" 
      width="500px" 
      :modal="true" 
      :close-on-click-modal="false"
      :append-to-body="true"
      :lock-scroll="true"
      class="iso-detail-dialog">
      <div v-if="row" class="dialog-content">
        <p><b>ID：</b>{{ row.id }}</p>
        <p><b>文件名：</b>{{ row.filename || row.name }}</p>
        <p><b>路径：</b>{{ row.path }}</p>
        <p><b>MD5：</b>{{ row.md5 }}</p>
        <p><b>标签：</b>{{ row.tags }}</p>
        <p><b>是否挂载：</b>{{ row.ismounted ? '已挂载' : '未挂载' }}</p>
      </div>
      <template #footer>
        <div class="footer-btns">
            <el-button type="danger" @click="handleDelete" style="float:left;background:#e53935;color:#fff;border:none;">删除</el-button>
            <el-button type="warning" :loading="refreshing" @click="handleRefresh" style="margin-left:8px;">刷新</el-button>
            <el-button type="primary" :loading="downloading" @click="handleDownload" style="margin-left:8px;">下载</el-button>
            <span v-if="downloadHint" class="download-hint">{{ downloadHint }}</span>
            <el-button @click="dialogVisible = false">关闭</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElButton, ElDialog, ElMessage } from 'element-plus'
import emitter from '../eventBus'
import { findLatestTaskBySource, getTaskHint, isTaskBusy, startManagedDownload } from '../download/manager'
const props = defineProps({ row: Object })
const dialogVisible = ref(false)
const refreshing = ref(false)

function buildDownloadSourceKey(row) {
  if (!row) {
    return ''
  }

  if (row.id) {
    return `iso-detail:${row.id}`
  }

  return `iso-detail:path:${row.path || ''}`
}

const downloadTask = computed(() => {
  const sourceKey = buildDownloadSourceKey(props.row)
  return findLatestTaskBySource(sourceKey)
})

const downloading = computed(() => isTaskBusy(downloadTask.value))
const downloadHint = computed(() => getTaskHint(downloadTask.value))

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

async function handleDelete() {
  if (!props.row || !props.row.id) {
    ElMessage.error('未获取到ISO记录ID，无法删除')
    return
  }
  try {
    const res = await fetch(`/api/delisos/${props.row.id}`, {
      method: 'DELETE'
    })
    if (!res.ok) {
      const text = await res.text()
      ElMessage.error('删除失败：' + text)
      return
    }
  ElMessage.success('ISO记录已删除')
  dialogVisible.value = false
  emitter.emit('refresh-all') // 通知父组件刷新ISO列表
  } catch (e) {
    ElMessage.error('删除请求失败')
  }
}

async function handleRefresh() {
  if (!props.row || !props.row.id) {
    ElMessage.error('未获取到ISO记录ID，无法刷新')
    return
  }

  refreshing.value = true
  try {
    const res = await fetch(`/api/isos/${props.row.id}/file-status`)
    if (!res.ok) {
      ElMessage.error(await parseErrorMessage(res, '刷新失败'))
      return
    }

    const data = await res.json()
    if (data?.exists) {
      ElMessage.success('文件存在，记录有效')
      return
    }

    const tip = '本记录已失效，可以删除。是否立即删除该记录？'
    const shouldDelete = window.confirm(tip)
    if (shouldDelete) {
      await handleDelete()
    } else {
      ElMessage.warning('本记录已失效，可以删除')
    }
  } catch (e) {
    ElMessage.error('刷新请求失败')
  } finally {
    refreshing.value = false
  }
}

// 下载处理（前端实现）
async function handleDownload() {
  if (!props.row || !props.row.path) {
    ElMessage.error('未获取到ISO路径，无法下载')
    return
  }

  const sourceKey = buildDownloadSourceKey(props.row)
  try {
    const url = '/api/download?path=' + encodeURIComponent(props.row.path)
    const fallbackFileName = props.row.filename || props.row.name || 'download.iso'
    const result = await startManagedDownload({
      sourceKey,
      sourceLabel: 'ISO 详情下载',
      url,
      fallbackFileName
    })

    if (result.ok) {
      if (result.task.status === 'delegated') {
        ElMessage.success('已交给浏览器下载管理：' + (result.task.fileName || fallbackFileName))
      } else {
        ElMessage.success('下载已开始：' + (result.task.fileName || fallbackFileName))
      }
      return
    }

    ElMessage.error(result.task.errorMessage || '下载失败')
  } catch (e) {
    ElMessage.error('下载请求失败')
  }
}
</script>

<style>
p { margin: 8px 0; }

/* 全局弹窗遮罩层样式 */
.el-overlay {
  background-color: rgba(0, 0, 0, 0.6) !important;
  z-index: 2000 !important;
}

/* 弹窗主体样式 */
.iso-detail-dialog {
  background: #ffffff !important;
  border-radius: 12px !important;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4) !important;
  z-index: 2001 !important;
}

/* 弹窗内容区域 */
.dialog-content {
  background: #ffffff !important;
  padding: 16px;
  border-radius: 8px;
  min-height: 200px;
}

.footer-btns {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  flex-wrap: wrap;
  gap: 8px;
}

.download-hint {
  max-width: 360px;
  color: #475569;
  font-size: 12px;
  line-height: 1.3;
}

/* 强制所有弹窗相关元素背景 */
.el-dialog {
  background: #ffffff !important;
  border: 2px solid #e0e0e0 !important;
}

.el-dialog__header {
  background: #f8f9fa !important;
  border-bottom: 2px solid #e0e0e0 !important;
  padding: 16px 20px !important;
}

.el-dialog__body {
  background: #ffffff !important;
  color: #333333 !important;
  padding: 20px !important;
}

.el-dialog__footer {
  background: #f8f9fa !important;
  border-top: 2px solid #e0e0e0 !important;
  padding: 16px 20px !important;
}
</style>

<template>
  <div v-if="visibleTaskCount > 0" class="download-panel">
    <div class="download-panel-header">
      <div class="download-panel-title">下载任务</div>
      <div class="download-panel-actions">
        <el-button type="info" text size="small" @click="minimized = !minimized">
          {{ minimized ? '展开' : '收起' }}
        </el-button>
        <el-button type="info" text size="small" @click="clearFinishedTasks">清理完成</el-button>
      </div>
    </div>

    <div v-if="!minimized" class="download-panel-list">
      <div v-if="showEmbeddedDownloadTip" class="download-mode-tip">
        下载中，请不要关闭界面，下载完成后弹出保存文件窗口，保存即可。
      </div>

      <div v-for="task in downloadTasks" :key="task.id" class="download-task-row">
        <div class="download-task-top-row">
          <div class="download-task-name">{{ task.fileName || task.fallbackFileName }}</div>
          <div class="download-task-actions">
            <el-button
              v-if="task.canCancel"
              size="small"
              type="warning"
              text
              @click="cancelTask(task.id)"
            >
              取消
            </el-button>
            <el-button
              v-if="task.canRemove"
              size="small"
              type="info"
              text
              @click="removeTask(task.id)"
            >
              移除
            </el-button>
          </div>
        </div>

        <div class="download-task-sub-row">
          <span>{{ task.sourceLabel || '下载' }}</span>
          <span class="download-task-status">{{ statusLabel(task.status) }}</span>
        </div>

        <el-progress
          v-if="task.status === 'running' && task.totalBytes > 0"
          :percentage="Math.max(0, Math.min(100, Math.round(task.percent || 0)))"
          :stroke-width="12"
        />

        <div class="download-task-meta">{{ getTaskHint(task) }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import {
  cancelTask,
  clearFinishedTasks,
  downloadTasks,
  getTaskHint,
  removeTask
} from '../download/manager'

const minimized = ref(false)
const visibleTaskCount = computed(() => downloadTasks.value.length)
const showEmbeddedDownloadTip = computed(() => {
  return downloadTasks.value.some((task) => {
    if (task.mode !== 'ui') {
      return false
    }
    return task.status === 'pending' || task.status === 'running'
  })
})

function statusLabel(status) {
  if (status === 'running') {
    return '下载中'
  }
  if (status === 'delegated') {
    return '浏览器接管'
  }
  if (status === 'completed') {
    return '已完成'
  }
  if (status === 'failed') {
    return '失败'
  }
  if (status === 'canceled') {
    return '已取消'
  }
  return '等待中'
}
</script>

<style scoped>
.download-panel {
  position: fixed;
  right: 16px;
  bottom: 16px;
  width: min(460px, calc(100vw - 24px));
  max-height: min(60vh, 520px);
  border: 1px solid #7dd3fc;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.22);
  z-index: 2200;
  overflow: hidden;
}

.download-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid #bae6fd;
  background: linear-gradient(120deg, #e0f2fe 0%, #f0f9ff 100%);
}

.download-panel-title {
  font-size: 14px;
  font-weight: 700;
  color: #0f172a;
}

.download-panel-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.download-panel-list {
  max-height: min(48vh, 420px);
  overflow: auto;
  padding: 8px 10px 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.download-mode-tip {
  border: 1px solid #fde68a;
  background: #fefce8;
  color: #854d0e;
  border-radius: 8px;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.5;
}

.download-task-row {
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 8px;
  background: #ffffff;
}

.download-task-top-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.download-task-name {
  font-size: 13px;
  font-weight: 600;
  color: #0f172a;
  word-break: break-all;
}

.download-task-actions {
  flex-shrink: 0;
}

.download-task-sub-row {
  margin-top: 3px;
  display: flex;
  justify-content: space-between;
  color: #64748b;
  font-size: 12px;
}

.download-task-status {
  font-weight: 600;
}

.download-task-meta {
  margin-top: 4px;
  color: #334155;
  font-size: 12px;
}
</style>

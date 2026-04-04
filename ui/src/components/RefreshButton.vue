<template>
  <el-button type="success" @click="onRefresh">刷新</el-button>
</template>

<script setup>
import { ElMessage } from 'element-plus'
import emitter from '../eventBus'

const props = defineProps({
  activeTab: {
    type: String,
    default: 'base'
  },
  activeRepoId: {
    type: Number,
    default: null
  }
})

function onRefresh() {
  ElMessage.success('刷新按钮被点击！')

  if (props.activeTab === 'base') {
    // Keep base tab behavior unchanged.
    emitter.emit('refresh-all')
    return
  }

  if (props.activeRepoId !== null) {
    emitter.emit('refresh-repo', { repoId: props.activeRepoId })
  }
}
</script>

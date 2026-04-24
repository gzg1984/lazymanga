<template>
  <div class="lzfile-full">
    <lzc-file-picker
      type="file"
      base-url="/_lzc/files/home"
      accept="application/x-cd-image"
      :multiple="false"
      :is-modal="true"
      :choice-file-only="true"
      title="选择ISO文件"
      confirm-button-title="确认选择"
      @close="handleClose"
      @submit="handleSubmit"
    />
  </div>
</template>
<style scoped>
.lzfile-full {
  height: 80vh;
  min-height: 400px;
  display: flex;
  flex-direction: column;
}
.lzfile-full > * {
  flex: 1 1 0%;
  min-height: 0;
}
</style>


<script>
import { ElMessage } from 'element-plus'
export default {
  methods: {
    handleClose() {
      ElMessage.info('已取消文件选择')
      this.$emit('close')
    },
    handleSubmit(files) {
      let msg = '选中的文件: '
      let names = ''
      // 事件对象误触发的情况
      if (files && typeof files === 'object' && 'isTrusted' in files) {
        ElMessage.error('文件选择失败：未正确获取到文件，请重试！')
        ElMessage.info('files原始内容: ' + JSON.stringify(files))
        this.$emit('close')
        return
      }
      if (Array.isArray(files)) {
        names = files.length ? files.map(f => f.name || f.path).join(', ') : ''
      } else if (files && typeof files === 'object' && (files.name || files.path)) {
        names = files.name || files.path
      } else if (typeof files === 'string') {
        names = files
      }
      msg += names || '无'
      ElMessage.success(msg)
      ElMessage.info('files原始内容: ' + JSON.stringify(files))
      this.$emit('submitfiles', files)
      this.$emit('close')
    }
  }
}
</script>
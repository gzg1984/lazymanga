<template>
  <button
    class="px-3 py-1 rounded bg-slate-200 hover:bg-slate-400 hover:text-white transition"
    style="width:100%; min-width:60px; box-sizing:border-box;"
    @click="handleMount"
  >
    <slot></slot>
  </button>
</template>

<script setup>
import { ElMessage } from 'element-plus'
const props = defineProps({
  path: String
})
const handleMount = async () => {
  try {
    const res = await fetch('/api/mount', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: props.path }),
    })
    if (!res.ok) throw new Error('挂载失败')
    ElMessage.success('挂载请求已发送')
  } catch (e) {
    ElMessage.error('挂载失败')
  }
}
</script>

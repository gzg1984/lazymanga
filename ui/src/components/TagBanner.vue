<template>
  <div v-if="tags.length" class="mb-4 p-3 rounded bg-blue-100 border border-blue-400 text-blue-800 flex flex-wrap gap-2">
    <span
      v-for="tag in tags"
      :key="tag"
      class="px-2 py-1 rounded text-sm cursor-pointer transition"
      :class="selectedTag === tag ? 'bg-blue-600 text-white' : 'bg-blue-200 hover:bg-blue-400 hover:text-white'"
      @click="selectTag(tag)"
    >
      {{ tag }}
    </span>
    <span
      v-if="selectedTag"
      class="ml-4 px-2 py-1 bg-gray-300 rounded text-xs cursor-pointer hover:bg-gray-400"
      @click="clearTag"
    >清除筛选</span>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import emitter from '../eventBus'


const tags = ref([])
const selectedTag = ref("")
import { defineEmits } from 'vue'
const emit = defineEmits(['tag-selected'])
function selectTag(tag) {
  selectedTag.value = tag
  emit('tag-selected', tag)
}

function clearTag() {
  selectedTag.value = ""
  emit('tag-selected', "")
}

const fetchTags = async () => {
  try {
    const res = await fetch('/api/queryalltags')
    if (!res.ok) throw new Error('网络错误')
    const data = await res.json()
    tags.value = Array.isArray(data) ? data : []
  } catch (e) {
    tags.value = []
  }
}

function handleRefreshAll() {
  fetchTags()
}

onMounted(() => {
  fetchTags()
  emitter.on('refresh-all', handleRefreshAll)
})
onUnmounted(() => {
  emitter.off('refresh-all', handleRefreshAll)
})
</script>

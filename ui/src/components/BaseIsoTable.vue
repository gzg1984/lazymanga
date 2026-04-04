<template>
  <div class="iso-table-wrap p-6 border-4 border-slate-400 rounded-lg bg-slate-100">
    <el-table :data="isoList" style="width: 100%" border>
      <el-table-column prop="path" label="iso文件" />
      <el-table-column prop="md5" label="md5" />
      <template #header>
        <div class="flex justify-between items-center w-full">
          <span>ISO文件列表</span>
          <el-button type="primary" @click="emit('add')">添加</el-button>
        </div>
      </template>
      <el-table-column label="详情">
        <template #default="scope">
          <IsoDetailColumn :row="scope.row" />
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import IsoDetailColumn from '../views/IsoDetailColumn.vue'

defineProps({
  isoList: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['add'])
</script>

<style scoped>
.el-table {
  --el-table-border-color: #94a3b8;
  --el-table-border: 2px solid #94a3b8;
  border-radius: 8px;
  font-size: 16px;
}

.el-table th,
.el-table td {
  border-right: 2px solid #94a3b8 !important;
  border-bottom: 2px solid #94a3b8 !important;
}

.el-table th:last-child,
.el-table td:last-child {
  border-right: none !important;
}

.el-table tr:last-child td {
  border-bottom: none !important;
}
</style>

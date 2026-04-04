
<template>
	<div class="input-wrap flex border-2 border-slate-400 transition ease duration-300">
		<el-button type="primary" @click="openDialog">添加</el-button>
	</div>
	<el-dialog v-model="dialogVisible" title="文件列表" width="600px">
		<el-table v-if="fileList.length" :data="fileList" style="width: 100%" @row-click="handleRowClick">
			<el-table-column prop="name" label="文件名">
				<template #default="scope">
					<span v-if="scope.row.isDir" style="color: #409EFF; cursor: pointer;">
						📁 {{ scope.row.name }}
					</span>
					<span v-else>
						{{ scope.row.name }}
					</span>
				</template>
			</el-table-column>
			<el-table-column prop="size" label="大小" />
		</el-table>
		<div v-else>暂无文件</div>
		<template #footer>
			<el-button @click="dialogVisible = false">关闭</el-button>
		</template>
	</el-dialog>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { ElDialog, ElTable, ElTableColumn, ElButton, ElMessage } from 'element-plus'
import 'element-plus/dist/index.css'
import emitter from '../eventBus'

const emit = defineEmits(['create-todo'])

const todoState = reactive({
	todo: '',
	invalid: null,
	errMsg: '',
})




const dialogVisible = ref(false)
const fileList = ref([])
const currentDir = ref("")


function parentDirOf(dir) {
	const normalized = String(dir || '').split('/').filter(Boolean)
	normalized.pop()
	return normalized.join('/')
}


function openDialog() {
	dialogVisible.value = true
	currentDir.value = ""
	fetchFiles()
}

async function fetchFiles(dir = "") {
	let url = "/api/files"
	if (dir) {
		url += `?dir=${encodeURIComponent(dir)}`
	}
	try {
		const res = await fetch(url)
		if (!res.ok) throw new Error('网络错误')
		const data = await res.json()
		if (Array.isArray(data)) {
			const entries = [...data]
			if (dir) {
				entries.unshift({ name: '..', size: 0, isDir: true, isParentDir: true })
			}
			fileList.value = entries
		} else {
			fileList.value = dir ? [{ name: '..', size: 0, isDir: true, isParentDir: true }] : []
		}
		if (fileList.value.length === 0) {
			ElMessage.info('没有可用的文件')
		}
	} catch (e) {
		fileList.value = []
		ElMessage.error('获取文件列表失败')
	}
}

async function handleRowClick(row) {
	if (row.isParentDir) {
		currentDir.value = parentDirOf(currentDir.value)
		fetchFiles(currentDir.value)
		return
	}

	if (row.isDir) {
		currentDir.value = currentDir.value ? `${currentDir.value}/${row.name}` : row.name
		fetchFiles(currentDir.value)
	} else {
		// 选中iso文件，组合完整路径并发送添加请求
		const fullPath = currentDir.value ? `${currentDir.value}/${row.name}` : row.name
		try {
			const res = await fetch('/api/addiso', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ path: fullPath }),
			})
			if (res.status === 409) {
				const data = await res.json()
				ElMessage.error('添加失败：ISO记录重复。' + (data.error || ''))
				return
			}
			if (!res.ok) {
				const text = await res.text()
				ElMessage.error('添加失败，后端错误码 ' + res.status + '。部分原始响应：' + text.slice(0, 100))
				return
			}
			ElMessage.success('已添加到ISO列表')
			emitter.emit('iso-added', fullPath)
			dialogVisible.value = false
		} catch (e) {
			ElMessage.error('添加到ISO列表失败')
		}
	}
}

</script>

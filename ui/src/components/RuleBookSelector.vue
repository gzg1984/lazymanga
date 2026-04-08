<template>
  <div class="rulebook-selector">
    <div class="rulebook-selector-row">
      <div class="rulebook-select-group">
        <el-select
          :model-value="localName"
          class="rulebook-name-select"
          filterable
          allow-create
          default-first-option
          :disabled="disabled || loading"
          placeholder="选择或输入 rulebook_name"
          @change="handleNameChange"
        >
          <el-option v-for="item in availableNames" :key="item" :label="item" :value="item" />
        </el-select>

        <el-select
          :model-value="localVersion"
          class="rulebook-version-select"
          filterable
          allow-create
          default-first-option
          :disabled="disabled || loading"
          placeholder="选择或输入版本"
          @change="handleVersionChange"
        >
          <el-option v-for="item in versionsForCurrentName" :key="item" :label="item" :value="item" />
        </el-select>
      </div>
    </div>

    <div class="rulebook-action-row">
      <el-button size="small" class="rulebook-action-btn" plain :disabled="disabled || !selectedRuleBook" @click="openEditDialog">
        {{ editActionLabel }}
      </el-button>
      <el-button size="small" class="rulebook-action-btn" type="primary" plain :disabled="disabled" @click="openCreateDialog">
        新建规则书
      </el-button>
      <el-button size="small" text :disabled="disabled || loading" @click="fetchCatalog">刷新</el-button>
    </div>

    <div v-if="selectedRuleBook" class="rulebook-meta">
      <el-tag size="small" :type="selectedRuleBook.source === 'user' ? 'success' : 'info'">
        {{ selectedRuleBook.source === 'user' ? '用户规则书' : '内置规则书' }}
      </el-tag>
      <span>规则数：{{ selectedRuleBook.rule_count }}</span>
      <span class="rulebook-meta-path" :title="selectedRuleBook.path">{{ selectedRuleBook.path }}</span>
    </div>

    <div v-if="writableDir" class="rulebook-hint">
      新建的规则书会保存到：{{ writableDir }}
    </div>
    <div v-if="catalogError" class="rulebook-error">{{ catalogError }}</div>

    <el-dialog v-model="createDialogVisible" title="新建规则书" width="760px" append-to-body>
      <div class="create-grid">
        <label class="field-label">规则书名称</label>
        <el-input v-model="createForm.name" :disabled="creating" placeholder="如 manga-manual" />

        <label class="field-label">版本</label>
        <el-input v-model="createForm.version" :disabled="creating" placeholder="如 v1" />

        <label class="field-label">起始模板</label>
        <el-select v-model="createForm.preset" :disabled="creating" @change="applyPresetTemplate">
          <el-option label="漫画文件模板" value="manga" />
          <el-option label="空白模板" value="blank" />
          <el-option label="无动作模板 (noop)" value="noop" />
        </el-select>

        <label class="field-label">JSON 内容</label>
        <el-input
          v-model="createForm.content"
          type="textarea"
          :disabled="creating"
          :autosize="{ minRows: 12, maxRows: 20 }"
          placeholder="请输入完整 RuleBook JSON"
        />
      </div>

      <div class="rulebook-hint create-hint">
        保存位置：{{ writableDir || '后端 rulebooks 用户目录' }}
      </div>

      <template #footer>
        <div class="footer-actions">
          <el-button :disabled="creating" @click="createDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="creating" @click="submitCreateRuleBook">创建规则书</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="editDialogVisible" :title="editDialogTitle" width="860px" append-to-body>
      <div v-loading="loadingContent" class="create-grid">
        <label class="field-label">规则书信息</label>
        <div class="edit-meta-box">
          <el-tag size="small" :type="editForm.editable ? 'success' : 'info'">
            {{ editForm.editable ? '可编辑用户规则书' : '内置只读规则书' }}
          </el-tag>
          <span>{{ editForm.name || '-' }} @ {{ editForm.version || '-' }}</span>
          <span v-if="editForm.path" class="rulebook-meta-path" :title="editForm.path">{{ editForm.path }}</span>
        </div>

        <label class="field-label">JSON 内容</label>
        <el-input
          v-model="editForm.content"
          type="textarea"
          :disabled="loadingContent || savingContent"
          :readonly="!editForm.editable"
          :autosize="{ minRows: 14, maxRows: 22 }"
          placeholder="规则书 JSON 内容"
        />
      </div>

      <div class="rulebook-hint create-hint">
        <template v-if="editForm.editable">
          保存后会直接覆盖当前用户规则书文件。
        </template>
        <template v-else>
          当前是内置规则书，只能查看；如需修改请先点击“新建规则书”复制出一份用户规则书。
        </template>
      </div>

      <template #footer>
        <div class="footer-actions">
          <el-button :disabled="savingContent" @click="editDialogVisible = false">关闭</el-button>
          <el-button type="primary" :disabled="!editForm.editable" :loading="savingContent" @click="saveEditedRuleBook">保存修改</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'

const props = defineProps({
  name: {
    type: String,
    default: 'noop'
  },
  version: {
    type: String,
    default: 'v1'
  },
  disabled: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:name', 'update:version', 'created'])

const loading = ref(false)
const creating = ref(false)
const loadingContent = ref(false)
const savingContent = ref(false)
const createDialogVisible = ref(false)
const editDialogVisible = ref(false)
const catalogError = ref('')
const writableDir = ref('')
const ruleBookItems = ref([])
const localName = ref('noop')
const localVersion = ref('v1')
const createForm = reactive({
  name: 'custom-rulebook',
  version: 'v1',
  preset: 'manga',
  content: ''
})
const editForm = reactive({
  name: '',
  version: '',
  path: '',
  editable: false,
  content: ''
})

const validItems = computed(() => {
  return ruleBookItems.value.filter((item) => item && item.valid)
})

const availableNames = computed(() => {
  return Array.from(new Set(validItems.value.map((item) => item.name))).sort((a, b) => a.localeCompare(b))
})

const versionsForCurrentName = computed(() => {
  const currentName = normalizeName(localName.value)
  const versions = validItems.value
    .filter((item) => item.name === currentName)
    .map((item) => item.version)
  const unique = Array.from(new Set(versions)).sort((a, b) => a.localeCompare(b))
  if (unique.length > 0) {
    return unique
  }
  return [normalizeVersion(localVersion.value) || 'v1']
})

const selectedRuleBook = computed(() => {
  const currentName = normalizeName(localName.value)
  const currentVersion = normalizeVersion(localVersion.value)
  return validItems.value.find((item) => item.name === currentName && item.version === currentVersion) || null
})

const editActionLabel = computed(() => {
  if (!selectedRuleBook.value) return '编辑规则书'
  return selectedRuleBook.value.editable ? '编辑规则书' : '查看规则书'
})

const editDialogTitle = computed(() => {
  const label = editForm.editable ? '编辑规则书' : '查看规则书'
  const name = editForm.name || normalizeName(localName.value) || 'rulebook'
  const version = editForm.version || normalizeVersion(localVersion.value) || 'v1'
  return `${label}：${name}@${version}`
})

watch(
  () => props.name,
  (value) => {
    localName.value = normalizeName(value) || 'noop'
  },
  { immediate: true }
)

watch(
  () => props.version,
  (value) => {
    localVersion.value = normalizeVersion(value) || 'v1'
  },
  { immediate: true }
)

function normalizeName(value) {
  return String(value || '').trim().toLowerCase()
}

function normalizeVersion(value) {
  return String(value || '').trim().toLowerCase()
}

function emitSelection() {
  emit('update:name', normalizeName(localName.value) || 'noop')
  emit('update:version', normalizeVersion(localVersion.value) || 'v1')
}

function handleNameChange(value) {
  localName.value = normalizeName(value) || 'noop'
  const versions = versionsForCurrentName.value
  if (versions.length && !versions.includes(normalizeVersion(localVersion.value))) {
    localVersion.value = versions[0]
  }
  emitSelection()
}

function handleVersionChange(value) {
  localVersion.value = normalizeVersion(value) || 'v1'
  emitSelection()
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

async function fetchCatalog() {
  loading.value = true
  try {
    const res = await fetch('/api/rulebooks')
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取规则书列表失败'))
    }

    const data = await res.json()
    ruleBookItems.value = Array.isArray(data?.items) ? data.items : []
    writableDir.value = String(data?.writable_dir || '').trim()
    catalogError.value = ''

    if (!normalizeName(localName.value) && availableNames.value.length > 0) {
      localName.value = availableNames.value[0]
    }
    if (!versionsForCurrentName.value.includes(normalizeVersion(localVersion.value))) {
      localVersion.value = versionsForCurrentName.value[0] || 'v1'
    }
    emitSelection()
  } catch (e) {
    catalogError.value = e.message || '获取规则书列表失败'
  } finally {
    loading.value = false
  }
}

async function loadRuleBookContent() {
  const name = normalizeName(localName.value)
  const version = normalizeVersion(localVersion.value)
  if (!name || !version) {
    ElMessage.warning('请先选择规则书')
    return
  }

  loadingContent.value = true
  try {
    const params = new URLSearchParams({ name, version })
    const res = await fetch(`/api/rulebooks/content?${params.toString()}`)
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取规则书内容失败'))
    }

    const data = await res.json()
    editForm.name = data?.item?.name || name
    editForm.version = data?.item?.version || version
    editForm.path = data?.item?.path || ''
    editForm.editable = !!data?.item?.editable
    editForm.content = String(data?.content || '')
  } catch (e) {
    ElMessage.error(e.message || '获取规则书内容失败')
    editDialogVisible.value = false
  } finally {
    loadingContent.value = false
  }
}

async function openEditDialog() {
  if (!selectedRuleBook.value) {
    ElMessage.warning('请先选择一个规则书')
    return
  }
  editDialogVisible.value = true
  await loadRuleBookContent()
}

async function saveEditedRuleBook() {
  if (!editForm.editable) {
    ElMessage.warning('当前规则书为只读，不能直接保存')
    return
  }

  let content
  try {
    content = JSON.parse(String(editForm.content || '{}'))
  } catch (e) {
    ElMessage.error(`规则书 JSON 格式无效: ${e.message}`)
    return
  }

  savingContent.value = true
  try {
    const params = new URLSearchParams({
      name: normalizeName(editForm.name),
      version: normalizeVersion(editForm.version)
    })
    const res = await fetch(`/api/rulebooks/content?${params.toString()}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content })
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '保存规则书失败'))
    }

    await fetchCatalog()
    await loadRuleBookContent()
    ElMessage.success(`规则书 ${editForm.name}@${editForm.version} 已保存`)
    emit('created', selectedRuleBook.value)
  } catch (e) {
    ElMessage.error(e.message || '保存规则书失败')
  } finally {
    savingContent.value = false
  }
}

function buildTemplateObject(name, version, preset) {
  const normalizedName = normalizeName(name) || 'custom-rulebook'
  const normalizedVersion = normalizeVersion(version) || 'v1'

  if (preset === 'noop') {
    return {
      name: normalizedName,
      version: normalizedVersion,
      scan: {
        extensions: ['.iso']
      },
      rules: []
    }
  }

  if (preset === 'os') {
    return {
      name: normalizedName,
      version: normalizedVersion,
      scan: {
        extensions: ['.iso']
      },
      rules: [
        {
          id: 'example-os-rule',
          priority: 10,
          enabled: true,
          match: {
            file_name_contains: ['windows']
          },
          action: {
            target_dir: 'windows',
            rule_type: 'os',
            infer_is_os: true
          }
        }
      ]
    }
  }

  if (preset === 'blank') {
    return {
      name: normalizedName,
      version: normalizedVersion,
      scan: {
        extensions: ['.cbz', '.zip', '.rar', '.7z', '.pdf'],
        directory_rules: []
      },
      rules: []
    }
  }

  return {
    name: normalizedName,
    version: normalizedVersion,
    scan: {
      extensions: ['.cbz', '.zip', '.rar', '.7z', '.pdf'],
      directory_rules: [
        {
          name: 'image-folder',
          extensions: ['.jpg', '.jpeg', '.png', '.webp'],
          min_file_count: 5
        }
      ]
    },
    rules: []
  }
}

function applyPresetTemplate() {
  createForm.content = JSON.stringify(
    buildTemplateObject(createForm.name, createForm.version, createForm.preset),
    null,
    2
  )
}

function openCreateDialog() {
  createForm.name = normalizeName(localName.value) || 'custom-rulebook'
  createForm.version = normalizeVersion(localVersion.value) || 'v1'
  createForm.preset = 'manga'
  applyPresetTemplate()
  createDialogVisible.value = true
}

async function submitCreateRuleBook() {
  const name = normalizeName(createForm.name)
  const version = normalizeVersion(createForm.version)
  if (!name || !version) {
    ElMessage.warning('请先填写规则书名称和版本')
    return
  }

  let content
  try {
    content = JSON.parse(String(createForm.content || '{}'))
  } catch (e) {
    ElMessage.error(`规则书 JSON 格式无效: ${e.message}`)
    return
  }

  creating.value = true
  try {
    const res = await fetch('/api/rulebooks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        version,
        content
      })
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '创建规则书失败'))
    }

    const data = await res.json()
    await fetchCatalog()
    localName.value = data?.item?.name || name
    localVersion.value = data?.item?.version || version
    emitSelection()
    createDialogVisible.value = false
    ElMessage.success(`规则书 ${localName.value}@${localVersion.value} 已创建`)
    emit('created', data?.item || null)
  } catch (e) {
    ElMessage.error(e.message || '创建规则书失败')
  } finally {
    creating.value = false
  }
}

onMounted(() => {
  fetchCatalog()
})
</script>

<style scoped>
.rulebook-selector {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rulebook-selector-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rulebook-select-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.rulebook-action-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding-left: 2px;
}

.rulebook-action-btn {
  min-width: 92px;
}

.rulebook-name-select {
  min-width: 220px;
  flex: 1 1 240px;
}

.rulebook-version-select {
  width: 120px;
}

.rulebook-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #475569;
  flex-wrap: wrap;
}

.rulebook-meta-path {
  max-width: 100%;
  color: #64748b;
  word-break: break-all;
}

.rulebook-hint {
  font-size: 12px;
  color: #64748b;
}

.rulebook-error {
  font-size: 12px;
  color: #dc2626;
}

.create-grid {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr);
  gap: 10px 12px;
  align-items: start;
}

.field-label {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
  padding-top: 8px;
}

.create-hint {
  margin-top: 12px;
}

.edit-meta-box {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 6px;
  color: #475569;
}

.footer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

@media (max-width: 900px) {
  .rulebook-action-row {
    justify-content: flex-start;
  }
}
</style>

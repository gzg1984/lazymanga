<template>
  <div>
    <el-button class="repo-type-manager-btn" plain @click="openDialog">仓库类型</el-button>

    <el-dialog v-model="dialogVisible" title="仓库类型管理" width="980px">
      <div v-loading="loading" class="repo-type-manager">
        <div class="type-list-panel">
          <div class="panel-header">
            <span class="panel-title">模板列表</span>
            <el-button size="small" @click="startCreate">+ 新建模板</el-button>
          </div>

          <div v-if="repoTypes.length === 0" class="empty-state">暂无仓库类型模板</div>
          <button
            v-for="item in repoTypes"
            :key="item.key"
            type="button"
            class="type-card"
            :class="{ active: selectedKey === item.key }"
            @click="selectRepoType(item)"
          >
            <div class="type-card-header">
              <span class="type-name">{{ item.name }}</span>
              <el-tag :type="item.enabled ? 'success' : 'info'" size="small">{{ item.enabled ? '启用中' : '已禁用' }}</el-tag>
            </div>
            <div class="type-key">key: {{ item.key }}</div>
            <div class="type-desc">{{ item.description || '（无描述）' }}</div>
          </button>
        </div>

        <div class="type-editor-panel">
          <div class="panel-header">
            <span class="panel-title">{{ creatingNew ? '新建模板' : '编辑模板' }}</span>
            <el-button size="small" @click="fetchRepoTypes(selectedKey)">刷新</el-button>
          </div>

          <div class="editor-grid">
            <label class="field-label">类型 Key</label>
            <el-input v-model="form.key" :disabled="busy || !creatingNew" placeholder="如 manga-library" />

            <label class="field-label">显示名称</label>
            <el-input v-model="form.name" :disabled="busy" placeholder="如 漫画仓库" />

            <label class="field-label">描述</label>
            <el-input v-model="form.description" :disabled="busy" placeholder="描述模板用途" />

            <label class="field-label">启用状态</label>
            <el-switch v-model="form.enabled" :disabled="busy" inline-prompt active-text="启用" inactive-text="禁用" />

            <label class="field-label">排序</label>
            <el-input-number v-model="form.sortOrder" :disabled="busy" :min="0" :max="999" />

            <label class="field-label">默认 RuleBook</label>
            <RuleBookSelector
              v-model:name="form.rulebookName"
              v-model:version="form.rulebookVersion"
              :disabled="busy"
            />

            <label class="field-label">手动编辑器</label>
            <el-select v-model="form.manualEditorMode" :disabled="busy">
              <el-option label="元数据编辑" value="metadata-editor" />
              <el-option label="旧版类型编辑" value="legacy-type-editor" />
            </el-select>

            <label class="field-label">metadata 展示</label>
            <el-select v-model="form.metadataDisplayMode" :disabled="busy">
              <el-option label="不显示" value="hidden" />
              <el-option label="自动显示识别字段" value="auto" />
              <el-option label="只显示指定字段" value="selected" />
            </el-select>

            <label class="field-label">metadata 字段</label>
            <div>
              <el-input
                v-model="form.metadataDisplayFields"
                :disabled="busy || form.metadataDisplayMode !== 'selected'"
                type="textarea"
                :rows="3"
                placeholder="用逗号或换行分隔，例如 title, series_name, author_name"
              />
              <div class="editor-tip-inline">只在“只显示指定字段”时生效；这些字段也会成为弹窗里可编辑的 metadata 项目。</div>
            </div>

            <label class="field-label">archive 子目录</label>
            <div>
              <el-input v-model="form.archiveSubdir" :disabled="busy" placeholder="例如 archives" />
              <div class="editor-tip-inline">压缩文件默认导入到这个相对路径下。</div>
            </div>

            <label class="field-label">materialized 子目录</label>
            <div>
              <el-input v-model="form.materializedSubdir" :disabled="busy" placeholder="/ 或例如 library" />
              <div class="editor-tip-inline">填 `/` 表示仓库根目录本身；普通扫描会自动跳过 archive 子目录。</div>
            </div>
          </div>

          <div class="settings-box">
            <div class="settings-title">默认行为</div>
            <div class="settings-grid">
              <el-checkbox v-model="form.addButton" :disabled="busy">允许添加文件</el-checkbox>
              <el-checkbox v-model="form.addDirectoryButton" :disabled="busy">允许添加目录</el-checkbox>
              <el-checkbox v-model="form.deleteButton" :disabled="busy">允许删除</el-checkbox>
              <el-checkbox v-model="form.autoNormalize" :disabled="busy">自动归类</el-checkbox>
              <el-checkbox v-model="form.showMD5" :disabled="busy">显示 MD5</el-checkbox>
              <el-checkbox v-model="form.showSize" :disabled="busy">显示大小</el-checkbox>
              <el-checkbox v-model="form.singleMove" :disabled="busy">允许单条移动</el-checkbox>
            </div>
          </div>

          <div class="editor-tip">
            `repo type` 是模板；后续每个仓库可在自己的 overlay 中覆盖这些默认项。
          </div>
        </div>
      </div>

      <template #footer>
        <div class="footer-actions">
          <div>
            <el-button type="danger" :disabled="creatingNew || busy || !form.key" :loading="deleting" @click="deleteRepoType">删除/禁用</el-button>
          </div>
          <div class="footer-right-actions">
            <el-button :disabled="busy" @click="dialogVisible = false">关闭</el-button>
            <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="saveRepoType">{{ creatingNew ? '创建模板' : '保存修改' }}</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import emitter from '../eventBus'
import RuleBookSelector from './RuleBookSelector.vue'
import { filterVisibleRepoTypes } from '../utils/repoTypeVisibility'
import { stringifyMetadataDisplayFields } from '../utils/repoMetadataDisplay'

const dialogVisible = ref(false)
const loading = ref(false)
const submitting = ref(false)
const deleting = ref(false)
const repoTypes = ref([])
const selectedKey = ref('')
const creatingNew = ref(false)

const form = reactive(createEmptyForm())

const busy = computed(() => loading.value || submitting.value || deleting.value)
const canSubmit = computed(() => {
  return String(form.key || '').trim() !== '' && String(form.name || '').trim() !== ''
})

function createEmptyForm() {
  return {
    key: '',
    name: '',
    description: '',
    enabled: true,
    sortOrder: 50,
    addButton: true,
    addDirectoryButton: false,
    deleteButton: true,
    autoNormalize: false,
    showMD5: true,
    showSize: true,
    singleMove: true,
    manualEditorMode: 'legacy-type-editor',
    metadataDisplayMode: 'hidden',
    metadataDisplayFields: '',
    archiveSubdir: 'archives',
    materializedSubdir: '/',
    rulebookName: 'noop',
    rulebookVersion: 'v1'
  }
}

function applyForm(next) {
  Object.assign(form, createEmptyForm(), next || {})
}

async function parseErrorMessage(res, fallback) {
  try {
    const data = await res.clone().json()
    if (data && data.error) {
      return `${fallback}: ${data.error}`
    }
    if (data && data.message) {
      return `${fallback}: ${data.message}`
    }
  } catch (_) {
    // ignore json parse failures
  }

  try {
    const text = await res.text()
    if (text) {
      return `${fallback}: ${text}`
    }
  } catch (_) {
    // ignore text parse failures
  }

  return `${fallback} (HTTP ${res.status})`
}

function selectRepoType(item) {
  creatingNew.value = false
  selectedKey.value = item?.key || ''
  applyForm({
    key: item?.key || '',
    name: item?.name || '',
    description: item?.description || '',
    enabled: !!item?.enabled,
    sortOrder: Number(item?.sort_order || 0),
    addButton: !!item?.add_button,
    addDirectoryButton: !!item?.add_directory_button,
    deleteButton: !!item?.delete_button,
    autoNormalize: !!item?.auto_normalize,
    showMD5: !!item?.show_md5,
    showSize: !!item?.show_size,
    singleMove: !!item?.single_move,
    manualEditorMode: item?.manual_editor_mode || 'legacy-type-editor',
    metadataDisplayMode: item?.metadata_display_mode || 'hidden',
    metadataDisplayFields: item?.metadata_display_fields || '',
    archiveSubdir: item?.archive_subdir || 'archives',
    materializedSubdir: item?.materialized_subdir || '/',
    rulebookName: item?.rulebook_name || 'noop',
    rulebookVersion: item?.rulebook_version || 'v1'
  })
}

function startCreate() {
  creatingNew.value = true
  selectedKey.value = ''
  applyForm(createEmptyForm())
}

async function fetchRepoTypes(preferKey = '') {
  loading.value = true
  try {
    const res = await fetch('/api/repo-types?include_disabled=true')
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '获取仓库类型失败'))
    }

    const data = await res.json()
    repoTypes.value = filterVisibleRepoTypes(Array.isArray(data?.items) ? data.items : [])

    const targetKey = preferKey || selectedKey.value
    const matched = repoTypes.value.find((item) => item.key === targetKey)
    if (matched) {
      selectRepoType(matched)
    } else if (repoTypes.value.length > 0 && !creatingNew.value) {
      selectRepoType(repoTypes.value[0])
    } else if (repoTypes.value.length === 0) {
      startCreate()
    }
  } catch (e) {
    ElMessage.error(e.message || '获取仓库类型失败')
  } finally {
    loading.value = false
  }
}

async function saveRepoType() {
  if (!canSubmit.value) {
    ElMessage.warning('请先填写类型 Key 和显示名称')
    return
  }

  submitting.value = true
  try {
    const body = {
      key: String(form.key || '').trim(),
      name: String(form.name || '').trim(),
      description: String(form.description || '').trim(),
      enabled: !!form.enabled,
      sort_order: Number(form.sortOrder || 0),
      add_button: !!form.addButton,
      add_directory_button: !!form.addDirectoryButton,
      delete_button: !!form.deleteButton,
      auto_normalize: !!form.autoNormalize,
      show_md5: !!form.showMD5,
      show_size: !!form.showSize,
      single_move: !!form.singleMove,
      manual_editor_mode: String(form.manualEditorMode || '').trim() || 'legacy-type-editor',
      metadata_display_mode: String(form.metadataDisplayMode || '').trim() || 'hidden',
      metadata_display_fields: stringifyMetadataDisplayFields(form.metadataDisplayFields),
      archive_subdir: String(form.archiveSubdir || '').trim() || 'archives',
      materialized_subdir: String(form.materializedSubdir || '').trim() || '/',
      rulebook_name: String(form.rulebookName || '').trim(),
      rulebook_version: String(form.rulebookVersion || '').trim()
    }

    const isCreate = creatingNew.value
    const url = isCreate ? '/api/repo-types' : `/api/repo-types/${encodeURIComponent(body.key)}`
    const method = isCreate ? 'POST' : 'PUT'

    const res = await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, isCreate ? '创建仓库类型失败' : '保存仓库类型失败'))
    }

    const saved = await res.json()
    await fetchRepoTypes(saved?.key || body.key)
    emitter.emit('repo-types-updated')
    ElMessage.success(isCreate ? '仓库类型已创建' : '仓库类型已保存')
  } catch (e) {
    ElMessage.error(e.message || '保存仓库类型失败')
  } finally {
    submitting.value = false
  }
}

async function deleteRepoType() {
  const key = String(form.key || '').trim()
  if (!key) return

  const ok = window.confirm(`确认删除或禁用仓库类型 ${key} 吗？`)
  if (!ok) return

  deleting.value = true
  try {
    const res = await fetch(`/api/repo-types/${encodeURIComponent(key)}`, { method: 'DELETE' })
    if (!res.ok) {
      throw new Error(await parseErrorMessage(res, '删除仓库类型失败'))
    }

    if (res.status !== 204) {
      const data = await res.json().catch(() => null)
      ElMessage.success(data?.message || '仓库类型已处理')
    } else {
      ElMessage.success('仓库类型已删除')
    }

    startCreate()
    await fetchRepoTypes('')
    emitter.emit('repo-types-updated')
  } catch (e) {
    ElMessage.error(e.message || '删除仓库类型失败')
  } finally {
    deleting.value = false
  }
}

function openDialog() {
  dialogVisible.value = true
  fetchRepoTypes(selectedKey.value)
}
</script>

<style scoped>
.repo-type-manager-btn {
  border-radius: 999px;
}

.repo-type-manager {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 16px;
  min-height: 420px;
}

.type-list-panel,
.type-editor-panel {
  border: 1px solid #dbe3ee;
  border-radius: 12px;
  background: #fff;
  padding: 12px;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 12px;
}

.panel-title {
  font-size: 14px;
  font-weight: 700;
  color: #334155;
}

.empty-state {
  color: #64748b;
  font-size: 13px;
}

.type-card {
  width: 100%;
  text-align: left;
  border: 1px solid #dbe3ee;
  border-radius: 10px;
  background: #f8fafc;
  padding: 10px;
  margin-bottom: 10px;
}

.type-card.active {
  border-color: #60a5fa;
  background: #eff6ff;
}

.type-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.type-name {
  font-weight: 700;
  color: #1e293b;
}

.type-key {
  font-size: 12px;
  color: #475569;
  margin-bottom: 4px;
}

.type-desc {
  font-size: 12px;
  color: #64748b;
  line-height: 1.5;
}

.editor-grid {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
}

.field-label {
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.rulebook-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 90px;
  gap: 8px;
}

.settings-box {
  margin-top: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 12px;
  background: #f8fafc;
}

.settings-title {
  margin-bottom: 10px;
  font-size: 13px;
  font-weight: 700;
  color: #334155;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 12px;
}

.editor-tip {
  margin-top: 12px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.6;
}

.editor-tip-inline {
  margin-top: 6px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.footer-actions {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.footer-right-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>

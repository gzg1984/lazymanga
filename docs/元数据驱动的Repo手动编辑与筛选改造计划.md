# 元数据驱动的 Repo 手动编辑与筛选改造计划

> 更新时间：2026-04-05
> 目标：让“手动修改”与“元素列表筛选”不再依赖全局写死模板，而是跟随 repo 类型 / rulebook / 文件名识别规则输出的元数据工作。

---

## 1. 问题现状

当前系统已经具备：

- `repo type` / `repo overlay`：决定按钮、自动规范化、默认规则书等
- `rulebook`：决定扫描哪些文件/目录，以及目录重命名与 sidecar 生成规则
- `filename_rules`：可以从目录名 / 文件名提取复杂元数据

但还有两个关键缺口：

1. **手动修改框仍是全局写死模板**
   - 只支持 `os / entertainment / others`
   - 无法针对漫画仓库展示 `title / 汉化组 / 作者 / 原作` 等字段

2. **元数据只写入 sidecar JSON，没有正式入库**
   - 列表页无法稳定按这些字段筛选
   - 手动修改框也无法以元数据表单方式回显和编辑

---

## 2. 总体设计目标

改造成：

- **repo 决定编辑模式**（而不是全软件一套 UI）
- **rulebook / filename recognizer 决定有哪些可识别字段**
- **识别结果同时写入 sidecar 与 repo.db**
- **元素列表页和手动修改框复用同一套元数据 schema**

一句话：

> 规则书负责“识别与规范化”，repo 配置负责“UI 如何展示和编辑这些结果”。

---

## 3. 分阶段落地方案

### Phase 1：元数据正式入库 + 列表页可筛选（本次先做）

**目标**

- 给 `RepoISO` 增加 `metadata_json`
- 在 `directory-transform` / `refresh` / `增量规范化` 中把识别出的 metadata 同步入库
- `GET /api/repos/:id/repoisos` 返回解析后的 `metadata`
- 列表页支持按 metadata key/value 做筛选

**本阶段不做**

- 手动修改框改成元数据驱动表单
- 编辑 metadata 后自动回写目录名

**验收标准**

- `karita-manga` 目录经过规范化后，数据库里可看到 metadata
- 元素列表页能按 `scanlator_group / author_name / original_work / title` 等字段筛选
- 不依赖读取 sidecar 文件来筛选

---

### Phase 2：手动修改框切换为 repo 驱动模式

**目标**

- repo 设置中增加编辑模式，例如：
  - `legacy-type-editor`（旧版 OS/娱乐/其他）
  - `metadata-editor`（元数据表单）
- 漫画 repo 打开“手动修改”时，显示 metadata 字段而不是固定类型选择
- 同一组件根据 repo 类型动态渲染不同编辑器

**验收标准**

- `karita-manga` 仓库打开手动修改时，显示元数据字段
- `os` 仓库仍保留原有类型编辑体验

---

### Phase 3：元数据编辑回写规范化结果

**目标**

- 修改 `title / author / original_work` 等字段后：
  - 更新 `metadata_json`
  - 更新 sidecar JSON
  - 按 rulebook 模板重新生成目录名 / 路径

**验收标准**

- 在手动修改框里修改 metadata 后，目录名和 sidecar 能同步变化
- 不会破坏现有 repo item 引用关系

---

## 4. 技术原则

### 4.1 sidecar 继续保留，但不再作为唯一数据源

- sidecar (`.karita.meta.json`)：保留，便于文件系统自描述
- `repo.db.repoisos.metadata_json`：新增，作为 UI / 筛选 / 编辑主数据源

### 4.2 UI schema 应尽量配置化

后续建议在 repo type 或 rulebook 中增加类似配置：

```json
{
  "ui": {
    "editor_mode": "metadata",
    "fields": [
      { "key": "title", "label": "标题", "editable": true, "filterable": true },
      { "key": "scanlator_group", "label": "汉化组", "editable": true, "filterable": true },
      { "key": "author_name", "label": "作者", "editable": true, "filterable": true },
      { "key": "original_work", "label": "原作", "editable": true, "filterable": true }
    ]
  }
}
```

本次 Phase 1 先不引入完整 schema，只先把 metadata 入库并给列表页做动态 key/value 筛选。

---

## 5. 预计改动文件范围

### 后端

- `backend/models/repoiso.go`
- `backend/normalization/directory_transform_step.go`
- `backend/normalization/pipeline.go`
- `backend/handlers/repoisos.go`
- `backend/handlers/repoiso_refresh.go`
- `backend/handlers/repo_normalize.go`

### 前端

- `ui/src/components/RepoIsoTable.vue`
- 后续 Phase 2 再改 `ui/src/components/RepoManualEditDialog.vue`

---

## 6. 当前实施边界（避免跑偏）

本轮 **只做 Phase 1**，明确边界如下：

- ✅ 做：metadata 入库
- ✅ 做：列表页展示/筛选 metadata
- ✅ 做：refresh / normalize 时回填 metadata
- ❌ 不做：完整的 metadata 编辑表单
- ❌ 不做：repo type 配置化 editor schema
- ❌ 不做：metadata 改动后的目录重命名回写

如果后续继续推进，就按本文件 Phase 2 → Phase 3 顺序执行，避免同时改 UI、规则书、路径回写，导致链路混乱。

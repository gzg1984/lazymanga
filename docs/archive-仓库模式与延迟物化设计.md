# Archive 仓库模式与延迟物化设计

> 更新时间：2026-04-22
> 目标：在不破坏当前“漫画默认放在仓库根目录”使用习惯的前提下，为 zip/rar/7z/cbz/cbr 等压缩包元素建立一套可实现、可扩展、低风险的管理模型。

---

## 1. 背景与问题

当前系统对“目录型漫画元素”的支持已经较完整：

- `repo type` / `repo overlay`：决定仓库默认行为；
- `rulebook`：决定扫描范围、识别规则与目录变换；
- `RepoISO.metadata_json`：可以承载结构化元数据；
- 手动编辑、筛选、自动规范化等流程已经围绕 repo 级配置工作。

但对 archive 类型漫画元素，还缺少一套明确边界：

1. 压缩包通常不需要立即解压；
2. 用户希望压缩包能被纳入统一管理，而不是原样堆放；
3. 又不希望为了管理而频繁改写 zip/rar 文件本体；
4. 当前许多仓库已经默认把“所有漫画都放在仓库根目录”当作直观模型，不适合强制切换到全新目录布局。

因此，本设计的核心任务不是“让 archive 立刻变成目录型元素”，而是：

> 把 archive 视为默认不可变、可识别、可托管、可延迟物化的一类仓库元素。

---

## 2. 结论与总体方向

本设计选择以下方向：

- **默认不改写 archive 文件本体**；
- **archive 元数据默认外置存储**，主存放位置为 repo.db；
- **archive 归入固定的 archive 子路径管理**；
- **已整理内容的物化区 (`materialized_subdir`) 默认兼容当前根目录模式**；
- **archive 与 materialized 内容在扫描与写入阶段显式隔离**；
- **“解压整理 / 重打包 / 写回可移植元数据”是显式工作流，不是默认扫描行为。**

一句话：

> 先把 archive 管起来，再决定什么时候把它真正变成“物理整理后的内容”。

---

## 3. 设计原则

### 3.1 文件本体尽量不可变

- 默认情况下，不向 zip/rar/7z 写入自定义 metadata；
- 不把“改写压缩包”作为 archive 管理的前提；
- 后续如果要支持“导出 metadata 到压缩包”或“重打包固化结果”，应作为显式动作提供。

原因：

- zip/rar 写回存在损坏风险；
- 不同压缩格式的读写能力并不一致；
- repo.db 中的结构化 metadata 更适合筛选、编辑与后续演化。

### 3.2 逻辑整理优先于物理整理

- archive 可以先拥有逻辑上的标题、作者、系列、标签、状态；
- UI 可以展示“整理后的逻辑名称”，而不强制立即改动压缩包；
- 物理整理延后到用户显式确认时再执行。

### 3.3 保持当前根目录直觉

- `materialized_subdir` 默认值不是 `library`，而是“仓库根目录本身”；
- 这意味着当前大多数仓库无需迁移，也无需改变“根目录就是漫画库”的理解方式；
- 同时保留将来切换为 `library` 等子目录模式的能力。

### 3.4 archive 必须有明确隔离边界

- archive 统一存放在 `archive_subdir` 下；
- materialized 扫描、自动规范化、目录变换，必须显式避开 `archive_subdir`；
- archive 扫描、archive 操作，也必须只在 `archive_subdir` 下进行。

这条边界比“默认值到底是 `/` 还是 `library`”更重要。

---

## 4. 术语定义

### 4.1 archive element

指 zip/rar/7z/cbz/cbr 等压缩包元素。其特点是：

- 默认不可变；
- 可以有 metadata；
- 可以被扫描和筛选；
- 可以在未来进入“物化”流程。

### 4.2 materialized content

指已经在文件系统上按普通漫画库方式存在、可被目录型规则书处理的内容。它可以是：

- 原本就放在仓库中的普通目录；
- 某个 archive 后续被解压并整理后的结果。

### 4.3 archive root

archive 的固定管理根目录：

```text
archive_root = repo.root_path + archive_subdir
```

### 4.4 materialized root

materialized 内容的扫描与写入根目录：

```text
materialized_root = repo.root_path + materialized_subdir
```

当 `materialized_subdir` 为空或表示 `/` 时，`materialized_root` 就是 `repo.root_path` 本身。

---

## 5. 仓库级配置设计

本设计不新起一套 archive 专用配置系统，而是复用现有 `repo type template + repo overlay` 模型。

建议新增的 repo 级设置如下：

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `archive_subdir` | string | `archives` | archive 固定存放子目录，相对 repo root |
| `materialized_subdir` | string | `/` | materialized 内容根目录；`/` 表示 repo root 本身 |
| `archive_extensions` | string[] | `.zip,.rar,.7z,.cbz,.cbr` | archive 仓库默认扫描扩展名 |
| `archive_metadata_mode` | string | `external-only` | archive metadata 默认仅外置存储 |
| `archive_import_mode` | string | `move` 或 `copy` | 导入 archive 时如何进入 archive root |
| `archive_materialize_mode` | string | `manual` | 是否只允许显式手动物化 |
| `archive_read_inner_layout` | bool | `true` | 是否允许轻量读取包内结构摘要 |

说明：

- `archive_subdir` 与 `materialized_subdir` 应作为 repo overlay 可覆盖项；
- 模板负责提供默认值；
- 仓库自己的 overlay 只记录差异项。

示例：

```json
{
  "archive_subdir": "archives",
  "materialized_subdir": "/",
  "archive_extensions": [".zip", ".rar", ".7z", ".cbz", ".cbr"],
  "archive_metadata_mode": "external-only",
  "archive_import_mode": "move",
  "archive_materialize_mode": "manual",
  "archive_read_inner_layout": true
}
```

### 5.1 Phase 1 最小配置合同

为避免第一阶段一开始就扩展过多设置，建议先把下面四项视为最小必需合同：

| 字段 | 是否必做 | 说明 |
| --- | --- | --- |
| `archive_subdir` | 是 | archive 的唯一受管根目录 |
| `materialized_subdir` | 是 | materialized 扫描与写入根目录 |
| `archive_extensions` | 是 | archive 扫描白名单 |
| `archive_read_inner_layout` | 否 | 允许包内浅层结构摘要，必要时可在首版先关闭 |

其余字段如 `archive_import_mode`、`archive_metadata_mode`、`archive_materialize_mode` 可以先在文档中保留，但在第一阶段允许只作为设计占位，不强制全部落地到 UI。

### 5.2 配置解析合同

建议在后端形成一层明确的“archive 生效配置”解析结果，而不是在各个 handler 中零散解析 JSON 字段。

建议的解析结果结构可表达为：

```json
{
  "archive_subdir": "archives",
  "materialized_subdir": "/",
  "archive_root": "<repo-root>/archives",
  "materialized_root": "<repo-root>",
  "archive_extensions": [".zip", ".rar", ".7z", ".cbz", ".cbr"],
  "exclude_roots": ["<repo-root>/archives"]
}
```

关键要求：

- archive 相关路径只在一处统一归一化；
- 其它扫描、refresh、normalize 逻辑只消费解析后的最终结果；
- `materialized_subdir = /` 时，`exclude_roots` 必须明确包含 `archive_root`。

---

## 6. 默认目录布局

本设计正式支持两种布局。

### 6.1 兼容模式（推荐默认）

```text
repo_root/
  archives/
  <普通漫画目录或文件>
```

配置含义：

- `materialized_subdir = /`
- `archive_subdir = archives`

特点：

- 保持“仓库根目录就是主漫画库”的直觉；
- archive 被收敛到单独子树；
- 普通扫描以 repo root 为范围，但显式排除 `archives/`。

### 6.2 隔离模式（长期推荐）

```text
repo_root/
  archives/
  library/
```

配置含义：

- `materialized_subdir = library`
- `archive_subdir = archives`

特点：

- archive 与 materialized 内容天然隔离；
- 扫描边界和后续自动整理实现更简单；
- 适合新建仓库或 archive 占比较高的仓库。

### 6.3 默认值判断

当前版本建议：

- **默认值采用兼容模式**；
- **架构上完整支持隔离模式**；
- 不要求现有仓库立即迁移到 `library`。

原因：

- 当前产品已形成“所有漫画都在根目录”的使用习惯；
- 立刻强推 `library` 会引入不必要的迁移与认知成本；
- 但未来新仓库或高级用户，仍然应能选择更规整的布局。

---

## 7. 路径与配置归一化规则

### 7.1 `materialized_subdir = /` 的内部语义

不建议在后端真正把 `/` 当作路径拼接值处理，而建议做如下归一化：

- UI / 文档层允许用户看到 `/`；
- 配置解析层把 `/` 视为“空偏移”；
- 最终计算时：
  - `materialized_root = repo.root_path`
  - `archive_root = repo.root_path + archive_subdir`

### 7.2 路径合法性约束

建议增加统一校验：

1. `archive_subdir` 不能为空，也不能是 `/`；
2. `archive_subdir` 必须是 repo root 下的相对路径；
3. `materialized_subdir` 允许是 `/` 或 repo root 下的相对路径；
4. 当 `materialized_subdir != /` 时，`archive_subdir` 与 `materialized_subdir` 不能互相包含；
5. 所有实际拼接结果都必须再次校验仍位于 repo root 内。

### 7.3 根目录兼容模式下的扫描排除规则

当 `materialized_subdir = /` 时：

- materialized 扫描范围是 repo root；
- 但必须显式排除 `archive_root` 子树；
- 任何自动重命名、目录变换、目录型 refresh，都不得进入 `archive_root`。

这是兼容模式成立的前提。

### 7.4 建议的辅助函数边界

后续编码时，建议尽量把 archive 路径行为收束为少量统一辅助函数，而不是分散在各个 handler 里直接拼路径。建议至少有：

- `NormalizeArchiveSubdir(raw string) (string, error)`
- `NormalizeMaterializedSubdir(raw string) (string, error)`
- `ResolveArchivePaths(repoRoot string, settings ...) (...)`
- `IsPathUnderArchiveRoot(absPath string, archiveRoot string) bool`
- `ShouldSkipForMaterializedScan(absPath string, archiveRoot string, materializedRoot string) bool`

这不是要求函数名必须完全一致，而是要求职责边界必须稳定。

---

## 8. 数据模型建议

### 8.1 继续复用 `RepoISO`

archive 元素在第一阶段不建议单独新建一张表，而建议继续复用 `RepoISO`，并通过 metadata 区分元素类型。

原因：

- 当前列表、筛选、刷新、repo 级管理能力已经围绕 `RepoISO` 工作；
- 先把 archive 作为一种特殊 repo item，更利于最小落地；
- 真正需要拆表，应等 archive 行为明显复杂化后再考虑。

### 8.2 建议新增或规范的 metadata 字段

archive 元素建议在 `RepoISO.metadata_json` 中使用以下字段：

| 字段 | 说明 |
| --- | --- |
| `item_kind` | 固定为 `archive` |
| `archive_format` | `zip` / `rar` / `7z` / `cbz` / `cbr` |
| `lifecycle` | `ingested` / `recognized` / `managed` / `materialized` |
| `metadata_status` | `weak` / `confirmed` / `edited` |
| `display_title` | UI 展示用逻辑标题 |
| `series_name` | 系列名 |
| `author_name` | 作者 |
| `author_alias` | 作者别名 |
| `scanlator_group` | 汉化组 / 社团 |
| `original_work` | 原作 |
| `source_path` | 导入前来源路径 |
| `archive_storage_path` | 当前 archive 在 archive root 下的相对路径 |
| `logical_target_path` | 若未来物化，期望落入的 materialized 相对路径 |
| `inner_layout` | `single-root` / `flat` / `multi-root` / `unknown` |
| `materialized` | 是否已进入物化流程 |
| `materialized_path` | 若已物化，对应 materialized 相对路径 |

示例：

```json
{
  "item_kind": "archive",
  "archive_format": "zip",
  "lifecycle": "managed",
  "metadata_status": "edited",
  "display_title": "作品名 第01卷",
  "series_name": "作品名",
  "author_name": "作者",
  "scanlator_group": "汉化组",
  "source_path": "incoming/abc.zip",
  "archive_storage_path": "archives/作品名/abc.zip",
  "logical_target_path": "作品名/第01卷",
  "inner_layout": "single-root",
  "materialized": false
}
```

### 8.4 第一阶段建议最少写入的 archive metadata

为避免首版 metadata 字段过多，第一阶段建议至少写入：

- `item_kind = archive`
- `archive_format`
- `lifecycle = ingested | recognized | managed`
- `archive_storage_path`
- `display_title` 或回退为原文件名
- `inner_layout`（如果实现了轻量读取）

其余如 `logical_target_path`、`materialized_path`、`metadata_status` 可以在第二阶段补齐。

这样首版就能支持：

- 正确区分 archive 与普通元素；
- 正确显示 archive 所在位置；
- 为后续 metadata 编辑预留基础字段。

### 8.3 三类路径必须区分

archive 设计中必须避免把“当前文件位置”和“未来整理目标”混为一谈。至少应明确区分：

1. `source_path`
   - 导入前来源位置；
   - 用于回溯和审计。

2. `archive_storage_path`
   - archive 当前在 archive root 下的实际相对路径；
   - 用于日常管理。

3. `logical_target_path`
   - 若后续物化，期望进入 materialized 区时的逻辑目标路径；
   - 用于 UI 展示与后续执行。

---

## 9. archive 生命周期模型

建议把 archive 看成状态流转对象，而不是“是否已整理”的二元状态。

### 9.1 `ingested`

- archive 已进入 `archive_root`；
- 已建立基础索引；
- 可能尚未做识别。

### 9.2 `recognized`

- 已完成轻量识别；
- 已有初步 metadata；
- 尚未人工确认。

### 9.3 `managed`

- metadata 已人工修正或确认；
- 可以稳定筛选、搜索、展示；
- 文件本体仍默认不可变。

### 9.4 `materialized`

- 用户已显式触发后续动作；
- 可能已经解压整理到 materialized 区；
- 也可能已经经过“解压 -> 重组 -> 重压缩”流程。

说明：

- 第一阶段实现不要求完整打通 `materialized` 行为；
- 但 metadata 模型与 UI 文案应预留此状态。

---

## 10. 扫描与规则书行为

### 10.1 archive 扫描

archive 仓库或 archive 模式下的扫描，应只在 `archive_root` 中进行，依据 `archive_extensions` 或绑定 rulebook 的 scan spec 识别文件。

建议行为：

- 只扫描符合扩展名的压缩包；
- 默认不做目录重命名；
- 默认不写入压缩包内部 metadata；
- 允许轻量读取包内文件列表、根目录层级等信息，形成 `inner_layout` 等摘要字段。

### 10.2 materialized 扫描

materialized 扫描应遵守以下规则：

- 范围只在 `materialized_root`；
- 当 `materialized_subdir = /` 时，必须排除 `archive_root`；
- 目录型 rulebook、目录变换、sidecar 逻辑，只对 materialized 内容生效。

### 10.3 不应发生的行为

以下行为应在设计上显式禁止：

- 普通目录型规范化误处理 archive 文件；
- archive refresh 把 materialized 区目录重新识别成 archive；
- 自动整理任务把 archive 文件搬入 materialized 区；
- 根目录兼容模式下，archive 子树被普通全仓扫描重复索引。

### 10.4 第一阶段扫描实现提示

第一阶段不追求一次性重构所有扫描链路，建议优先守住以下提示：

1. `pre-add` 浏览阶段应能识别 archive 扩展名，但不要把 archive 和普通目录混成同一类可操作对象。
2. repo 全量或增量扫描时，目录型扫描逻辑必须先拿到 `exclude_roots`，再执行 walk。
3. archive 扫描最好是独立入口或独立分支，不要在目录型 transform 流程中临时插入“如果是 zip 就特殊处理”的零散分支。
4. Phase 1 先做到“识别和隔离”即可，不必把 archive 纳入现有目录 rename/sidecar 流程。

---

## 11. 导入与存储策略

### 11.1 archive 导入

导入 archive 时，建议支持以下模式：

- `move`：移动到 `archive_root`；
- `copy`：复制到 `archive_root`；
- `link`：后续可选，不作为第一阶段必做能力。

无论来源为何，系统内受管 archive 的最终管理位置都应位于 `archive_root` 下。

### 11.2 archive 收纳原则

本设计建议把 `archive_subdir` 视为**强约束边界**，而不是“推荐放这里”。

即：

- 要么 archive 还未纳入系统管理；
- 要么一旦纳入管理，就应被放进 `archive_root`；
- 避免仓库内到处散落受管 archive，导致后续扫描、筛选与迁移失控。

---

## 12. UI 与交互建议

### 12.1 列表展示

archive 元素在列表中建议明确显示为独立类型，而不要伪装成普通目录元素。

建议展示：

- 类型：`Archive`
- 格式：`zip` / `rar` / `7z`
- 逻辑标题：`display_title`
- 实际存放路径：`archive_storage_path`
- 生命周期：`ingested/recognized/managed/materialized`
- 识别状态：`weak/confirmed/edited`

### 12.2 操作按钮

archive 元素建议默认提供：

- 编辑 metadata
- 打开文件所在目录
- 移动到 archive 规范位置
- 标记已确认
- 触发物化

archive 元素默认不应提供或默认隐藏：

- 普通目录型 rename / relocate 行为
- 目录 sidecar 回写行为
- 目录型强制恢复原始路径行为

### 12.3 逻辑标题与实际路径分离

UI 应明确区分：

- `display_title`：用于阅读和筛选；
- `archive_storage_path`：用于文件定位；
- `logical_target_path`：用于说明未来物化结果。

不要用一个字段同时承担这三层含义。

### 12.4 Phase 1 UI 最小提示要求

第一阶段即使暂不完整实现 archive 专属 UI，也建议至少补上以下提示信息：

- 仓库设置页：明确显示 `archive_subdir` 与 `materialized_subdir` 的当前值；
- 当 `materialized_subdir = /` 时，显式提示“普通扫描将自动跳过 archive 子目录”；
- archive 元素若出现在列表中，应至少显示一个明确类型标记，避免被误认为普通目录元素；
- 如果用户尝试对 archive 元素执行目录型操作，UI 应隐藏该操作或给出不可用说明。

---

## 13. 与现有系统的对接策略

### 13.1 与 `repo type` / `repo overlay` 的关系

archive 模式应作为 repo type 模板的一种能力组合，而不是旁路系统。

建议新增一个 archive 相关 repo type，例如：

- `archive-manga`

其默认行为可包括：

- `add_button = true`
- `add_directory_button = false`
- `auto_normalize = false`
- `manual_editor_mode = metadata-editor`
- `metadata_display_mode = selected`
- 默认绑定 archive 专用 rulebook

### 13.2 与 `RepoISO.metadata_json` 的关系

archive 的业务特征先通过 metadata 字段落在现有 `RepoISO` 中，避免第一阶段扩大模型复杂度。

### 13.3 与 rulebook 的关系

建议为 archive 提供专用 scan 配置与识别规则，但不强求第一阶段实现复杂的包内识别 DSL。

第一阶段可先做到：

- 指定 archive 扩展名；
- 识别文件格式；
- 读取包内浅层目录结构摘要；
- 允许根据文件名做 metadata 推断。

---

## 14. 分阶段落地方案

### Phase 1：配置与边界落地（最小实现）

目标：

- 在 repo type / overlay 中增加 `archive_subdir`、`materialized_subdir` 等设置；
- 路径归一化支持 `materialized_subdir = /`；
- materialized 扫描显式排除 `archive_root`；
- archive 扫描只在 `archive_root` 下生效；
- `RepoISO.metadata_json` 可以标记 `item_kind = archive`。

验收标准：

- 现有根目录型仓库不需要迁移；
- archive 被统一识别并限定在 `archive_root`；
- 普通目录型 normalize 不会误伤 archive。

建议代码落点：

- `backend/models/repo_info.go`
  - 当前 `settings_override_json` 可承载 archive 配置，无需第一阶段新增单独表。
- `backend/handlers/repo_types.go`
  - 增加 archive 相关默认模板字段、overlay 解析、输入校验。
- `backend/handlers/repo_path.go`
  - 与 repo 根路径配置界面联动时，可补充 archive/materialized 相对路径展示或校验提示。
- `backend/handlers/preaddlist.go`
  - 基于 repo 生效配置，决定预添加列表中哪些 archive 文件可见。
- `backend/normalization/...`
  - 全量/增量扫描入口在目录 walk 前应用 `exclude_roots`；archive 扫描保持独立分支。

建议测试落点：

- `materialized_subdir = /` 时，普通扫描跳过 `archive_subdir`；
- `materialized_subdir = library` 时，只扫描 `library/` 且不触碰 `archives/`；
- `archive_subdir` 非法值会被拒绝；
- archive 与普通目录不会被重复索引成两类元素。

### Phase 2：archive 列表与 metadata 管理

目标：

- 列表页清晰区分 archive 与普通目录元素；
- archive 支持 metadata 编辑、筛选、状态流转；
- 支持 `display_title`、`logical_target_path` 等逻辑字段展示。

验收标准：

- 用户可以在不解压的前提下管理 archive；
- 搜索、筛选、手动修正 metadata 可正常工作。

### Phase 3：archive 物化流程

目标：

- 提供显式 `materialize archive` 动作；
- 支持 archive 解压到 materialized 区；
- 允许后续追加“重打包”或“导出 metadata”能力。

验收标准：

- 物化流程与普通扫描/刷新行为解耦；
- 用户可以选择继续保留原 archive，或与物化内容建立关联关系。

---

## 15. 第一阶段明确不做的内容

为避免设计跑偏，第一阶段明确不做：

- 直接向 zip/rar 写入复杂 metadata；
- 自动解压全部 archive；
- 自动把 archive 重打包为规范格式；
- 单独新建 archive 专用数据库表；
- 包内深度内容识别与图片级索引；
- 从根目录兼容模式自动迁移到 `library` 的全自动迁移器。

这些能力可以在后续阶段逐步追加，但不应成为当前落地的前置条件。

---

## 16. 实现建议摘要

如果按当前代码结构推进，建议优先落地以下改动面：

### 后端

- repo type 默认模板与 overlay 配置解析
- materialized / archive 路径归一化与合法性校验
- 扫描入口增加 archive root 与排除规则
- `RepoISO.metadata_json` 中的 archive 元数据字段约定

建议优先顺序：

1. 先做配置解析与路径校验；
2. 再做扫描排除；
3. 最后补 archive metadata 的最低限度写入。

这样可以先确保“不误伤现有目录型仓库”，再逐步加 archive 能力。

### 前端

- repo 设置中增加 archive 配置项
- 列表页区分 archive 元素展示
- metadata 编辑框支持 archive 生命周期与逻辑字段

### 文档与测试

- 补充 archive 仓库模式用例
- 增加 `materialized_subdir = /` 的排除扫描测试
- 增加 archive root / materialized root 冲突校验测试

---

## 17. 编码前关键提示

本节用于在后续编码时快速回忆设计目标，避免长上下文丢失。

### 17.1 不要做成什么

- 不要把 archive 设计成必须先解压才能管理；
- 不要把 archive metadata 的主真相放进 zip/rar 本体；
- 不要在目录型 normalize 流程里零散加很多 archive 特判；
- 不要因为引入 archive，就破坏当前“根目录就是主漫画库”的默认使用方式。

### 17.2 第一阶段真正要守住什么

- archive 有固定 `archive_subdir`；
- `materialized_subdir` 默认是 `/`；
- 普通扫描必须跳过 archive 子树；
- archive 至少能被识别成独立元素类型；
- 现有非 archive 仓库行为不应出现回归。

### 17.3 什么时候再进入下一阶段

只有在以下条件满足后，才建议继续进入 archive 专属 UI 和物化流程：

- 路径合同稳定；
- 排除扫描稳定；
- archive 索引不会污染普通目录流程；
- 用户能看懂当前仓库的 archive/materialized 边界。


---

## 18. 最终结论

本设计最终确定的基线如下：

1. archive 默认不可变，metadata 默认外置；
2. archive 必须统一收敛到 `archive_subdir`；
3. `materialized_subdir` 默认保持 `/`，兼容当前“根目录就是主漫画库”的模型；
4. 当 `materialized_subdir = /` 时，materialized 扫描必须显式排除 `archive_subdir`；
5. `library` 等独立 materialized 子目录是正式支持的增强模式，但不是当前默认；
6. archive 的“真正整理”应通过显式物化流程完成，而不是绑定在默认扫描链路上。

这套方案能在最小破坏现状的前提下，为 archive 建立清晰边界，并为后续逐步演进到更规整的双区仓库结构预留空间。
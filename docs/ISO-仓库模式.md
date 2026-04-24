# ISO 仓库模式（设计文档）

本文档描述 “ISO 仓库模式” 的设计与最小可行实现方案（MVP）。目标是支持多个独立仓库，每个仓库有自己的存储目录和内部数据库，仓库之间的数据相互独立，前端可以方便地切换和管理仓库。

## 设计目标（从用户需求提取）
- 支持多个仓库（repository），每个仓库有一个“仓库路径（root）”。
- 仓库信息（名称、路径、内部 DB 文件名等）保存到全局应用的一张表中（自动加载），但每个仓库的 ISO 数据保存在该仓库内部的独立 DB 文件中（不写入全局 lazybird.db）。
- 仓库 Tab 放在当前 UI header 的同一行（靠近“添加”按钮和页面标题）。
- 未添加任何仓库时，显示一个“+仓库”图标；添加后自动显示仓库的 tab，并从全局表自动加载。
- 后端在启动或添加仓库后，会扫描仓库根目录并打开仓库内部的 db 文件以读取 ISO 列表。
- 在仓库模式下，ISO 元素文件的路径与名称由程序自动管理，用户不能随意修改（用户可以上传/导入，但程序会将文件放到仓库约定目录并按规则命名）。

## 概念与数据模型

1. 全局表（保存在主 DB lazybird.db）
   - 表名：`repositories`
   - 字段（建议）：
     - id (integer, primary key)
     - name (string) - 仓库显示名
     - root_path (string) - 仓库根目录绝对路径（例如 /lzcapp/run/mnt/home/repo1）
     - db_filename (string) - 仓库内部 DB 文件名（默认：repo.db）
     - is_active (bool) - 是否为当前选中仓库（可选）
     - created_at, updated_at

2. 仓库内部 DB（每个仓库目录内的单独 sqlite 文件）
   - 结构与现有 `isos` 表一致（字段：id, path（相对于仓库 root 的子路径）, filename, md5, tags, ismounted...）
   - 建议仓库内部结构：在仓库 root 下建立 `repo.db`（sqlite）和 `isos/` 子目录保存元素文件，即：
     - /repo-root/repo.db
     - /repo-root/isos/<files.iso>

## 后端行为（MVP）

1. 启动时
   - 从全局 DB 载入 `repositories` 表。
   - 对每个仓库记录，检查 `root_path` 是否存在；若存在并且仓库 DB 文件存在（或为空则自动创建），打开该 sqlite（使用 GORM 打开不同 DB 文件），读取仓库中的 ISOs 表到内存或支持按需查询。

2. 添加仓库（API）
   - POST /repos { name, root_path, db_filename? }
   - 后端验证 root_path 可访问且有读写权限。
   - 在 global DB 中插入一条 repository 记录。
   - 在仓库 root 下创建 `db_filename`（如不存在）并初始化 `isos` 表，创建 `isos/` 子目录。

3. 删除仓库 / 更新仓库
   - DELETE /repos/:id：仅删除全局记录；可选联动删除仓库目录（需确认）。
   - PUT /repos/:id：修改名称或 root_path（修改 root_path 后需重新打开/验证仓库）。

4. 切换/查询仓库
   - GET /repos：列出已注册仓库（来自 global DB）。
   - PUT /repos/:id/select：将某个仓库设为活动（可选），或者前端直接记住当前选择。
   - GET /repos/:id/isos：从该仓库的内部 DB 中读取 ISO 列表并返回。

5. 仓库内 ISO 操作（与当前行为的差异）
   - 添加 ISO（POST /repos/:id/isos）：后端接收文件或路径，强制性的复制/移动该 ISO 到仓库内部 `isos/` 子目录，并按预设命名规则生成文件名（例如：<timestamp>_<md5>.iso 或 使用 UUID）；记录 path 字段为相对路径，例如 `isos/<stored-name>.iso`。随后在仓库内部 DB 写入记录并计算 md5（可异步）。
   - 删除 ISO（DELETE /repos/:id/isos/:iso_id）：从仓库 DB 删除对应记录；可选同时删除物理文件。
   - 下载 ISO（GET /repos/:id/download?path=... 或 GET /repos/:id/isos/:iso_id/download）：后端从仓库 root 拼接真实文件路径并返回，不可通过 arbitrary path 访问仓库外部文件（路径校验）。

## 前端变更（MVP）

1. UI：在 `IsoView` 的 header 行（与 `IsoAdd`、`RefreshButton` 同一行）添加 `RepoTabs` 组件。
   - 未注册仓库时显示 “+ 仓库” 图标，点击弹出添加仓库对话框（`root_path` 输入或者选择目录）。
   - 已注册仓库时显示 tab 列表，切换时向后端请求该仓库的 ISO 列表（`GET /repos/:id/isos`）。

2. 添加/上传流程变化：
   - 在仓库模式下，`IsoAdd` 的行为由“手工填写 path”变更为“选择本地文件上传或从宿主复制”，前端上传文件到后端 `/repos/:id/isos`，后端负责保存文件名与路径。
   - 在仓库模式下，UI 禁止直接编辑 ISO 的 path/filename，显示为只读。

3. 视图与路由：
   - 保留现有基础模式（global DB）不变，新增“仓库模式”切换行为。可以通过 `IsoView` 的一个开关来切换：
     - Global 模式：原有接口 `/isos`（不变）。
     - Repository 模式：选中某个 repo 后使用 `/repos/:id/isos` 等接口。

## API 设计（示例）

- GET /repos
  - 返回：[{id, name, root_path, db_filename, is_active}, ...]

- POST /repos
  - Body: {name, root_path, db_filename?}
  - 返回：{id, ...}

- DELETE /repos/:id

- GET /repos/:id/isos
  - 返回该仓库内部的 ISO 列表

- POST /repos/:id/isos
  - 上传 body multipart/form-data file 字段或者 JSON 指定宿主路径（由后端复制）
  - 后端将文件写入 repoRoot/isos/xxx.iso 并在 repo.db 写入记录

- DELETE /repos/:id/isos/:iso_id

- GET /repos/:id/isos/:iso_id/download 或 GET /repos/:id/download?path=...

## 安全与边界条件

- 文件系统安全：后端所有文件操作必须限定在仓库 root 下，使用 `filepath.Join` 加 `Clean` 并验证结果仍在 root 范围内，防止路径遍历。
- 权限：确保后端运行用户对仓库 root 有读写权限。
- 并发：多个请求同时写入同一仓库文件时需加锁或采用临时文件+重命名策略。
- 容量与错误：当磁盘空间不足或写入失败时返回明确错误并回滚 DB 记录。

## 实现步骤（分阶段）

阶段 A（基础框架，快速可用）
1. 在 global DB 中新增 `repositories` 表（GORM automigrate），并实现 handlers：GET/POST/DELETE /repos。
2. 后端实现 per-repo DB 打开逻辑：根据 repository 记录打开仓库内部 sqlite（使用 GORM 的 `gorm.Open(sqlite.Open(path), &gorm.Config{})`），并提供 `GetRepoISOs(repoID)` 读取逻辑。
3. 前端新增 `RepoTabs` 组件（header），实现显示/添加/切换仓库的 UI，切换后通过 `/repos/:id/isos` 加载 ISO 列表。
4. 修改后端并保留旧接口：新增 `/repos/:id/isos`、`/repos/:id/isos/:id` 的基本实现（读取/删除）。

阶段 B（文件存储、上传与命名规则）
1. 在仓库 root 下创建 `isos/` 子目录，后端实现接收上传并保存文件（自动命名规则），写入仓库 DB。异步计算 md5 并更新记录。
2. 修改前端 `IsoAdd`：在仓库模式下支持上传文件（multipart），且路径字段只读。

阶段 C（额外功能与完善）
1. 支持按仓库选择下载接口（按 repo id 的下载路由）。
2. 权限、配额、日志、UI 提示、错误处理优化、测试用例。

## 验证与测试计划（最小）

1. 单元测试：仓库表的增删查改；repo DB 打开与 ISO 列表读取。
2. 集成测试：添加仓库 -> 上传文件 -> 列表出现 -> 下载文件 -> 删除记录与文件。
3. 手动测试步骤：
   - 在本地创建一个测试仓库目录，运行 POST /repos 指向该目录，确认 `repo.db` 与 `isos/` 被创建；
   - 上传一个 ISO 文件，确认文件落在 `repoRoot/isos/` 且 repo.db 有记录；
   - 通过前端切换仓库查看列表。

## 未来扩展（建议）

- 支持仓库元数据导入/导出（便于迁移）。
- 提供仓库同步机制（把远程仓库元素同步到本地）。
- 仓库权限与多用户支持（不同用户只见到自己有权限的仓库）。
- 仓库级别的配额管理与清理策略（保留 N 份快照等）。

---

文档作者：自动化设计助手
时间：2026-03-08

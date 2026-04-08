# 仓库类型模板与 Repo Overlay 改造计划

更新时间：2026-04-04

## 1. 目标

本轮改造的目标，是把当前“新增仓库时临时选择 `repo_type`”的硬编码逻辑，升级为一套**可管理、可扩展、可按仓库局部覆盖**的配置体系。

核心设计：

- `repo type` 不再只是创建仓库时的一个临时参数；
- `repo type` 被视为 **模板（template）**；
- 每个 `repo` 绑定一个 `repo type`；
- 每个 `repo` 又可以在自己的 `repo.db` 中保存一份 **overlay 配置**；
- 最终动态表现由“模板 + overlay”共同决定。

---

## 2. 设计原则

### 2.1 模板与覆盖分离

- 全局 `repo type`：负责定义默认行为；
- 仓库自己的 overlay：只负责记录“与模板不同的那部分”。

### 2.2 最终生效顺序

```text
repo overlay setting
> repo type template
> system default fallback
```

即：

1. 若当前仓库存在 overlay 值，优先使用 overlay；
2. 若 overlay 未设置该项，则回退到绑定的 `repo type`；
3. 若模板也没有，则使用系统默认值。

### 2.3 V1 不处理旧数据迁移

当前软件仍处于第一个版本，**本轮不做旧数据迁移兼容设计**。

处理原则：

- 旧版基础 ISO 迁移逻辑先屏蔽；
- 升级迁移相关入口先关闭；
- 等未来确实需要做升级兼容时，再单独恢复并设计迁移方案。

---

## 3. 推荐的数据结构

## 3.1 全局模板表：`repo_type_defs`

存放于全局 `lazymanga.db`。

建议字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `key` | 类型唯一键，如 `manga` / `none` |
| `name` | 显示名称，如“漫画仓库” |
| `description` | 类型说明 |
| `enabled` | 是否可用 |
| `sort_order` | 排序 |
| `add_button` | 默认是否允许添加文件 |
| `add_directory_button` | 默认是否允许添加目录 |
| `delete_button` | 默认是否允许删除 |
| `auto_normalize` | 默认是否自动归类 |
| `show_md5` | 默认是否显示 MD5 |
| `show_size` | 默认是否显示文件大小 |
| `single_move` | 默认是否允许单条移动 |
| `rulebook_name` | 默认规则书名 |
| `rulebook_version` | 默认规则书版本 |
| `created_at` / `updated_at` | 时间戳 |

> 说明：`repo type` 是“模板”，不直接承载具体仓库实例信息。

## 3.2 每个仓库的 repo 级配置

存放于各自 `repo.db` 的 `repo_info` 中。

建议新增字段：

| 字段 | 说明 |
| --- | --- |
| `repo_type_key` | 当前仓库绑定的模板 key |
| `settings_override_json` | 仓库自己的 overlay 配置，仅记录差异项 |

示例：

```json
{
  "auto_normalize": false,
  "show_md5": true,
  "rulebook_name": "manga-manual",
  "rulebook_version": "v1"
}
```

说明：

- 如果字段没出现在 `settings_override_json` 中，表示“继承模板”；
- 不建议在 repo.db 再完整复制一份 repo type 表；
- V1 推荐只存“差异覆盖项”。

---

## 4. 配置解析规则

建议新增一个统一解析函数，例如：

- `ResolveEffectiveRepoSettings(repo, repoInfo)`

职责：

1. 读取 repo 绑定的 `repo_type_defs`；
2. 读取 `repo_info.settings_override_json`；
3. 按优先级合并；
4. 返回最终生效的：
   - `add_button`
   - `add_directory_button`
   - `delete_button`
   - `auto_normalize`
   - `show_md5`
   - `show_size`
   - `single_move`
   - `rulebook_name`
   - `rulebook_version`

后续 UI 与归类流程都应只依赖“最终生效配置”，不要再直接写死 `none/os` 分支。

---

## 5. 后端改造计划

### Phase 0：先做减法（本次）

- [x] 屏蔽旧版基础 ISO 自动迁移启动逻辑；
- [x] 屏蔽升级迁移提示入口；
- [x] 在设计文档中明确：V1 暂不考虑历史升级兼容。

### Phase 1：模型层改造

- [x] 新增 `models.RepoTypeDef`；
- [x] 全局库自动迁移 `repo_type_defs` 表；
- [x] 为 `models.RepoInfo` 增加：
  - [x] `RepoTypeKey string`
  - [x] `SettingsOverrideJSON string`
- [x] 准备初始模板数据：
  - [x] `none`
  - [x] `manga`
  - [x] 保留 `os` 作为兼容模板。

### Phase 2：后端接口改造

新增接口：

- [x] `GET /api/repo-types`
- [x] `POST /api/repo-types`
- [x] `PUT /api/repo-types/:key`
- [x] `DELETE /api/repo-types/:key`（使用中时自动改为禁用）
- [x] `GET /api/repos/:id/type-settings`
- [x] `PUT /api/repos/:id/type-settings`

同时改造现有逻辑：

- [x] `CreateRepo` 不再只接受硬编码 `none/os`；
- [x] `applyRepoInfoPresetByType` 改为按模板表读取；
- [x] rulebook 绑定在模板应用时可写入 repo overlay 的最终结果；
- [x] 仓库动态按钮显示，统一读取“最终生效配置”。

### Phase 3：前端界面改造

#### 3.1 顶部入口

在 `IsoView.vue` 顶部右侧：

- [x] 在帮助入口左边增加“仓库类型”按钮；
- [x] 打开“仓库类型管理”弹窗。

#### 3.2 新增仓库弹窗

改造 `RepoTabs.vue`：

- [x] 从 `/api/repo-types` 动态读取类型列表；
- [x] 不再写死 `无类型 / 操作系统元素库`；
- [x] 自动名称生成逻辑改为使用模板名称。

#### 3.3 仓库设置

改造 `RepoSettingsButton.vue`：

- [x] 支持修改当前 repo 绑定的 `repo type`；
- [x] 支持编辑当前 repo 的 overlay 配置；
- [x] 提供“恢复继承模板”按钮。

#### 3.4 仓库类型管理页/弹窗

建议新增组件：

- [x] `RepoTypeManagerButton.vue`
- [x] `RepoTypeManagerDialog.vue`（当前合并在按钮组件内部实现）

功能：

- [x] 查看所有模板；
- [x] 新建模板；
- [x] 修改模板；
- [x] 启用/禁用模板；
- [x] 配置默认 rulebook；
- [x] 预览某个模板的默认行为。

### Phase 4：联调与验证

- [ ] 验证新建仓库时，模板配置会正确写入 repo；
- [ ] 验证 overlay 可以覆盖模板；
- [ ] 验证恢复继承后会回退到模板值；
- [ ] 验证 rulebook 绑定解析顺序正确；
- [ ] 验证前端动态按钮展示无回归。

---

## 6. V1 推荐的初始模板

### `none`

适合纯手工管理仓库：

- `add_button = true`
- `add_directory_button = false`
- `delete_button = true`
- `auto_normalize = false`
- `show_md5 = true`
- `show_size = true`
- `single_move = true`
- `rulebook = noop@v1`

### `manga`

作为本产品主推模板：

- 第一阶段可以先做成偏保守：
  - `add_button = true`
  - `add_directory_button = true`
  - `auto_normalize = false`
  - `show_md5 = true`
  - `show_size = true`
- 后续等漫画规则书稳定后，再切换到：
  - `rulebook = manga-library@v1`
  - `auto_normalize = true`

### `os`（可选）

- 仅作为兼容模板保留；
- 不再作为主推入口；
- 后续视产品方向决定是否完全隐藏。

---

## 7. 本轮实施边界

本轮只做下面几件事：

1. 固化设计文档；
2. 关闭旧迁移入口；
3. 后续按 Phase 1 → Phase 4 逐步落地。

暂不做：

- 旧数据迁移；
- 升级版本兼容；
- 自动把历史仓库映射到新模板体系。

---

## 8. 实施建议

后续开发时，建议严格按下面顺序推进：

1. 先完成数据模型与 API；
2. 再改新增仓库弹窗；
3. 再做“仓库类型管理”入口；
4. 最后做 repo 级 overlay 编辑界面。

这样可以保证：

- 每一步都能独立验证；
- 避免一次性改动过大；
- 更适合逐步演进当前项目。

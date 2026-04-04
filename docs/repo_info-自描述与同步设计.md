# RepoInfo 自描述与同步设计

本文档用于固定 repo.db 中 repo_info 元信息表的设计方向，并明确它与全局 repositories 表之间的职责边界、同步规则、Go 结构体合同、启动流程与后续实现顺序。

目标不是一次性把所有细节实现完，而是先建立一个稳定、可回看、可校验的设计基线。后续每一步实现都应以本文件为对照，避免实现逐步偏离最初目标。

## 当前实现状态（2026-03-21）

- 阶段 1 已完成：
- 全局 `repositories` 已新增 `repo_uuid` 字段。
- `repo.db` 已新增 `repo_info` 模型，并在打开 repo.db 时自动迁移。
- 阶段 2 已完成：
- 启动时会执行仓库 bootstrap（补建 `repo_info`、修复全局缓存 `repo_uuid/name/basic`、检测 UUID 冲突）。
- 阶段 3 已完成：
- 新建仓库会生成 `repo_uuid`；在 `root_path` 已配置时会立即写 repo_info 并同步缓存。
- 改仓库 `name/basic` 时，优先写 repo_info，再同步全局缓存；若 `root_path` 尚未配置，则先更新全局缓存并在后续路径配置后补齐。
- 更新仓库路径后，会触发一次 bootstrap，补齐/修复 repo_info 与缓存。
- 阶段 4 已部分完成：
- `GetRepos` 读路径已接入元信息刷新，返回前会尝试按 repo_info 修复缓存。
- 仓库列表排序已切换为语义驱动：`basic DESC, id ASC`，不再依赖仓库名称硬编码。

## 1. 设计目标

这套设计服务于两个长期目标：

- 让 repo.db 从“只存 repoisos 的索引库”升级为“自描述的仓库数据库”。
- 逐步统一基础仓库与普通仓库的抽象层级，为后续合并处理逻辑打基础。

本设计不追求把所有仓库元数据都塞进 repo.db，而是强调职责分离：

- repo.db 负责描述“这个仓库自己是谁”。
- 全局 lazymanga.db 负责描述“当前这台机器如何接入这个仓库”。

## 2. 核心原则

### 2.1 双层模型

系统中存在两类仓库元数据：

1. 自描述元数据
   - 保存在每个仓库自己的 repo.db 中。
   - 表名建议为 repo_info。
   - 用于描述仓库自身的稳定身份与元属性。

2. 本机接入注册数据
   - 保存在全局 lazymanga.db 的 repositories 表中。
   - 用于描述当前应用实例如何找到并接入这个仓库。

### 2.2 单一主真相

必须避免“双向都算主数据”的设计。

建议规则：

- repo_info 是以下字段的主真相：repo_uuid、name、basic、schema_version、flags_json。
- repositories 是以下字段的主真相：root_path、db_filename、is_internal、external_device_name、is_active、本地 id。

### 2.3 单行表合理性

repo_info 设计为“只有一行数据的单例表”是合理的，因为一个 repo.db 天然只属于一个仓库。

但这个合理性依赖两个前提：

- 单例语义必须有明确约束，而不是只靠口头约定。
- 表中字段必须代表“仓库自身元信息”，而不是本机接入环境信息。

## 3. 职责边界

### 3.1 repo_info 负责什么

repo_info 只负责仓库自身的稳定元信息，适合放这些字段：

- repo_uuid
- name
- basic
- schema_version
- flags_json
- created_at
- updated_at

这些字段应尽量具备“仓库复制到另一台机器后仍然有意义”的特征。

### 3.2 repositories 负责什么

全局 repositories 表继续承担本机接入注册职责，适合保留这些字段：

- id
- repo_uuid
- name
- basic
- root_path
- db_filename
- is_internal
- external_device_name
- is_active
- created_at
- updated_at

其中：

- repo_uuid 是和 repo_info 建立稳定绑定关系的关键字段。
- name/basic 虽然也存在于全局表中，但应被视为缓存字段，而不是长期主真相。

### 3.3 明确不放进 repo_info 的字段

第一版明确不建议把这些字段作为 repo_info 的主数据：

- root_path
- db_filename
- is_internal
- external_device_name
- is_active
- 全局 repositories 的本地自增 id

原因：这些字段依赖当前机器的目录结构、挂载方式和接入策略，不是仓库自身稳定身份的一部分。

## 4. 表结构设计

## 4.1 repo_info 表建议结构

建议在 repo.db 中新增表 repo_info，第一版结构如下：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | integer | 固定值 1，用于表达单例表 |
| repo_uuid | string | 仓库稳定唯一标识 |
| name | string | 仓库正式名称 |
| basic | bool | 是否为基础仓库 |
| schema_version | integer | repo.db 元数据结构版本 |
| flags_json | string | 预留扩展字段，默认 `{}` |
| created_at | time | 创建时间 |
| updated_at | time | 更新时间 |

建议约束：

- id 固定为 1。
- repo_uuid 非空，且在单表内唯一。
- name 非空。
- schema_version 非空，默认 1。
- flags_json 非空，默认 `{}`。

## 4.2 全局 repositories 表建议补充字段

当前全局 repositories 表已存在 name、basic、root_path 等字段。为支撑新的绑定模型，建议新增：

- repo_uuid string

语义：

- repositories.repo_uuid 用于标识“这条本机注册记录连接的是哪个仓库实体”。

建议约束：

- 第一阶段允许为空，用于兼容旧数据。
- 完成迁移后，应尽量做到非空。

## 5. Go 结构体合同草案

以下为建议的 Go 结构体草案，目的是固定模型语义，而不是要求逐字实现。

```go
package models

import "time"

type RepoInfo struct {
    ID            uint      `gorm:"primaryKey;autoIncrement:false" json:"id"`
    RepoUUID      string    `gorm:"not null;uniqueIndex" json:"repo_uuid"`
    Name          string    `gorm:"not null" json:"name"`
    Basic         bool      `gorm:"not null;default:false" json:"basic"`
    SchemaVersion int       `gorm:"not null;default:1" json:"schema_version"`
    FlagsJSON     string    `gorm:"not null;default:{}" json:"flags_json"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}

func (RepoInfo) TableName() string {
    return "repo_info"
}
```

全局 Repository 结构体建议增加：

```go
type Repository struct {
    ID                 uint      `gorm:"primaryKey;autoIncrement;index" json:"id"`
    RepoUUID           string    `gorm:"index" json:"repo_uuid"`
    Name               string    `json:"name"`
    Basic              bool      `gorm:"not null;default:false" json:"basic"`
    RootPath           string    `json:"root_path"`
    DBFile             string    `json:"db_filename"`
   IsInternal         bool      `gorm:"not null" json:"is_internal"`
    ExternalDeviceName string    `json:"external_device_name"`
    IsActive           bool      `json:"is_active"`
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
}
```

说明：

- RepoInfo.ID 设计成固定值 1，比“允许多行但约定只用第一行”更稳。
- Repository.RepoUUID 是和 repo_info 关联的关键字段。
- Repository.Name/Basic 在过渡期仍保留，作为缓存和前端展示字段。

## 6. 同步规则

## 6.1 主真相归属

字段主真相规则如下：

| 字段 | 主真相位置 |
| --- | --- |
| repo_uuid | repo_info |
| name | repo_info |
| basic | repo_info |
| schema_version | repo_info |
| flags_json | repo_info |
| root_path | repositories |
| db_filename | repositories |
| is_internal | repositories |
| external_device_name | repositories |
| is_active | repositories |

## 6.2 启动同步规则

服务启动时：

1. 执行 `EnsureBasicRepository(...)`，确保基础仓库存在并与当前环境路径一致。
2. 执行 `BootstrapRepositories()`，扫描全局 repositories。
3. 对每条可访问仓库记录：
   - 打开 repo.db 并迁移 repo 表结构。
   - 若 repo_info 缺失则补建（从全局记录初始化）。
   - 用 repo_info 修复全局缓存：`repo_uuid/name/basic`。
4. 若发现绑定冲突，则记录错误并停止对该仓库的自动修复。

### 6.3 绑定冲突定义

以下情况视为绑定冲突，而不是普通不同步：

- repositories.repo_uuid 非空，且与 repo_info.repo_uuid 不一致。
- 同一 repo_uuid 在全局 repositories 中被多个不同仓库路径绑定。

遇到绑定冲突时，不应自动覆盖，应记录明确错误，等待人工修复。

## 6.4 新建仓库流程

创建新仓库时建议顺序：

1. 先生成并写入全局记录：`repo_uuid/name/basic/root_path/db_filename/...`。
2. 当 `root_path` 已配置时，立刻执行 `BootstrapSingleRepository(...)`：
   - 创建/修复 repo_info。
   - 同步全局缓存。
3. 当 `root_path` 为空时，延后 bootstrap，等待后续路径配置后自动补齐。

与代码一致：

- 当前不会因为尚未配置路径而阻止仓库创建。
- 当前在 repo.db 初始化失败时会回滚刚创建的全局仓库记录。

理想顺序（长期目标）仍然是：

1. 决定本机接入参数：root_path、db_filename、is_internal、external_device_name。
2. 打开或创建 repo.db。
3. 在 repo.db 中写入 repo_info。
4. 在全局 repositories 中写入本机注册记录。

已验证的实现约束：

- `is_internal=false` 不能依赖 Gorm 的 `default:true` 写入默认值，否则外部仓库创建时可能把 Go 零值 `false` 误当成“未赋值”，最终被数据库默认值覆盖成 `true`。
- 创建 repositories 记录时，应显式包含 `is_internal`、`external_device_name`、`root_path` 等绑定字段写入。
- 写入后应立即回读并校验绑定参数；若持久化后的 `is_internal/external_device_name/root_path` 与请求不一致，应立刻回滚新记录并终止 bootstrap，避免误打开错误路径下的 `repo.db`。

repo_info 建议字段：

- repo_uuid
- name
- basic
- schema_version=1
- flags_json=`{}`

全局 repositories 建议字段：

- repo_uuid
- name
- basic
- root_path
- db_filename
- is_internal
- external_device_name

## 6.5 修改仓库名字流程

修改名字时建议流程：

1. 先写 repo_info.name。
2. 再同步写 repositories.name 缓存。
3. 如果第二步失败，不回滚第一步；依靠下一次启动同步修复缓存。

与代码一致的补充规则：

- 当仓库 `root_path` 未配置时，暂时只能更新全局缓存；待路径配置完成后再补齐 repo_info。

这是典型的最终一致性策略，不强求跨两个 sqlite 库做事务一致性。

## 6.6 修改 basic 流程

修改 basic 时建议流程：

1. 先写 repo_info.basic。
2. 再写 repositories.basic 缓存。
3. 如果全局库更新失败，下一次启动时自动回填。

与代码一致的补充规则：

- `PUT /repos/:id` 已支持独立更新 `basic`（也支持 name+basic 同时更新）。
- 当仓库 `root_path` 未配置时，先更新全局缓存并延后 repo_info 补齐。

## 6.7 修改路径或挂载方式流程

修改这些字段时，只更新全局 repositories，不更新 repo_info：

- root_path
- db_filename
- is_internal
- external_device_name
- is_active

理由：这些字段只描述当前应用实例如何接入仓库，不是仓库自描述数据的一部分。

与代码一致的补充规则：

- `UpdateRepoPath` 在保存全局路径后，会立即触发 `BootstrapSingleRepository(...)`。
- 这一步会确保路径生效后 repo_info 已可用，并修复全局缓存字段。

## 6.8 列表读取与排序规则

当前 `GetRepos` 的读路径规则：

1. 先从全局 repositories 按 `basic DESC, id ASC` 查询。
2. 执行 `RefreshRepositoryMetadataCaches(...)`，尝试从 repo_info 修复缓存。
3. 若有缓存更新，则按同样排序重查一次并返回。
4. 刷新失败按 warning 记录，不阻断读取。

说明：

- “基础仓库在前”是 `basic` 语义驱动，不再依赖名称硬编码。
- 名称只用于默认展示，不用于排序语义。

## 6.10 已验证的外部仓库创建陷阱

2026-03-23 的一次真实故障表明，仓库创建流程里最容易被忽略的不是 `repo_uuid` 本身，而是“接入绑定字段”在 ORM 层被错误持久化。

故障表现：

- 请求创建的是外部仓库。
- 创建前的 repo.db 探测按外部路径执行，没有命中现有 `repo.db`。
- 全局记录插入后，`is_internal=false` 被错误落成 `true`。
- 随后的 bootstrap 按内部路径打开 `/lzcapp/run/mnt/home/.../repo.db`。
- 最终与内部目录下已有仓库的 `repo_info.repo_uuid` 冲突。

这个问题说明：

- “创建前探测路径正确”不代表“插入后的仓库绑定一定正确”。
- `BootstrapSingleRepository(...)` 之前必须先验证全局记录已经把绑定参数持久化对了。

当前代码中的防御策略：

1. `models.Repository.IsInternal` 不再依赖 `default:true`。
2. `CreateRepo` 使用显式字段列表写入 `RepoUUID/Name/Basic/RootPath/DBFile/IsInternal/ExternalDeviceName`。
3. `CreateRepo` 在 `db.Create(...)` 之后立刻回读新记录并校验：
   - `is_internal`
   - `external_device_name`
   - `root_path`
4. 若校验失败，则删除刚插入的全局记录并返回错误，不再进入 bootstrap。

这条经验应视为仓库创建流程的固定约束，而不是一次性的排障补丁。

## 6.9 删除仓库流程

应明确区分两类删除：

1. 仅删除本机注册
   - 删除全局 repositories 记录
   - 不删除 repo.db 中的 repo_info
   - 不删除仓库目录与镜像文件

2. 删除仓库实体
   - 删除全局 repositories 记录
   - 删除 repo.db 与仓库目录
   - 删除物理镜像文件

当前系统第一阶段建议只支持第 1 类行为。

## 7. repo_info 单例写入规则

repo_info 应始终被当作单例表处理：

- 初始化时插入固定主键 `id=1`
- 后续读写一律以 `id=1` 访问
- 若意外存在多行，应记录严重错误，而不是默认选第一条继续执行

建议封装统一入口，避免业务代码直接散落读写：

```go
func GetRepoInfo(repoDB *gorm.DB) (models.RepoInfo, error)
func UpsertRepoInfo(repoDB *gorm.DB, info models.RepoInfo) error
func EnsureRepoInfoFromRepository(repoDB *gorm.DB, repo models.Repository) (models.RepoInfo, error)
func SyncRepositoryCacheFromRepoInfo(repo models.Repository, info models.RepoInfo) error
func writeRepoInfoMetadata(repo models.Repository, nextName string, nextBasic bool) error
```

## 8. 建议的启动流程合同

为后续实现稳定性，建议把当前 openRepoScopedDB 的职责保持为“打开 repo.db + AutoMigrate 仓库内部表”，然后在启动阶段新增一个明确的同步流程。

建议合同如下：

```go
func BootstrapRepositories() error
```

其内部逻辑建议为：

1. 查询全局 repositories。
2. 对每个仓库：
   - 打开 repo.db
   - AutoMigrate repo_info 与 repoisos
   - 读取 repo_info
   - 若 repo_info 缺失则补建
   - 若 repositories 的 name/basic/repo_uuid 缓存落后则修复
   - 若发现 UUID 冲突则记录并上报

建议拆分辅助函数：

```go
func BootstrapSingleRepository(repo models.Repository) error
func EnsureRepoInfoFromRepository(repoDB *gorm.DB, repo models.Repository) (models.RepoInfo, error)
func SyncRepositoryCacheFromRepoInfo(repo models.Repository, info models.RepoInfo) error
func DetectRepositoryBindingConflict(repo models.Repository, info models.RepoInfo) error
func RefreshRepositoryMetadataCaches(repos []models.Repository) (int, []error)
```

## 9. 实施进度与顺序

建议按以下顺序推进，而不是并行大改：

### 阶段 1：建模与迁移

- 在 repo.db 中新增 RepoInfo 模型与 AutoMigrate。
- 在全局 Repository 模型中新增 RepoUUID 字段。
- 为基础仓库生成 repo_info。

状态：已完成。

### 阶段 2：启动同步

- 服务启动时扫描所有仓库。
- 为旧仓库补建 repo_info。
- 自动修复全局 name/basic/repo_uuid 缓存。

状态：已完成。

### 阶段 3：写路径收口

- 新建仓库时同时写 repo_info 和 repositories。
- 改名字时先写 repo_info，再写 repositories 缓存。
- 改 basic 时先写 repo_info，再写 repositories 缓存。

状态：已完成（含 `root_path` 为空时的延后补齐策略）。

### 阶段 4：读路径收口

- 前端列表仍可先读全局 repositories。
- 后端语义上明确 name/basic 的最终来源是 repo_info。

状态：进行中（GetRepos 读路径已接入刷新；后续可继续补测试与更多读接口收口）。

### 阶段 5：扩展演进

- 视需要把 basic 升级为 repo_kind。
- 增加更多 flags 或拆分 flags_json。
- 为 repo.db 提供导入、导出、自检能力。

状态：未开始。

## 10. 风险与边界

### 10.1 不要过早把 root_path 放进 repo_info

当前基础仓库已经表现出“root_path 与 lazymanga.db 所在目录绑定”的需求，这恰恰说明 root_path 是部署环境相关信息。把它做成 repo_info 主数据，会弱化设计边界。

### 10.2 不要依赖名字做长期绑定

“基础仓库”这个名字可以用于默认初始化与前端展示，但不能替代 repo_uuid 的长期绑定作用。

### 10.3 不要试图跨两个 sqlite 做强事务

repo.db 与 lazymanga.db 是两个独立数据库文件，强事务会增加复杂度且收益有限。第一版应接受最终一致性，并用启动同步做修复机制。

### 10.4 不要允许 repo_info 静默多行

一旦 repo_info 出现两行以上，说明数据已损坏或逻辑已失控。此时应显式报错，而不是悄悄继续工作。

## 11. 验证清单

实现过程中，每完成一个阶段，都应回头检查以下问题：

- 是否已经明确 repo_info 与 repositories 的主真相边界。
- 是否仍然只在本机注册表中维护路径和挂载方式。
- 是否已经用 repo_uuid 替代“只靠名字绑定”的隐式逻辑。
- 是否所有 repo_info 读写都走统一函数而不是散落多处。
- 是否保留了“启动自修复”的能力。
- 是否出现了新的双向同步耦合。

## 12. 当前结论

当前推荐方案是：

- 在 repo.db 中新增 repo_info 单例表。
- 在全局 repositories 表中新增 repo_uuid。
- 把 repo_info 设计成仓库自描述元数据表，而不是全局表的完整镜像。
- 把 repositories 继续保留为本机接入注册表。
- 把 name/basic 视为可缓存字段，最终由 repo_info 驱动。

本文件是后续实现的设计基线。如果未来实现与本文件冲突，应优先更新本文件，再修改代码，避免“代码和设计各自演化”。
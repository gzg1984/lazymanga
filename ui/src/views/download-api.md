# Download API (临时规范)

前端会调用此接口下载 ISO 文件的二进制流。此文档描述请求/响应/错误码约定，前后端按照此约定实现。

接口
- 方法: GET
- 路径: /api/download
- 参数: query 参数 `path`，示例: `/api/download?path=ISO%2Fubuntu-20.04.2-live-server-amd64.iso`

请求示例

GET /api/download?path=ISO%2Fubuntu.iso

成功响应
- HTTP 200
- Headers:
  - Content-Disposition: attachment; filename="ubuntu-20.04.2-live-server-amd64.iso"
  - Content-Type: application/octet-stream  (或基于文件类型推断的 mime)
- Body: 文件二进制流

错误响应
- 400 Bad Request: 缺少或无效的 `path` 参数。返回 JSON: { "error": "missing path" }
- 404 Not Found: 文件不存在或不可访问。返回 JSON: { "error": "file not found" }
- 500 Internal Server Error: 其他服务器错误。返回 JSON: { "error": "..." }

备注
- 前端会使用 `fetch` 获取响应并使用 `response.blob()` 创建下载链接。
- 路径 `path` 是数据库中记录的子路径（例如 `ISO/xxx.iso`），后端需使用 `sys.GetFullPathFromDBSubPath` 或等效方法映射到真实磁盘路径并做权限校验。

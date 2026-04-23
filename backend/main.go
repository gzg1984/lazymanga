package main

import (
	"flag"
	"lazymanga/database"
	"lazymanga/handlers"
	"lazymanga/normalization"
	"log"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func main() {
	// 定义命令行参数
	dbPath := flag.String("db", "/lzcapp/var/lazymanga.db", "Path to the SQLite database file")
	flag.Parse()

	db := database.InitDB(*dbPath)
	normalization.SetRuleBookUserDir(filepath.Join(filepath.Dir(*dbPath), "rulebooks"))

	handlers.SetDB(db)
	if err := handlers.EnsureDefaultRepoTypes(); err != nil {
		log.Printf("初始化仓库类型模板失败: %v", err)
	}
	if err := handlers.EnsureBasicRepository(*dbPath); err != nil {
		log.Printf("初始化基础仓库失败: %v", err)
	}
	// V1 暂不处理旧版本升级迁移；后续需要兼容升级时再恢复此入口。
	if err := handlers.AuditRepositoryBindingsFromEnv(); err != nil {
		log.Printf("启动仓库绑定审计失败: %v", err)
	}
	if err := handlers.BootstrapRepositories(); err != nil {
		log.Printf("启动仓库元信息同步失败: %v", err)
	}

	// 自动清理path重复记录
	// 需确保handlers包已正确import
	if err := handlers.DeleteDuplicateISOs(); err != nil {
		log.Printf("自动清理重复ISO记录失败: %v", err)
	}

	r := gin.Default()

	// 记录所有未被路由匹配的请求，方便排查 404 问题
	r.NoRoute(func(c *gin.Context) {
		log.Printf("NoRoute matched: method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.String(), c.ClientIP())
		c.JSON(404, gin.H{"error": "not found"})
	})

	r.GET("/userinfo", handlers.GetUserInfo)

	r.GET("/isos", handlers.GetISOs)
	r.GET("/isos/:id/file-status", handlers.CheckISOFileStatus)
	r.GET("/rulebook/status", handlers.GetRuleBookStatus)
	r.GET("/rulebooks", handlers.ListRuleBooks)
	r.POST("/rulebooks", handlers.CreateRuleBook)
	r.GET("/rulebooks/content", handlers.GetRuleBookContent)
	r.PUT("/rulebooks/content", handlers.UpdateRuleBookContent)
	//r.GET("/reboot", handlers.Reboot)
	r.POST("/addiso", handlers.CreateISOs)
	r.GET("/open", handlers.HandleOpen)
	r.POST("/open", handlers.HandleOpen)
	//r.PUT("/todos/:id", handlers.UpdateTodo)
	r.DELETE("/delisos/:id", handlers.DeleteISO)

	// 仓库相关接口
	r.GET("/repos", handlers.GetRepos)
	r.POST("/repos", handlers.CreateRepo)
	r.PUT("/repos/:id", handlers.UpdateRepo)
	r.GET("/repo-types", handlers.ListRepoTypes)
	r.POST("/repo-types", handlers.CreateRepoType)
	r.PUT("/repo-types/:key", handlers.UpdateRepoType)
	r.DELETE("/repo-types/:key", handlers.DeleteRepoType)
	r.GET("/repos/:id/type-settings", handlers.GetRepoTypeSettings)
	r.PUT("/repos/:id/type-settings", handlers.UpdateRepoTypeSettings)
	r.GET("/repos/:id/path/external-devices", handlers.ListExternalRepoDevices)
	r.GET("/repos/:id/path/options", handlers.ListRepoPathOptions)
	r.PUT("/repos/:id/path", handlers.UpdateRepoPath)
	r.GET("/repos/:id/storage-summary", handlers.GetRepoStorageSummary)
	r.GET("/repos/:id/repo-info", handlers.GetRepoInfo)
	r.GET("/repos/:id/rulebook/binding", handlers.GetRepoRuleBookBinding)
	r.PUT("/repos/:id/rulebook/binding", handlers.UpdateRepoRuleBookBinding)
	r.GET("/repos/:id/repoisos", handlers.GetRepoISOs)
	r.POST("/repos/:id/repoisos", handlers.CreateRepoISO)
	r.DELETE("/repos/:id/repoisos/missing", handlers.DeleteMissingRepoISOs)
	r.POST("/repos/:id/repoisos/:isoId/move", handlers.MoveSingleRepoISO)
	r.GET("/repos/:id/repoisos/:isoId/download", handlers.DownloadRepoISO)
	r.DELETE("/repos/:id/repoisos/:isoId", handlers.DeleteRepoISO)
	r.POST("/repos/:id/repoisos/:isoId/manual-edit", handlers.ManualEditRepoISO)
	r.POST("/repos/:id/repoisos/:isoId/refresh", handlers.RefreshRepoISORecord)
	r.POST("/repos/:id/repoisos/refresh-proposals", handlers.ListRepoISORefreshProposals)
	r.POST("/repos/:id/normalize", handlers.ForceNormalizeRepo)
	r.POST("/repos/:id/normalize/incremental", handlers.ForceNormalizeRepoIncremental)
	r.POST("/repos/merge-transfer", handlers.RequestRepoMergeTransfer)
	r.GET("/repos/merge-transfer/tasks/:taskId", handlers.GetRepoMergeTask)
	r.DELETE("/repos/:id", handlers.DeleteRepo)
	// 同时支持带 /api 前缀的路由，匹配前端代理场景
	r.GET("/api/repos", handlers.GetRepos)
	r.POST("/api/repos", handlers.CreateRepo)
	r.PUT("/api/repos/:id", handlers.UpdateRepo)
	r.GET("/api/repo-types", handlers.ListRepoTypes)
	r.POST("/api/repo-types", handlers.CreateRepoType)
	r.PUT("/api/repo-types/:key", handlers.UpdateRepoType)
	r.DELETE("/api/repo-types/:key", handlers.DeleteRepoType)
	r.GET("/api/repos/:id/type-settings", handlers.GetRepoTypeSettings)
	r.PUT("/api/repos/:id/type-settings", handlers.UpdateRepoTypeSettings)
	r.GET("/api/repos/:id/path/external-devices", handlers.ListExternalRepoDevices)
	r.GET("/api/repos/:id/path/options", handlers.ListRepoPathOptions)
	r.PUT("/api/repos/:id/path", handlers.UpdateRepoPath)
	r.GET("/api/repos/:id/storage-summary", handlers.GetRepoStorageSummary)
	r.GET("/api/repos/:id/repo-info", handlers.GetRepoInfo)
	r.GET("/api/repos/:id/rulebook/binding", handlers.GetRepoRuleBookBinding)
	r.PUT("/api/repos/:id/rulebook/binding", handlers.UpdateRepoRuleBookBinding)
	r.GET("/api/repos/:id/repoisos", handlers.GetRepoISOs)
	r.POST("/api/repos/:id/repoisos", handlers.CreateRepoISO)
	r.DELETE("/api/repos/:id/repoisos/missing", handlers.DeleteMissingRepoISOs)
	r.POST("/api/repos/:id/repoisos/:isoId/move", handlers.MoveSingleRepoISO)
	r.GET("/api/repos/:id/repoisos/:isoId/download", handlers.DownloadRepoISO)
	r.DELETE("/api/repos/:id/repoisos/:isoId", handlers.DeleteRepoISO)
	r.POST("/api/repos/:id/repoisos/:isoId/manual-edit", handlers.ManualEditRepoISO)
	r.POST("/api/repos/:id/repoisos/:isoId/refresh", handlers.RefreshRepoISORecord)
	r.POST("/api/repos/:id/repoisos/refresh-proposals", handlers.ListRepoISORefreshProposals)
	r.POST("/api/repos/:id/normalize", handlers.ForceNormalizeRepo)
	r.POST("/api/repos/:id/normalize/incremental", handlers.ForceNormalizeRepoIncremental)
	r.POST("/api/repos/merge-transfer", handlers.RequestRepoMergeTransfer)
	r.GET("/api/repos/merge-transfer/tasks/:taskId", handlers.GetRepoMergeTask)
	r.DELETE("/api/repos/:id", handlers.DeleteRepo)
	r.GET("/api/isos/:id/file-status", handlers.CheckISOFileStatus)
	r.GET("/api/rulebook/status", handlers.GetRuleBookStatus)
	r.GET("/api/rulebooks", handlers.ListRuleBooks)
	r.POST("/api/rulebooks", handlers.CreateRuleBook)
	r.GET("/api/rulebooks/content", handlers.GetRuleBookContent)
	r.PUT("/api/rulebooks/content", handlers.UpdateRuleBookContent)
	r.GET("/api/open", handlers.HandleOpen)
	r.POST("/api/open", handlers.HandleOpen)
	r.GET("/api/debug/open-flow-logs", handlers.GetOpenFlowLogs)
	r.GET("/debug/open-flow-logs", handlers.GetOpenFlowLogs)

	// 新增文件列表接口
	r.GET("/files", handlers.FilesHandler)

	// 挂载请求
	r.POST("/mount", handlers.MountHandler)

	// 下载接口
	r.GET("/download", handlers.DownloadHandler)
	// 支持通过前端代理使用 /api/download 的情况
	r.GET("/api/download", handlers.DownloadHandler)

	// TEST
	r.GET("/debug/updateemptyid", handlers.UpdateEmptyID)
	r.POST("/debug/updateemptyid", handlers.UpdateEmptyID)

	r.GET("/queryalltags", handlers.QueryAllTags)

	// 输出已注册路由，便于排查 404 问题
	for _, ri := range r.Routes() {
		log.Printf("route registered: method=%s path=%s", ri.Method, ri.Path)
	}

	r.Run(":3000")
}

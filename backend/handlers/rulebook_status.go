package handlers

import (
	"lazymanga/normalization"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetRuleBookStatus returns current default rulebook runtime status.
func GetRuleBookStatus(c *gin.Context) {
	status := normalization.GetDefaultRuleBookLoadStatus()

	c.JSON(http.StatusOK, gin.H{
		"source":         status.Source,
		"file_path":      status.FilePath,
		"using_fallback": status.UsingFallback,
		"last_error":     status.LastError,
		"book_name":      status.BookName,
		"book_version":   status.BookVersion,
		"rule_count":     status.RuleCount,
		"updated_at":     status.UpdatedAt,
	})
}

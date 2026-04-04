package handlers

import (
	"lazymanga/normalization"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ListRuleBooks returns currently discoverable rulebook files and validation status.
func ListRuleBooks(c *gin.Context) {
	books := normalization.ListAvailableRuleBooks()
	validCount := 0
	for _, b := range books {
		if b.Valid {
			validCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":       len(books),
		"valid_count": validCount,
		"items":       books,
	})
}

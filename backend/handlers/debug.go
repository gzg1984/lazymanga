package handlers

import (
	"lazymanga/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func UpdateEmptyID(c *gin.Context) {
	// 查询所有ISOs记录
	var all []models.ISOs
	if err := db.Find(&all).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 找到最大id
	maxID := uint(0)
	for _, iso := range all {
		if iso.ID > maxID {
			maxID = iso.ID
		}
	}

	// 为id为空的记录分配新id
	updated := 0
	for _, iso := range all {
		if iso.ID == 0 {
			maxID++
			// 不能用ID为key更新，需用其它唯一字段
			db.Model(&models.ISOs{}).Where("path = ? AND id = 0", iso.Path).Update("id", maxID)
			updated++
		}
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}
func DeleteDublicate(c *gin.Context) {

	// 查询所有ISOs记录
	var all []models.ISOs
	if err := db.Find(&all).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 统计每个path出现的次数，保留每个path的第一条，删除其余
	pathMap := make(map[string]bool)
	var toDelete []uint
	for _, iso := range all {
		if pathMap[iso.Path] {
			toDelete = append(toDelete, iso.ID)
		} else {
			pathMap[iso.Path] = true
		}
	}
	if len(toDelete) > 0 {
		if err := db.Delete(&models.ISOs{}, toDelete).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusNoContent, nil)
}

package handlers

import (
	"fmt"
	"lazyiso/models"

	"github.com/gin-gonic/gin"
)

// 删除数据库中指定id的ISO记录
func DeleteISOByID(id uint) error {
	result := db.Delete(&models.ISOs{}, id)
	return result.Error
}

// 自动遍历所有ISO记录，删除path重复的记录（仅保留每个path的第一条）
func DeleteDuplicateISOs() error {
	var isos []models.ISOs
	if err := db.Find(&isos).Error; err != nil {
		return err
	}
	pathMap := make(map[string]uint) // path -> first id
	var toDelete []uint
	for _, iso := range isos {
		if _ /*firstID*/, exists := pathMap[iso.Path]; exists {
			toDelete = append(toDelete, iso.ID)
		} else {
			pathMap[iso.Path] = iso.ID
		}
	}
	if len(toDelete) > 0 {
		fmt.Printf("[ISO] 删除path重复记录 %d 条\n", len(toDelete))
		if err := db.Delete(&models.ISOs{}, toDelete).Error; err != nil {
			return err
		}
	} else {
		fmt.Println("[ISO] 未发现path重复记录，无需删除")
	}
	return nil
}

// DeleteISO 用于删除指定id的ISO记录
// 示例请求: DELETE /todos/123
func DeleteISO(c *gin.Context) {
	// 获取id参数
	idParam := c.Param("id")
	var id uint
	_, err := fmt.Sscanf(idParam, "%d", &id)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "无效的id参数"})
		return
	}
	if err := DeleteISOByID(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "删除成功", "id": id})
}

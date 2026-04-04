package handlers

import (
	"lazymanga/models"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// checkAndUpdateMountStatus 检查并更新ISO列表的挂载状态
func checkAndUpdateMountStatus(isos []models.ISOs) []models.ISOs {
	// 读取/proc/mounts
	mountsRaw, err := os.ReadFile("/proc/mounts")
	var mountLines []string
	if err == nil {
		mountLines = splitLines(string(mountsRaw))
	}

	for i, iso := range isos {
		mountDir := iso.Path
		if strings.HasSuffix(mountDir, ".iso") {
			mountDir = mountDir[:len(mountDir)-len(".iso")]
		}
		for _, line := range mountLines {
			fields := splitFields(line)
			if len(fields) >= 2 && fields[1] == mountDir {
				isos[i].MountPath = fields[1]
				isos[i].IsMounted = true
				break
			}
		}
	}
	return isos
}

func GetISOs(c *gin.Context) {
	var ISOs []models.ISOs
	if err := db.Find(&ISOs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 检查并更新挂载状态
	//todos = checkAndUpdateMountStatus(todos)

	c.JSON(http.StatusOK, ISOs)
}

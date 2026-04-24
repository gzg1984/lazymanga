package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"lazymanga/models"
	"lazymanga/sys"

	"github.com/gin-gonic/gin"
)

// MountHandler 处理 /mount POST 请求
func MountHandler(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 查询数据库中是否有已挂载记录
	var isoRecords []models.ISOs
	if db != nil {
		if err := db.Where("path = ?", req.Path).Find(&isoRecords).Error; err == nil {
			for _, rec := range isoRecords {
				if rec.IsMounted {
					c.JSON(http.StatusOK, gin.H{"status": "already mounted in db"})
					return
				}
			}
		}
	}

	realPath := sys.GetFullPathFromDBSubPath(req.Path)
	// 1. 检查iso文件是否存在
	if _, err := os.Stat(realPath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "iso文件不存在"})
		return
	}

	// 2. 去掉iso后缀，得到挂载目录
	mountDir := realPath
	if filepath.Ext(mountDir) == ".iso" {
		mountDir = mountDir[:len(mountDir)-len(".iso")]
	}

	// 检查挂载目录是否存在，不存在则创建
	if _, err := os.Stat(mountDir); os.IsNotExist(err) {
		if err := os.MkdirAll(mountDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建挂载目录失败"})
			return
		}
	}

	// 3. 检查是否已经挂载
	// 通过解析 /proc/mounts 判断
	alreadyMounted := false
	if mounts, err := os.ReadFile("/proc/mounts"); err == nil {
		lines := string(mounts)
		for _, line := range splitLines(lines) {
			if len(line) > 0 && containsMountPoint(line, mountDir) {
				alreadyMounted = true
				break
			}
		}
	}
	if alreadyMounted {
		c.JSON(http.StatusOK, gin.H{"status": "already mounted"})
		return
	}

	// 4. 执行mount命令
	cmd := fmt.Sprintf("sudo mount -o loop '%s' '%s'", realPath, mountDir)
	if err := runShell(cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mount失败", "detail": err.Error()})
		return
	}
	fmt.Printf("[MOUNT] 成功: %s -> %s\n", realPath, mountDir)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "mountDir": mountDir})
}

// splitLines 按行分割字符串
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// containsMountPoint 检查mounts行是否包含挂载点
func containsMountPoint(line, mountDir string) bool {
	// /dev/loop0 /mnt/iso1 iso9660 ro 0 0
	fields := splitFields(line)
	if len(fields) >= 2 && fields[1] == mountDir {
		return true
	}
	return false
}

// splitFields 按空白分割
func splitFields(s string) []string {
	var fields []string
	field := ""
	for _, c := range s {
		if c == ' ' || c == '\t' {
			if field != "" {
				fields = append(fields, field)
				field = ""
			}
		} else {
			field += string(c)
		}
	}
	if field != "" {
		fields = append(fields, field)
	}
	return fields
}

// runShell 执行shell命令
func runShell(cmd string) error {
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

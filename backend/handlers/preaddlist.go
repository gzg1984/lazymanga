package handlers

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func FilesHandler(c *gin.Context) {
	root := "/lzcapp/run/mnt/home/" // 根目录
	dirParam := c.Query("dir")
	dir := root
	if dirParam != "" {
		dir = filepath.Join(root, dirParam)
	}
	type FileInfo struct {
		Name  string `json:"name"`
		Size  int64  `json:"size"`
		IsDir bool   `json:"isDir"`
	}
	var files []FileInfo
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(500, gin.H{"error": "无法读取目录"})
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if entry.IsDir() {
			files = append(files, FileInfo{
				Name:  entry.Name(),
				Size:  0,
				IsDir: true,
			})
		} else if filepath.Ext(entry.Name()) == ".iso" {
			files = append(files, FileInfo{
				Name:  entry.Name(),
				Size:  info.Size(),
				IsDir: false,
			})
		}
	}
	c.JSON(200, files)
}
